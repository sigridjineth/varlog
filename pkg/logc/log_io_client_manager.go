package logc

import (
	"io"
	"strconv"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/proto/varlogpb"
)

type LogClientManager interface {
	GetOrConnect(storageNodeID types.StorageNodeID, addr string) (LogIOClient, error)
	io.Closer
}

type logClientManager struct {
	m      sync.Map // map[types.StorageNodeID]*logIOClientProxy
	group  singleflight.Group
	logger *zap.Logger
}

var _ LogClientManager = (*logClientManager)(nil)

func NewLogClientManager(metadata *varlogpb.MetadataDescriptor, logger *zap.Logger) (mgr *logClientManager, err error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.Named("logclmanager")

	mgr = &logClientManager{
		logger: logger,
	}
	for _, sndesc := range metadata.GetStorageNodes() {
		storageNodeID := sndesc.GetStorageNodeID()
		addr := sndesc.GetAddress()
		if _, err = mgr.GetOrConnect(storageNodeID, addr); err != nil {
			break
		}
	}
	if err != nil {
		mgr.Close()
		mgr = nil
	}
	return mgr, err
}

func (mgr *logClientManager) Close() (err error) {
	mgr.m.Range(func(storageNodeID interface{}, logCL interface{}) bool {
		if e := logCL.(LogIOClient).Close(); e != nil {
			err = e
		}
		mgr.m.Delete(storageNodeID)
		return true
	})
	return err
}

func (mgr *logClientManager) GetOrConnect(storageNodeID types.StorageNodeID, addr string) (LogIOClient, error) {
	key := makeGroupKey(storageNodeID)
	lip, err, _ := mgr.group.Do(key, func() (interface{}, error) {
		lipTmp, ok := mgr.m.Load(storageNodeID)
		if ok {
			lip := lipTmp.(*logClientProxy)
			if !lip.closed.Load() {
				return lip, nil
			}
			lip.client.Close()
			mgr.m.Delete(storageNodeID)
		}

		logcl, err := NewLogIOClient(addr)
		if err != nil {
			return nil, err
		}

		lip := newLogIOProxy(logcl)
		mgr.m.Store(storageNodeID, lip)
		return lip, nil
	})
	return lip.(*logClientProxy), err
}

func makeGroupKey(storageNodeID types.StorageNodeID) string {
	return strconv.FormatUint(uint64(storageNodeID), 10)
}