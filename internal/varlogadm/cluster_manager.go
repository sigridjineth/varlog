package varlogadm

//go:generate mockgen -build_flags -mod=vendor -self_package github.com/kakao/varlog/internal/varlogadm -package varlogadm -destination varlogadm_mock.go . ClusterMetadataView,StorageNodeManager

import (
	"context"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/pkg/util/netutil"
	"github.com/kakao/varlog/pkg/util/runner/stopwaiter"
	"github.com/kakao/varlog/pkg/verrors"
	"github.com/kakao/varlog/proto/mrpb"
	"github.com/kakao/varlog/proto/snpb"
	"github.com/kakao/varlog/proto/varlogpb"
	"github.com/kakao/varlog/proto/vmspb"
)

type StorageNodeEventHandler interface {
	HandleHeartbeatTimeout(context.Context, types.StorageNodeID)

	HandleReport(context.Context, *varlogpb.StorageNodeMetadataDescriptor)
}

// ClusterManager manages varlog cluster.
type ClusterManager interface {
	io.Closer

	// AddStorageNode adds new StorageNode to the cluster.
	AddStorageNode(ctx context.Context, addr string) (*varlogpb.StorageNodeMetadataDescriptor, error)

	UnregisterStorageNode(ctx context.Context, storageNodeID types.StorageNodeID) error

	AddTopic(ctx context.Context) (varlogpb.TopicDescriptor, error)

	Topics(ctx context.Context) ([]varlogpb.TopicDescriptor, error)

	DescribeTopic(ctx context.Context, topicID types.TopicID) (varlogpb.TopicDescriptor, []varlogpb.LogStreamDescriptor, error)

	UnregisterTopic(ctx context.Context, topicID types.TopicID) error

	AddLogStream(ctx context.Context, topicID types.TopicID, replicas []*varlogpb.ReplicaDescriptor) (*varlogpb.LogStreamDescriptor, error)

	UnregisterLogStream(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) error

	RemoveLogStreamReplica(ctx context.Context, storageNodeID types.StorageNodeID, topicID types.TopicID, logStreamID types.LogStreamID) error

	UpdateLogStream(ctx context.Context, logStreamID types.LogStreamID, poppedReplica, pushedReplica *varlogpb.ReplicaDescriptor) (*varlogpb.LogStreamDescriptor, error)

	// Seal seals the log stream replicas corresponded with the given logStreamID.
	Seal(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) ([]varlogpb.LogStreamMetadataDescriptor, types.GLSN, error)

	// Sync copies the log entries of the src to the dst. Sync may be long-running, thus it
	// returns immediately without waiting for the completion of sync. Callers of Sync
	// periodically can call Sync, and get the current state of the sync progress.
	// SyncState is one of SyncStateError, SyncStateInProgress, or SyncStateComplete. If Sync
	// returns SyncStateComplete, all the log entries were copied well. If it returns
	// SyncStateInProgress, it is still progressing. Otherwise, if it returns SyncStateError,
	// it is stopped by an error.
	// To start sync, the log stream status of the src must be LogStreamStatusSealed and the log
	// stream status of the dst must be LogStreamStatusSealing. If either of the statuses is not
	// correct, Sync returns ErrSyncInvalidStatus.
	Sync(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID, srcID, dstID types.StorageNodeID) (*snpb.SyncStatus, error)

	// Unseal unseals the log stream replicas corresponded with the given logStreamID.
	Unseal(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) (*varlogpb.LogStreamDescriptor, error)

	Metadata(ctx context.Context) (*varlogpb.MetadataDescriptor, error)

	MRInfos(ctx context.Context) (*mrpb.ClusterInfo, error)

	AddMRPeer(ctx context.Context, raftURL, rpcAddr string) (types.NodeID, error)

	RemoveMRPeer(ctx context.Context, raftURL string) error

	Trim(ctx context.Context, topicID types.TopicID, lastGLSN types.GLSN) ([]vmspb.TrimResult, error)

	Run() error

	Address() string

	Wait()
}

var _ ClusterManager = (*clusterManager)(nil)

type clusterManagerState int

const (
	clusterManagerReady clusterManagerState = iota
	clusterManagerRunning
	clusterManagerClosed

	numLogStreamMutex = 512
)

type clusterManager struct {
	server       *grpc.Server
	serverAddr   string
	healthServer *health.Server

	// single large lock
	mu                sync.RWMutex
	muLogStreamStatus [numLogStreamMutex]sync.Mutex

	cmState clusterManagerState
	sw      *stopwaiter.StopWaiter

	snMgr          StorageNodeManager
	mrMgr          MetadataRepositoryManager
	cmView         ClusterMetadataView
	snSelector     ReplicaSelector
	snWatcher      StorageNodeWatcher
	statRepository StatRepository
	logStreamIDGen LogStreamIDGenerator
	topicIDGen     TopicIDGenerator

	logger  *zap.Logger
	options *Options
}

func NewClusterManager(ctx context.Context, opts *Options) (ClusterManager, error) {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}

	mrMgr, err := NewMRManager(ctx, opts.ClusterID, opts.MRManagerOptions, opts.Logger)
	if err != nil {
		return nil, err
	}

	cmView := mrMgr.ClusterMetadataView()

	snMgr, err := NewStorageNodeManager(ctx, opts.ClusterID, cmView, opts.Logger)
	if err != nil {
		return nil, err
	}

	logStreamIDGen, err := NewSequentialLogStreamIDGenerator(ctx, cmView, snMgr)
	if err != nil {
		return nil, err
	}

	topicIDGen, err := NewSequentialTopicIDGenerator(ctx, cmView, snMgr)
	if err != nil {
		return nil, err
	}

	snSelector, err := newBalancedReplicaSelector(cmView, int(opts.ReplicationFactor))
	if err != nil {
		return nil, err
	}

	cm := &clusterManager{
		sw:             stopwaiter.New(),
		cmState:        clusterManagerReady,
		snMgr:          snMgr,
		mrMgr:          mrMgr,
		cmView:         cmView,
		snSelector:     snSelector,
		statRepository: NewStatRepository(ctx, cmView),
		logStreamIDGen: logStreamIDGen,
		topicIDGen:     topicIDGen,
		logger:         opts.Logger,
		options:        opts,
	}

	cm.snWatcher = NewStorageNodeWatcher(opts.WatcherOptions, cmView, snMgr, cm, opts.Logger)

	cm.server = grpc.NewServer()
	cm.healthServer = health.NewServer()
	grpc_health_v1.RegisterHealthServer(cm.server, cm.healthServer)

	newClusterManagerService(cm, cm.logger).Register(cm.server)

	return cm, nil
}

func (cm *clusterManager) Address() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.serverAddr
}

func (cm *clusterManager) Run() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	switch cm.cmState {
	case clusterManagerRunning:
		return nil
	case clusterManagerClosed:
		return errors.Wrap(verrors.ErrClosed, "vms")
	}
	cm.cmState = clusterManagerRunning

	// Listener
	lis, err := net.Listen("tcp", cm.options.ListenAddress)
	if err != nil {
		return errors.WithStack(err)
	}
	addrs, _ := netutil.GetListenerAddrs(lis.Addr())
	// TODO (jun): choose best address
	cm.serverAddr = addrs[0]

	// RPC Server
	go func() {
		if err := cm.server.Serve(lis); err != nil {
			cm.logger.Error("could not serve", zap.Error(err))
			cm.Close()
		}
	}()

	// SN Watcher
	if err := cm.snWatcher.Run(); err != nil {
		return err
	}

	cm.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	cm.logger.Info("start")
	return nil
}

func (cm *clusterManager) Wait() {
	cm.sw.Wait()
}

func (cm *clusterManager) Close() (err error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	switch cm.cmState {
	case clusterManagerReady:
		return errors.Wrapf(verrors.ErrState, "cluster manager: %s", cm.cmState)
	case clusterManagerClosed:
		return nil
	}
	cm.cmState = clusterManagerClosed
	cm.mu.Unlock()

	// SN Watcher
	err = cm.snWatcher.Close()

	cm.mu.Lock()
	err = multierr.Combine(err, cm.snMgr.Close(), cm.mrMgr.Close())
	cm.server.Stop()
	cm.sw.Stop()
	cm.logger.Info("stop")
	return err
}

func (cm *clusterManager) Metadata(ctx context.Context) (*varlogpb.MetadataDescriptor, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cmView.ClusterMetadata(ctx)
}

func (cm *clusterManager) MRInfos(ctx context.Context) (*mrpb.ClusterInfo, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.mrMgr.GetClusterInfo(ctx)
}

func (cm *clusterManager) AddMRPeer(ctx context.Context, raftURL, rpcAddr string) (types.NodeID, error) {
	nodeID := types.NewNodeIDFromURL(raftURL)
	if nodeID == types.InvalidNodeID {
		return nodeID, errors.Wrap(verrors.ErrInvalid, "raft address")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	err := cm.mrMgr.AddPeer(ctx, nodeID, raftURL, rpcAddr)
	if err != nil {
		if !errors.Is(err, verrors.ErrAlreadyExists) {
			return types.InvalidNodeID, err
		}
	}

	return nodeID, nil
}

func (cm *clusterManager) RemoveMRPeer(ctx context.Context, raftURL string) error {
	nodeID := types.NewNodeIDFromURL(raftURL)
	if nodeID == types.InvalidNodeID {
		return errors.Wrap(verrors.ErrInvalid, "raft address")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	err := cm.mrMgr.RemovePeer(ctx, nodeID)
	if err != nil {
		if !errors.Is(err, verrors.ErrAlreadyExists) {
			return err
		}
	}

	return nil
}

func (cm *clusterManager) AddStorageNode(ctx context.Context, addr string) (snmeta *varlogpb.StorageNodeMetadataDescriptor, err error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.snMgr.ContainsAddress(addr) {
		return nil, errors.Wrap(verrors.ErrExist, "storage node address")
	}

	snmcl, snmeta, err := cm.snMgr.GetMetadataByAddr(ctx, addr)
	if err != nil {
		return nil, err
	}

	var (
		clusmeta      *varlogpb.MetadataDescriptor
		storageNodeID = snmeta.GetStorageNode().GetStorageNodeID()
	)

	if cm.snMgr.Contains(storageNodeID) {
		err = errors.Wrap(verrors.ErrExist, "storage node id")
		goto errOut
	}

	clusmeta, err = cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		goto errOut
	}

	if err = clusmeta.MustNotHaveStorageNode(storageNodeID); err != nil {
		goto errOut
	}

	if err = cm.mrMgr.RegisterStorageNode(ctx, snmeta.GetStorageNode()); err != nil {
		goto errOut
	}

	cm.snMgr.AddStorageNode(snmcl)
	return snmeta, nil

errOut:
	return nil, multierr.Append(err, snmcl.Close())
}

func (cm *clusterManager) UnregisterStorageNode(ctx context.Context, storageNodeID types.StorageNodeID) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clusmeta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return err
	}

	if _, err := clusmeta.MustHaveStorageNode(storageNodeID); err != nil {
		return err
	}

	// TODO (jun): Use helper function
	for _, lsdesc := range clusmeta.GetLogStreams() {
		for _, replica := range lsdesc.GetReplicas() {
			if replica.GetStorageNodeID() == storageNodeID {
				return errors.New("active log stream")
				// return errors.Wrap(errRunningLogStream, "vms")
			}
		}
	}

	if err := cm.mrMgr.UnregisterStorageNode(ctx, storageNodeID); err != nil {
		return err
	}

	cm.snMgr.RemoveStorageNode(storageNodeID)
	return nil
}

func (cm *clusterManager) AddTopic(ctx context.Context) (varlogpb.TopicDescriptor, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	topicID := cm.topicIDGen.Generate()
	if err := cm.mrMgr.RegisterTopic(ctx, topicID); err != nil {
		return varlogpb.TopicDescriptor{}, err
	}

	return varlogpb.TopicDescriptor{TopicID: topicID}, nil
}

func (cm *clusterManager) Topics(ctx context.Context) ([]varlogpb.TopicDescriptor, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	md, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil || len(md.Topics) == 0 {
		return nil, err
	}

	tds := make([]varlogpb.TopicDescriptor, len(md.Topics))
	for idx := range md.Topics {
		tds[idx] = *md.Topics[idx]
	}
	return tds, nil
}

func (cm *clusterManager) DescribeTopic(ctx context.Context, topicID types.TopicID) (td varlogpb.TopicDescriptor, lsds []varlogpb.LogStreamDescriptor, err error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	md, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil || len(md.Topics) == 0 {
		return
	}

	tdPtr := md.GetTopic(topicID)
	if tdPtr == nil {
		err = errors.Wrapf(verrors.ErrNotExist, "no such topic (topicID=%d)", topicID)
		return
	}
	td = *proto.Clone(tdPtr).(*varlogpb.TopicDescriptor)
	lsds = make([]varlogpb.LogStreamDescriptor, 0, len(td.LogStreams))
	for _, lsID := range td.LogStreams {
		lsdPtr := md.GetLogStream(lsID)
		if lsdPtr == nil {
			continue
		}
		lsd := *proto.Clone(lsdPtr).(*varlogpb.LogStreamDescriptor)
		lsds = append(lsds, lsd)
	}

	return td, lsds, nil
}

func (cm *clusterManager) UnregisterTopic(ctx context.Context, topicID types.TopicID) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clusmeta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return err
	}

	topicdesc, err := clusmeta.MustHaveTopic(topicID)
	if err != nil {
		return err
	}

	status := topicdesc.GetStatus()
	if status.Deleted() {
		return errors.Errorf("invalid topic status: %s", status)
	}

	//TODO:: seal logStreams and refresh metadata

	return cm.mrMgr.UnregisterTopic(ctx, topicID)
}

func (cm *clusterManager) AddLogStream(ctx context.Context, topicID types.TopicID, replicas []*varlogpb.ReplicaDescriptor) (*varlogpb.LogStreamDescriptor, error) {
	lsdesc, err := cm.addLogStreamInternal(ctx, topicID, replicas)
	if err != nil {
		return lsdesc, err
	}

	err = cm.waitSealed(ctx, lsdesc.LogStreamID)
	if err != nil {
		return lsdesc, err
	}

	return cm.Unseal(ctx, topicID, lsdesc.LogStreamID)
}

func (cm *clusterManager) addLogStreamInternal(ctx context.Context, topicID types.TopicID, replicas []*varlogpb.ReplicaDescriptor) (*varlogpb.LogStreamDescriptor, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var err error

	if len(replicas) == 0 {
		replicas, err = cm.snSelector.Select(ctx)
		if err != nil {
			return nil, err
		}
	}

	clusmeta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return nil, err
	}

	// See https://github.com/kakao/varlog/pull/198#discussion_r215602
	logStreamID := cm.logStreamIDGen.Generate()
	if err := clusmeta.MustNotHaveLogStream(logStreamID); err != nil {
		if e := cm.logStreamIDGen.Refresh(ctx); e != nil {
			err = multierr.Append(err, e)
			cm.logger.Panic("could not refresh LogStreamIDGenerator", zap.Error(err))
		}
		return nil, err
	}

	logStreamDesc := &varlogpb.LogStreamDescriptor{
		TopicID:     topicID,
		LogStreamID: logStreamID,
		Status:      varlogpb.LogStreamStatusSealing,
		Replicas:    replicas,
	}

	if err := cm.verifyLogStream(clusmeta, logStreamDesc); err != nil {
		return nil, err
	}

	// TODO: Choose the primary - e.g., shuffle logStreamReplicaMetas
	return cm.addLogStream(ctx, logStreamDesc)
}

func (cm *clusterManager) waitSealed(ctx context.Context, logStreamID types.LogStreamID) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			lsStat := cm.statRepository.GetLogStream(logStreamID).Copy()
			if lsStat.status == varlogpb.LogStreamStatusSealed {
				return nil
			}
		}
	}
}

func (cm *clusterManager) UnregisterLogStream(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	clusmeta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return err
	}

	lsdesc, err := clusmeta.MustHaveLogStream(logStreamID)
	if err != nil {
		return err
	}

	status := lsdesc.GetStatus()
	// TODO (jun): Check whether status.Deleted means unregistered.
	// If so, is status.Deleted okay or not?
	if status.Running() || status.Deleted() {
		return errors.Errorf("invalid log stream status: %s", status)
	}

	// TODO (jun): test if the log stream has no logs

	return cm.mrMgr.UnregisterLogStream(ctx, logStreamID)
}

func (cm *clusterManager) verifyLogStream(clusmeta *varlogpb.MetadataDescriptor, lsdesc *varlogpb.LogStreamDescriptor) error {
	replicas := lsdesc.GetReplicas()
	// the number of logstream replica
	if uint(len(replicas)) != cm.options.ReplicationFactor {
		return errors.Errorf("invalid number of log stream replicas: %d", len(replicas))
	}
	// storagenode existence
	for _, replica := range replicas {
		if _, err := clusmeta.MustHaveStorageNode(replica.GetStorageNodeID()); err != nil {
			return err
		}
	}
	// logstream existence
	return clusmeta.MustNotHaveLogStream(lsdesc.GetLogStreamID())
}

func (cm *clusterManager) addLogStream(ctx context.Context, lsdesc *varlogpb.LogStreamDescriptor) (*varlogpb.LogStreamDescriptor, error) {
	if err := cm.snMgr.AddLogStream(ctx, lsdesc); err != nil {
		return nil, err
	}

	// NB: RegisterLogStream returns nil if the logstream already exists.
	return lsdesc, cm.mrMgr.RegisterLogStream(ctx, lsdesc)
}

func (cm *clusterManager) RemoveLogStreamReplica(ctx context.Context, storageNodeID types.StorageNodeID, topicID types.TopicID, logStreamID types.LogStreamID) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	clusmeta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return err
	}

	if err := cm.removableLogStreamReplica(clusmeta, storageNodeID, logStreamID); err != nil {
		return err
	}

	return cm.snMgr.RemoveLogStream(ctx, storageNodeID, topicID, logStreamID)
}

func (cm *clusterManager) UpdateLogStream(ctx context.Context, logStreamID types.LogStreamID, poppedReplica, pushedReplica *varlogpb.ReplicaDescriptor) (*varlogpb.LogStreamDescriptor, error) {
	// NOTE (jun): Name of the method - UpdateLogStream can be confused.
	// UpdateLogStream can change only replicas. To update status, use Seal or Unseal.
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	clusmeta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return nil, err
	}

	oldLSDesc, err := clusmeta.MustHaveLogStream(logStreamID)
	if err != nil {
		return nil, err
	}

	status := oldLSDesc.GetStatus()
	if status.Running() || status.Deleted() {
		return nil, errors.Errorf("invalid log stream status: %s", status)
	}

	if poppedReplica == nil {
		// TODO: Choose laggy replica
		selector := newVictimSelector(cm.snMgr, logStreamID, oldLSDesc.GetReplicas())
		victims, err := selector.Select(ctx)
		if err != nil {
			return nil, err
		}
		poppedReplica = victims[0]
	}

	if pushedReplica == nil {
		oldReplicas := oldLSDesc.GetReplicas()
		denylist := make([]types.StorageNodeID, len(oldReplicas))
		for i, replica := range oldReplicas {
			denylist[i] = replica.GetStorageNodeID()
		}

		selector, err := newRandomReplicaSelector(cm.cmView, 1, denylist...)
		if err != nil {
			return nil, err
		}
		candidates, err := selector.Select(ctx)
		if err != nil {
			return nil, err
		}
		pushedReplica = candidates[0]
	}

	replace := false
	newLSDesc := proto.Clone(oldLSDesc).(*varlogpb.LogStreamDescriptor)
	for i := range newLSDesc.Replicas {
		// TODO - fix? poppedReplica can ignore path.
		if newLSDesc.Replicas[i].GetStorageNodeID() == poppedReplica.GetStorageNodeID() {
			newLSDesc.Replicas[i] = pushedReplica
			replace = true
			break
		}
	}
	if !replace {
		cm.logger.Panic("logstream push/pop error")
	}

	if err := cm.snMgr.AddLogStreamReplica(ctx, pushedReplica.GetStorageNodeID(), newLSDesc.TopicID, logStreamID, pushedReplica.GetPath()); err != nil {
		return nil, err
	}

	// To reset the status of the log stream, set it as LogStreamStatusRunning
	defer func() {
		cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusRunning)
	}()

	if err := cm.mrMgr.UpdateLogStream(ctx, newLSDesc); err != nil {
		return nil, err
	}

	return newLSDesc, nil
}

func (cm *clusterManager) removableLogStreamReplica(clusmeta *varlogpb.MetadataDescriptor, storageNodeID types.StorageNodeID, logStreamID types.LogStreamID) error {
	lsdesc := clusmeta.GetLogStream(logStreamID)
	if lsdesc == nil {
		// unregistered LS or garbage
		return nil
	}

	replicas := lsdesc.GetReplicas()
	for _, replica := range replicas {
		if replica.GetStorageNodeID() == storageNodeID {
			return errors.Wrap(verrors.ErrState, "running log stream is not removable")
		}
	}
	return nil
}

func (cm *clusterManager) lockLogStreamStatus(logStreamID types.LogStreamID) {
	cm.muLogStreamStatus[logStreamID%numLogStreamMutex].Lock()
}

func (cm *clusterManager) unlockLogStreamStatus(logStreamID types.LogStreamID) {
	cm.muLogStreamStatus[logStreamID%numLogStreamMutex].Unlock()
}

func (cm *clusterManager) Seal(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) ([]varlogpb.LogStreamMetadataDescriptor, types.GLSN, error) {
	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	return cm.seal(ctx, topicID, logStreamID)
}

func (cm *clusterManager) seal(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) ([]varlogpb.LogStreamMetadataDescriptor, types.GLSN, error) {
	cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusSealing)

	lastGLSN, err := cm.mrMgr.Seal(ctx, logStreamID)
	if err != nil {
		cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusRunning)
		return nil, types.InvalidGLSN, err
	}

	result, err := cm.snMgr.Seal(ctx, topicID, logStreamID, lastGLSN)
	if err != nil {
		cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusRunning)
	}

	return result, lastGLSN, err
}

func (cm *clusterManager) Sync(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID, srcID, dstID types.StorageNodeID) (*snpb.SyncStatus, error) {
	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	return cm.sync(ctx, topicID, logStreamID, srcID, dstID)
}

func (cm *clusterManager) sync(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID, srcID, dstID types.StorageNodeID) (*snpb.SyncStatus, error) {
	lastGLSN, err := cm.mrMgr.Seal(ctx, logStreamID)
	if err != nil {
		return nil, err
	}
	return cm.snMgr.Sync(ctx, topicID, logStreamID, srcID, dstID, lastGLSN)
}

func (cm *clusterManager) Unseal(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) (*varlogpb.LogStreamDescriptor, error) {
	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	return cm.unseal(ctx, topicID, logStreamID)
}

func (cm *clusterManager) unseal(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) (*varlogpb.LogStreamDescriptor, error) {
	var err error
	var clusmeta *varlogpb.MetadataDescriptor
	cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusUnsealing)

	if err = cm.snMgr.Unseal(ctx, topicID, logStreamID); err != nil {
		goto errOut
	}

	if err = cm.mrMgr.Unseal(ctx, logStreamID); err != nil {
		goto errOut
	}

	if clusmeta, err = cm.cmView.ClusterMetadata(ctx); err != nil {
		goto errOut
	}

	return clusmeta.GetLogStream(logStreamID), nil

errOut:
	cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusRunning)
	return nil, err
}

func (cm *clusterManager) HandleHeartbeatTimeout(ctx context.Context, snID types.StorageNodeID) {
	meta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return
	}

	//TODO: store sn status
	for _, ls := range meta.GetLogStreams() {
		if ls.IsReplica(snID) {
			cm.logger.Debug("seal due to heartbeat timeout", zap.Any("snid", snID), zap.Any("lsid", ls.LogStreamID))
			cm.Seal(ctx, ls.TopicID, ls.LogStreamID)
		}
	}
}

func (cm *clusterManager) checkLogStreamStatus(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID, mrStatus, replicaStatus varlogpb.LogStreamStatus) {
	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	lsStat := cm.statRepository.GetLogStream(logStreamID).Copy()

	switch lsStat.Status() {
	case varlogpb.LogStreamStatusRunning:
		if mrStatus.Sealed() || replicaStatus.Sealed() {
			cm.logger.Info("seal due to status mismatch", zap.Any("lsid", logStreamID))
			cm.seal(ctx, topicID, logStreamID)
		}

	case varlogpb.LogStreamStatusSealing:
		for _, r := range lsStat.Replicas() {
			if r.Status != varlogpb.LogStreamStatusSealed {
				cm.logger.Info("seal due to status", zap.Any("lsid", logStreamID))
				cm.seal(ctx, topicID, logStreamID)
				return
			}
		}
		cm.logger.Info("sealed", zap.Any("lsid", logStreamID))
		cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusSealed)

	case varlogpb.LogStreamStatusSealed:
		for _, r := range lsStat.Replicas() {
			if r.Status != varlogpb.LogStreamStatusSealed {
				cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusSealing)
				return
			}
		}

	case varlogpb.LogStreamStatusUnsealing:
		for _, r := range lsStat.Replicas() {
			if r.Status == varlogpb.LogStreamStatusRunning {
				continue
			} else if r.Status == varlogpb.LogStreamStatusSealed {
				return
			} else if r.Status == varlogpb.LogStreamStatusSealing {
				cm.logger.Info("seal due to unexpected status", zap.Any("lsid", logStreamID))
				cm.seal(ctx, topicID, logStreamID)
				return
			}
		}
		cm.statRepository.SetLogStreamStatus(logStreamID, varlogpb.LogStreamStatusRunning)
	}
}

func (cm *clusterManager) syncLogStream(ctx context.Context, topicID types.TopicID, logStreamID types.LogStreamID) {
	cm.lockLogStreamStatus(logStreamID)
	defer cm.unlockLogStreamStatus(logStreamID)

	min, max := types.MaxVersion, types.InvalidVersion
	var src, tgt types.StorageNodeID

	lsStat := cm.statRepository.GetLogStream(logStreamID).Copy()

	if !lsStat.Status().Sealed() {
		return
	}

	snIDs := make([]types.StorageNodeID, 0, len(lsStat.Replicas()))
	for snID := range lsStat.Replicas() {
		snIDs = append(snIDs, snID)
	}
	sort.Slice(snIDs, func(i, j int) bool { return snIDs[i] < snIDs[j] })

	for i, snID := range snIDs {
		r, _ := lsStat.Replica(snID)

		if !r.Status.Sealed() {
			return
		}

		if i == 0 || r.Version < min {
			min = r.Version
			tgt = snID
		}

		if i == 0 || r.Version > max {
			max = r.Version
			src = snID
		}
	}

	if src != tgt {
		status, err := cm.sync(ctx, topicID, logStreamID, src, tgt)
		cm.logger.Debug("sync", zap.Any("lsid", logStreamID), zap.Any("src", src), zap.Any("dst", tgt), zap.String("status", status.String()), zap.Error(err))

		//TODO: Unseal
		//status, _ := cm.Sync(context.TODO(), ls.LogStreamID, src, tgt)
		//if status.GetState() == snpb.SyncStateComplete {
		//cm.Unseal(context.TODO(), ls.LogStreamID)
		//}
	}
}

func (cm *clusterManager) HandleReport(ctx context.Context, snm *varlogpb.StorageNodeMetadataDescriptor) {
	meta, err := cm.cmView.ClusterMetadata(ctx)
	if err != nil {
		return
	}

	cm.statRepository.Report(ctx, snm)

	// Sync LogStreamStatus
	for _, ls := range snm.GetLogStreams() {
		mls := meta.GetLogStream(ls.LogStreamID)
		if mls != nil {
			cm.checkLogStreamStatus(ctx, ls.TopicID, ls.LogStreamID, mls.Status, ls.Status)
			continue
		}
		if time.Since(ls.CreatedTime) > cm.options.WatcherOptions.GCTimeout {
			cm.RemoveLogStreamReplica(ctx, snm.StorageNode.StorageNodeID, ls.TopicID, ls.LogStreamID)
		}
	}

	// Sync LogStream
	for _, ls := range snm.GetLogStreams() {
		if ls.Status.Sealed() {
			cm.syncLogStream(ctx, ls.TopicID, ls.LogStreamID)
		}
	}
}

func (cm *clusterManager) Trim(ctx context.Context, topicID types.TopicID, lastGLSN types.GLSN) ([]vmspb.TrimResult, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.snMgr.Trim(ctx, topicID, lastGLSN)
}
