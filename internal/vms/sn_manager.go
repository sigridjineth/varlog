package vms

import (
	"context"
	"errors"
	"sync"

	"github.com/kakao/varlog/pkg/varlog"
	"github.com/kakao/varlog/pkg/varlog/types"
	snpb "github.com/kakao/varlog/proto/storage_node"
	vpb "github.com/kakao/varlog/proto/varlog"
	"go.uber.org/zap"
)

type StorageNodeManager interface {
	FindByStorageNodeID(storageNodeID types.StorageNodeID) varlog.ManagementClient

	FindByAddress(addr string) varlog.ManagementClient

	// GetMetadataByAddr returns metadata about a storage node. It is useful when id of the
	// storage node is not known.
	GetMetadataByAddr(ctx context.Context, addr string) (varlog.ManagementClient, *vpb.StorageNodeMetadataDescriptor, error)

	AddStorageNode(ctx context.Context, snmcl varlog.ManagementClient) error

	AddLogStream(ctx context.Context, logStreamDesc *vpb.LogStreamDescriptor) error

	Seal(ctx context.Context, logStreamID types.LogStreamID, lastCommittedGLSN types.GLSN) error

	Sync(ctx context.Context, logStreamID types.LogStreamID) error

	Unseal(ctx context.Context, logStreamID types.LogStreamID) error

	Close() error
}

var _ StorageNodeManager = (*snManager)(nil)

type snManager struct {
	clusterID types.ClusterID

	cs     map[types.StorageNodeID]varlog.ManagementClient
	cmView ClusterMetadataView
	mu     sync.RWMutex

	logger *zap.Logger
}

func NewStorageNodeManager(cmView ClusterMetadataView, logger *zap.Logger) StorageNodeManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.Named("snmanager")
	return &snManager{
		cmView: cmView,
		cs:     make(map[types.StorageNodeID]varlog.ManagementClient),
		logger: logger,
	}
}

func (sm *snManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var err error
	for id, cli := range sm.cs {
		if e := cli.Close(); e != nil {
			sm.logger.Error("could not close storagenode management client", zap.Any("snid", id), zap.Error(e))
			err = e
		}
	}
	return err
}

func (sm *snManager) FindByStorageNodeID(storageNodeID types.StorageNodeID) varlog.ManagementClient {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	snmcl, _ := sm.cs[storageNodeID]
	return snmcl
}

func (sm *snManager) FindByAddress(addr string) varlog.ManagementClient {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, snmcl := range sm.cs {
		if snmcl.PeerAddress() == addr {
			return snmcl
		}
	}
	return nil
}

func (sm *snManager) GetMetadataByAddr(ctx context.Context, addr string) (varlog.ManagementClient, *vpb.StorageNodeMetadataDescriptor, error) {
	mc, err := varlog.NewManagementClient(ctx, sm.clusterID, addr, sm.logger)
	if err != nil {
		sm.logger.Error("could not create storagenode management client", zap.Error(err))
		return nil, nil, err
	}
	snMeta, err := mc.GetMetadata(ctx, snpb.MetadataTypeHeartbeat)
	if err != nil {
		if err := mc.Close(); err != nil {
			sm.logger.Error("could not close storagenode management client", zap.Error(err))
			return nil, nil, err
		}
		return nil, nil, err
	}
	return mc, snMeta, nil
}

func (sm *snManager) AddStorageNode(ctx context.Context, snmcl varlog.ManagementClient) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	storageNodeID := snmcl.PeerStorageNodeID()
	if _, ok := sm.cs[storageNodeID]; ok {
		sm.logger.Panic("already registered storagenode", zap.Any("snid", storageNodeID))
	}
	sm.cs[storageNodeID] = snmcl
	return nil
}

func (sm *snManager) AddLogStream(ctx context.Context, logStreamDesc *vpb.LogStreamDescriptor) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logStreamID := logStreamDesc.GetLogStreamID()
	replicaDescList := logStreamDesc.GetReplicas()
	for _, replicaDesc := range replicaDescList {
		storageNodeID := replicaDesc.GetStorageNodeID()
		cli, ok := sm.cs[storageNodeID]
		if !ok {
			sm.logger.Panic("storagenodemanager: no such storage node", zap.Any("snid", storageNodeID))
		}

		// TODO (jun): Currently, it raises an error when the logStreamID already exists.
		// It is okay?
		snmeta, err := cli.GetMetadata(ctx, snpb.MetadataTypeLogStreams)
		if err != nil {
			return err
		}
		if _, found := snmeta.FindLogStream(logStreamID); found {
			return errors.New("vms: alredy exist logstream")
		}

		if err := cli.AddLogStream(ctx, logStreamID, replicaDesc.GetPath()); err != nil {
			return err
		}
	}
	return nil
}

func (sm *snManager) Seal(ctx context.Context, logStreamID types.LogStreamID, lastCommittedGLSN types.GLSN) error {
	panic("not implemented")
}

func (sm *snManager) Sync(ctx context.Context, logStreamID types.LogStreamID) error {
	panic("not implemented")
}

func (sm *snManager) Unseal(ctx context.Context, logStreamID types.LogStreamID) error {
	panic("not implemented")
}
