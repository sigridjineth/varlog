package metadata_repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	varlog "github.com/kakao/varlog/pkg/varlog"
	types "github.com/kakao/varlog/pkg/varlog/types"
	"github.com/kakao/varlog/pkg/varlog/util/runner"
	pb "github.com/kakao/varlog/proto/metadata_repository"
	snpb "github.com/kakao/varlog/proto/storage_node"
	varlogpb "github.com/kakao/varlog/proto/varlog"

	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
	"go.uber.org/zap"
)

const unusedRequestIndex uint64 = 0

type Config struct {
	Index             types.NodeID
	Join              bool
	NumRep            int
	PeerList          []string
	ReporterClientFac ReporterClientFactory
	Logger            *zap.Logger
}

func (config *Config) validate() error {
	if config.Index == types.InvalidNodeID {
		return errors.New("invalid index")
	}

	if config.NumRep < 1 {
		return errors.New("NumRep should be bigger than 0")
	}

	if len(config.PeerList) < 1 {
		return errors.New("# of PeerList should be bigger than 0")
	}

	if config.ReporterClientFac == nil {
		return errors.New("reporterClientFac should not be nil")
	}

	if config.Logger == nil {
		return errors.New("logger should not be nil")
	}

	return nil
}

type RaftMetadataRepository struct {
	index             types.NodeID
	nrReplica         int
	raftState         raft.StateType
	reportCollector   *ReportCollector
	logger            *zap.Logger
	raftNode          *raftNode
	reporterClientFac ReporterClientFactory

	storage *MetadataStorage

	// for ack
	requestNum uint64
	requestMap sync.Map

	// for raft
	proposeC      chan *pb.RaftEntry
	commitC       chan *pb.RaftEntry
	rnConfChangeC chan raftpb.ConfChange
	rnProposeC    chan string
	rnCommitC     chan *raftCommittedEntry
	rnErrorC      chan error
	rnStateC      chan raft.StateType

	runner runner.Runner
	cancel context.CancelFunc
}

func NewRaftMetadataRepository(config *Config) *RaftMetadataRepository {
	if err := config.validate(); err != nil {
		panic(err)
	}

	mr := &RaftMetadataRepository{
		index:             config.Index,
		nrReplica:         config.NumRep,
		logger:            config.Logger,
		reporterClientFac: config.ReporterClientFac,
	}

	mr.storage = NewMetadataStorage(mr.sendAck)

	mr.proposeC = make(chan *pb.RaftEntry, 4096)
	mr.commitC = make(chan *pb.RaftEntry, 4096)

	mr.rnConfChangeC = make(chan raftpb.ConfChange)
	mr.rnProposeC = make(chan string)
	mr.raftNode = newRaftNode(
		config.Index,
		config.PeerList,
		//false, // not to join an existing cluster
		config.Join,
		mr.storage.GetSnapshot,
		mr.rnProposeC,
		mr.rnConfChangeC,
		//mr.logger.Named("raftnode"),
		mr.logger.Named(fmt.Sprintf("%v", config.Index)),
	)
	mr.rnCommitC = mr.raftNode.commitC
	mr.rnErrorC = mr.raftNode.errorC
	mr.rnStateC = mr.raftNode.stateC

	cbs := ReportCollectorCallbacks{
		report:        mr.proposeReport,
		getClient:     mr.reporterClientFac.GetClient,
		lookupNextGLS: mr.storage.LookupNextGLS,
	}

	mr.reportCollector = NewReportCollector(cbs,
		mr.logger.Named("report"))

	return mr
}

func (mr *RaftMetadataRepository) Run() {
	mr.storage.Run()

	ctx, cancel := context.WithCancel(context.Background())
	mr.cancel = cancel
	mr.runner.Run(ctx, mr.runReplication)
	mr.runner.Run(ctx, mr.processCommit)
	mr.runner.Run(ctx, mr.processRNCommit)
	mr.runner.Run(ctx, mr.processRNState)
	mr.runner.Run(ctx, mr.runCommitTrigger)

	go mr.raftNode.startRaft()
}

//TODO:: fix it
func (mr *RaftMetadataRepository) Close() error {
	mr.reportCollector.Close()

	var err error
	if mr.cancel != nil {
		mr.cancel()
		err = <-mr.rnErrorC

		mr.runner.CloseWait()

		mr.storage.Close()
	}

	mr.setFollower()

	//TODO:: handle pendding msg

	return err
}

func (mr *RaftMetadataRepository) isLeader() bool {
	return raft.StateLeader == raft.StateType(atomic.LoadUint64((*uint64)(&mr.raftState)))
}

func (mr *RaftMetadataRepository) setFollower() {
	atomic.StoreUint64((*uint64)(&mr.raftState), uint64(raft.StateFollower))
}

func (mr *RaftMetadataRepository) runReplication(ctx context.Context) {
Loop:
	for {
		select {
		case e := <-mr.proposeC:
			b, err := e.Marshal()
			if err != nil {
				mr.logger.Error(err.Error())
				continue
			}

			select {
			case mr.rnProposeC <- string(b):
			case <-ctx.Done():
				mr.sendAck(e.NodeIndex, e.RequestIndex, ctx.Err())
			}
		case <-ctx.Done():
			break Loop
		}
	}

	close(mr.rnProposeC)

	// fix me
	close(mr.raftNode.stopc)
}

func (mr *RaftMetadataRepository) runCommitTrigger(ctx context.Context) {
	ticker := time.NewTicker(time.Millisecond)
Loop:
	for {
		select {
		case <-ticker.C:
			mr.proposeCommit()
		case <-ctx.Done():
			break Loop
		}
	}

	ticker.Stop()
}

func (mr *RaftMetadataRepository) processCommit(ctx context.Context) {
	for e := range mr.commitC {
		mr.apply(e)
	}
}

func (mr *RaftMetadataRepository) processRNCommit(ctx context.Context) {
	for d := range mr.rnCommitC {
		if d == nil {
			// TODO: handle snapshots
			continue
		}

		e := &pb.RaftEntry{}
		err := e.Unmarshal([]byte(d.data))
		if err != nil {
			mr.logger.Error(err.Error())
			continue
		}
		e.AppliedIndex = d.index

		mr.commitC <- e
	}

	close(mr.commitC)
}

func (mr *RaftMetadataRepository) processRNState(ctx context.Context) {
	for d := range mr.rnStateC {
		atomic.StoreUint64((*uint64)(&mr.raftState), uint64(d))
	}
}

func (mr *RaftMetadataRepository) sendAck(nodeIndex uint64, requestNum uint64, err error) {
	if mr.index != types.NodeID(nodeIndex) {
		return
	}

	f, ok := mr.requestMap.Load(requestNum)
	if !ok {
		return
	}

	c := f.(chan error)
	select {
	case c <- err:
	default:
	}
}

func (mr *RaftMetadataRepository) apply(e *pb.RaftEntry) {
	f := e.Request.GetValue()
	switch r := f.(type) {
	case *pb.RegisterStorageNode:
		mr.applyRegisterStorageNode(r, e.NodeIndex, e.RequestIndex)
	case *pb.UnregisterStorageNode:
		mr.applyUnregisterStorageNode(r, e.NodeIndex, e.RequestIndex)
	case *pb.RegisterLogStream:
		mr.applyRegisterLogStream(r, e.NodeIndex, e.RequestIndex)
	case *pb.UnregisterLogStream:
		mr.applyUnregisterLogStream(r, e.NodeIndex, e.RequestIndex)
	case *pb.UpdateLogStream:
		mr.applyUpdateLogStream(r, e.NodeIndex, e.RequestIndex)
	case *pb.Report:
		mr.applyReport(r)
	case *pb.Commit:
		mr.applyCommit()
	case *pb.Seal:
		mr.applySeal(r, e.NodeIndex, e.RequestIndex)
	case *pb.Unseal:
		mr.applyUnseal(r, e.NodeIndex, e.RequestIndex)
	}

	mr.storage.UpdateAppliedIndex(e.AppliedIndex)
}

func (mr *RaftMetadataRepository) applyRegisterStorageNode(r *pb.RegisterStorageNode, nodeIndex, requestIndex uint64) error {
	err := mr.storage.RegisterStorageNode(r.StorageNode, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	mr.reportCollector.RegisterStorageNode(r.StorageNode, mr.storage.GetHighWatermark())

	return nil
}

func (mr *RaftMetadataRepository) applyUnregisterStorageNode(r *pb.UnregisterStorageNode, nodeIndex, requestIndex uint64) error {
	err := mr.storage.UnregisterStorageNode(r.StorageNodeID, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	mr.reportCollector.UnregisterStorageNode(r.StorageNodeID)

	return nil
}

func (mr *RaftMetadataRepository) applyRegisterLogStream(r *pb.RegisterLogStream, nodeIndex, requestIndex uint64) error {
	err := mr.storage.RegisterLogStream(r.LogStream, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	return nil
}

func (mr *RaftMetadataRepository) applyUnregisterLogStream(r *pb.UnregisterLogStream, nodeIndex, requestIndex uint64) error {
	err := mr.storage.UnregisterLogStream(r.LogStreamID, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	return nil
}

func (mr *RaftMetadataRepository) applyUpdateLogStream(r *pb.UpdateLogStream, nodeIndex, requestIndex uint64) error {
	err := mr.storage.UpdateLogStream(r.LogStream, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	return nil
}

func (mr *RaftMetadataRepository) applyReport(r *pb.Report) error {
	snID := r.LogStream.StorageNodeID
	for _, l := range r.LogStream.Uncommit {
		lsID := l.LogStreamID

		u := &pb.MetadataRepositoryDescriptor_LocalLogStreamReplica{
			UncommittedLLSNOffset: l.UncommittedLLSNOffset,
			UncommittedLLSNLength: l.UncommittedLLSNLength,
			KnownHighWatermark:    r.LogStream.HighWatermark,
		}

		s := mr.storage.LookupLocalLogStreamReplica(lsID, snID)
		if s == nil || s.UncommittedLLSNEnd() < u.UncommittedLLSNEnd() {
			mr.storage.UpdateLocalLogStreamReplica(lsID, snID, u)
		}
	}

	return nil
}

func (mr *RaftMetadataRepository) applyCommit() error {
	curHWM := mr.storage.getHighWatermarkNoLock()
	trimHWM := types.MaxGLSN
	committedOffset := curHWM + types.GLSN(1)
	nrCommitted := uint64(0)

	gls := &snpb.GlobalLogStreamDescriptor{
		PrevHighWatermark: curHWM,
	}

	if mr.storage.NumUpdateSinceCommit() > 0 {
		lsIDs := mr.storage.GetLocalLogStreamIDs()

		for _, lsID := range lsIDs {
			replicas := mr.storage.LookupLocalLogStream(lsID)
			knownHWM, minHWM, nrUncommit := mr.calculateCommit(replicas)
			if minHWM < trimHWM {
				trimHWM = minHWM
			}

			if knownHWM != curHWM {
				nrCommitted := mr.numCommitSince(lsID, knownHWM)
				if nrCommitted > nrUncommit {
					mr.logger.Panic("# of uncommit should be bigger than # of commit",
						zap.Uint64("lsID", uint64(lsID)),
						zap.Uint64("known", uint64(knownHWM)),
						zap.Uint64("cur", uint64(curHWM)),
						zap.Uint64("uncommit", uint64(nrUncommit)),
						zap.Uint64("commit", uint64(nrCommitted)),
					)
				}

				nrUncommit -= nrCommitted
			}

			commit := &snpb.GlobalLogStreamDescriptor_LogStreamCommitResult{
				LogStreamID:         lsID,
				CommittedGLSNOffset: committedOffset,
				CommittedGLSNLength: nrUncommit,
			}

			if nrUncommit > 0 {
				committedOffset = commit.CommittedGLSNOffset + types.GLSN(commit.CommittedGLSNLength)
			} else {
				commit.CommittedGLSNOffset = mr.getLastCommitted(lsID)
				commit.CommittedGLSNLength = 0
			}

			gls.CommitResult = append(gls.CommitResult, commit)

			nrCommitted += nrUncommit
		}
	}
	gls.HighWatermark = curHWM + types.GLSN(nrCommitted)

	//fmt.Printf("commit %+v\n", gls)

	if nrCommitted > 0 {
		mr.storage.AppendGlobalLogStream(gls)

		if !trimHWM.Invalid() {
			mr.logger.Info("trim", zap.Uint64("hwm", uint64(trimHWM)))
			mr.storage.TrimGlobalLogStream(trimHWM)
		}

	}

	mr.reportCollector.Commit(gls)

	//TODO:: trigger next commit

	return nil
}

func (mr *RaftMetadataRepository) applySeal(r *pb.Seal, nodeIndex, requestIndex uint64) error {
	mr.applyCommit()
	err := mr.storage.SealLogStream(r.LogStreamID, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	return nil
}

func (mr *RaftMetadataRepository) applyUnseal(r *pb.Unseal, nodeIndex, requestIndex uint64) error {
	err := mr.storage.UnsealLogStream(r.LogStreamID, nodeIndex, requestIndex)
	if err != nil {
		return err
	}

	return nil
}

func getCommitResultFromGLS(gls *snpb.GlobalLogStreamDescriptor, lsId types.LogStreamID) *snpb.GlobalLogStreamDescriptor_LogStreamCommitResult {
	i := sort.Search(len(gls.CommitResult), func(i int) bool {
		return gls.CommitResult[i].LogStreamID >= lsId
	})

	if i < len(gls.CommitResult) && gls.CommitResult[i].LogStreamID == lsId {
		return gls.CommitResult[i]
	}

	return nil
}

func (mr *RaftMetadataRepository) numCommitSince(lsID types.LogStreamID, glsn types.GLSN) uint64 {
	var num uint64

	highest := mr.storage.getHighWatermarkNoLock()

	for glsn < highest {
		gls := mr.storage.lookupNextGLSNoLock(glsn)
		if gls == nil {
			mr.logger.Panic("gls should be exist",
				zap.Uint64("highest", uint64(highest)),
				zap.Uint64("cur", uint64(glsn)),
			)
		}

		r := getCommitResultFromGLS(gls, lsID)
		if r == nil {
			mr.logger.Panic("ls should be exist",
				zap.Uint64("lsID", uint64(lsID)),
				zap.Uint64("highest", uint64(highest)),
				zap.Uint64("cur", uint64(glsn)),
			)
		}

		num += uint64(r.CommittedGLSNLength)
		glsn = gls.HighWatermark
	}

	return num
}

func (mr *RaftMetadataRepository) calculateCommit(replicas *pb.MetadataRepositoryDescriptor_LocalLogStreamReplicas) (types.GLSN, types.GLSN, uint64) {
	var trimHWM types.GLSN = types.MaxGLSN
	var knownHWM types.GLSN = types.InvalidGLSN
	var beginLLSN types.LLSN = types.InvalidLLSN
	var endLLSN types.LLSN = types.InvalidLLSN

	if replicas == nil {
		return types.InvalidGLSN, types.InvalidGLSN, 0
	}

	if len(replicas.Replicas) < mr.nrReplica {
		return types.InvalidGLSN, types.InvalidGLSN, 0
	}

	for _, l := range replicas.Replicas {
		if beginLLSN.Invalid() || l.UncommittedLLSNOffset > beginLLSN {
			beginLLSN = l.UncommittedLLSNOffset
		}

		if endLLSN.Invalid() || l.UncommittedLLSNEnd() < endLLSN {
			endLLSN = l.UncommittedLLSNEnd()
		}

		if knownHWM.Invalid() || l.KnownHighWatermark > knownHWM {
			// knownHighWatermark 이 다르다면,
			// 일부 SN 이 commitResult 를 받지 못했을 뿐이다.
			knownHWM = l.KnownHighWatermark
		}

		if l.KnownHighWatermark < trimHWM {
			trimHWM = l.KnownHighWatermark
		}
	}

	if trimHWM == types.MaxGLSN {
		trimHWM = types.InvalidGLSN
	}

	if beginLLSN > endLLSN {
		return knownHWM, trimHWM, 0
	}

	return knownHWM, trimHWM, uint64(endLLSN - beginLLSN)
}

func (mr *RaftMetadataRepository) getLastCommitted(lsID types.LogStreamID) types.GLSN {
	gls := mr.storage.GetLastGLS()
	if gls == nil {
		return types.InvalidGLSN
	}

	r := getCommitResultFromGLS(gls, lsID)
	if r == nil {
		// newbie
		return types.InvalidGLSN
	}

	if r.CommittedGLSNLength == 0 {
		return r.CommittedGLSNOffset
	}

	return r.CommittedGLSNOffset + types.GLSN(r.CommittedGLSNLength-1)
}

func (mr *RaftMetadataRepository) proposeCommit() {
	if !mr.isLeader() {
		return
	}

	r := &pb.Commit{}
	mr.propose(context.TODO(), r, false)
}

func (mr *RaftMetadataRepository) proposeReport(lls *snpb.LocalLogStreamDescriptor) error {
	r := &pb.Report{
		LogStream: lls,
	}

	return mr.propose(context.TODO(), r, false)
}

func (mr *RaftMetadataRepository) propose(ctx context.Context, r interface{}, guarantee bool) error {
	e := &pb.RaftEntry{}
	e.Request.SetValue(r)
	e.NodeIndex = uint64(mr.index)
	e.RequestIndex = unusedRequestIndex

	if guarantee {
		c := make(chan error, 1)
		e.RequestIndex = atomic.AddUint64(&mr.requestNum, 1)
		mr.requestMap.Store(e.RequestIndex, c)
		defer mr.requestMap.Delete(e.RequestIndex)

		mr.proposeC <- e

		select {
		case err := <-c:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		select {
		case mr.proposeC <- e:
		default:
			return varlog.ErrIgnore
		}
	}

	return nil
}

func (mr *RaftMetadataRepository) proposeConfChange(ctx context.Context, r raftpb.ConfChange) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case mr.rnConfChangeC <- r:
	}

	return nil
}

func (mr *RaftMetadataRepository) RegisterStorageNode(ctx context.Context, sn *varlogpb.StorageNodeDescriptor) error {
	r := &pb.RegisterStorageNode{
		StorageNode: sn,
	}

	return mr.propose(ctx, r, true)
}

func (mr *RaftMetadataRepository) UnregisterStorageNode(ctx context.Context, snID types.StorageNodeID) error {
	r := &pb.UnregisterStorageNode{
		StorageNodeID: snID,
	}

	return mr.propose(ctx, r, true)
}

func (mr *RaftMetadataRepository) RegisterLogStream(ctx context.Context, ls *varlogpb.LogStreamDescriptor) error {
	r := &pb.RegisterLogStream{
		LogStream: ls,
	}

	return mr.propose(ctx, r, true)
}

func (mr *RaftMetadataRepository) UnregisterLogStream(ctx context.Context, lsID types.LogStreamID) error {
	r := &pb.UnregisterLogStream{
		LogStreamID: lsID,
	}

	return mr.propose(ctx, r, true)
}

func (mr *RaftMetadataRepository) UpdateLogStream(ctx context.Context, ls *varlogpb.LogStreamDescriptor) error {
	r := &pb.UpdateLogStream{
		LogStream: ls,
	}

	return mr.propose(ctx, r, true)
}

func (mr *RaftMetadataRepository) GetMetadata(ctx context.Context) (*varlogpb.MetadataDescriptor, error) {
	m := mr.storage.GetMetadata()
	return m, nil
}

func (mr *RaftMetadataRepository) Seal(ctx context.Context, lsID types.LogStreamID) (types.GLSN, error) {
	r := &pb.Seal{
		LogStreamID: lsID,
	}

	err := mr.propose(ctx, r, true)
	if err != nil && err != varlog.ErrIgnore {
		return types.InvalidGLSN, err
	}

	return mr.getLastCommitted(lsID), nil
}

func (mr *RaftMetadataRepository) Unseal(ctx context.Context, lsID types.LogStreamID) error {
	r := &pb.Unseal{
		LogStreamID: lsID,
	}

	err := mr.propose(ctx, r, true)
	if err != nil && err != varlog.ErrIgnore {
		return err
	}

	return nil
}

func (mr *RaftMetadataRepository) AddPeer(ctx context.Context, clusterID types.ClusterID, nodeID types.NodeID, url string) error {
	r := raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  uint64(nodeID),
		Context: []byte(url),
	}

	return mr.proposeConfChange(ctx, r)
}

func (mr *RaftMetadataRepository) RemovePeer(ctx context.Context, clusterID types.ClusterID, nodeID types.NodeID) error {
	r := raftpb.ConfChange{
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: uint64(nodeID),
	}

	return mr.proposeConfChange(ctx, r)
}

func (mr *RaftMetadataRepository) GetClusterInfo(ctx context.Context, clusterID types.ClusterID) (types.NodeID, []string, error) {
	return mr.raftNode.GetNodeID(), mr.raftNode.GetMembership(), nil
}
