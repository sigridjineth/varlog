package mrconnector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"go.uber.org/zap"

	"github.com/kakao/varlog/pkg/mrc"
	"github.com/kakao/varlog/pkg/rpc"
	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/pkg/util/runner"
	"github.com/kakao/varlog/pkg/verrors"
	"github.com/kakao/varlog/proto/mrpb"
)

// Connector represents a connection proxy for the metadata repository. It contains clients and
// management clients for the metadata repository.
type Connector interface {
	io.Closer

	ClusterID() types.ClusterID

	NumberOfMR() int

	ConnectedNodeID() types.NodeID

	// TODO (jun): use context to indicate that it can be network communication.
	Client() (mrc.MetadataRepositoryClient, error)

	// TODO (jun): use context to indicate that it can be network communication.
	ManagementClient() (mrc.MetadataRepositoryManagementClient, error)

	AddRPCAddr(nodeID types.NodeID, addr string)

	DelRPCAddr(nodeID types.NodeID)
}

type connector struct {
	clusterID types.ClusterID

	rpcAddrs         sync.Map     // map[types.NodeID]string
	connectedMRProxy atomic.Value // *mrProxy
	group            singleflight.Group

	runner *runner.Runner
	cancel context.CancelFunc

	logger  *zap.Logger
	options options
}

func New(ctx context.Context, seedRPCAddrs []string, opts ...Option) (Connector, error) {
	if len(seedRPCAddrs) == 0 {
		return nil, verrors.ErrInvalid
	}

	mrcOpts := defaultOptions
	for _, opt := range opts {
		opt(&mrcOpts)
	}
	mrcOpts.logger = mrcOpts.logger.Named("mrconnector")

	mrc := &connector{
		clusterID: mrcOpts.clusterID,
		runner:    runner.New("mrconnector", mrcOpts.logger),
		logger:    mrcOpts.logger,
		options:   mrcOpts,
	}

	tctx, cancel := context.WithTimeout(ctx, mrc.options.connectionTimeout)
	defer cancel()

	rpcAddrs, err := mrc.fetchRPCAddrs(tctx, seedRPCAddrs)
	if err != nil {
		return nil, err
	}
	mrc.updateRPCAddrs(rpcAddrs)
	if _, err = mrc.connect(); err != nil {
		return nil, err
	}

	mrc.cancel, err = mrc.runner.Run(mrc.fetchAndUpdate)
	if err != nil {
		return nil, err
	}

	return mrc, nil
}

func (c *connector) Close() (err error) {
	c.cancel()
	c.runner.Stop()
	if proxy := c.connectedProxy(); proxy != nil {
		err = proxy.Close()
	}
	return err
}

func (c *connector) ClusterID() types.ClusterID {
	return c.clusterID
}

func (c *connector) NumberOfMR() int {
	ret := 0
	c.rpcAddrs.Range(func(_ interface{}, _ interface{}) bool {
		ret++
		return true
	})
	return ret
}

func (c *connector) Client() (mrc.MetadataRepositoryClient, error) {
	return c.connect()
}

func (c *connector) ManagementClient() (mrc.MetadataRepositoryManagementClient, error) {
	return c.connect()
}

func (c *connector) ConnectedNodeID() types.NodeID {
	mrProxy, err := c.connect()
	if err != nil {
		return types.InvalidNodeID
	}
	return mrProxy.nodeID
}

func (c *connector) releaseMRProxy(nodeID types.NodeID) {
	c.rpcAddrs.Delete(nodeID)
}

func (c *connector) fetchAndUpdate(ctx context.Context) {
	ticker := time.NewTicker(c.options.clusterInfoFetchInterval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.update(ctx); err != nil {
				c.logger.Error("could not update")
			}
		}
	}
}

func (c *connector) AddRPCAddr(nodeID types.NodeID, addr string) {
	c.rpcAddrs.Store(nodeID, addr)
}

func (c *connector) DelRPCAddr(nodeID types.NodeID) {
	if proxy := c.connectedProxy(); proxy != nil && proxy.nodeID == nodeID {
		proxy.Close()
	}
}

func (c *connector) update(ctx context.Context) error {
	mrmcl, err := c.ManagementClient()
	if err != nil {
		return fmt.Errorf("mrconnector: client connection error (%w)", err)
	}

	rpcAddrs, err := getRPCAddrs(ctx, mrmcl, c.clusterID)
	if err != nil {
		return fmt.Errorf("mrconnector: clusterinfo fetch error (%w)", err)
	}
	if len(rpcAddrs) == 0 {
		return errors.New("mrconnector: number of mr is zero")
	}

	c.updateRPCAddrs(rpcAddrs)

	if proxy := c.connectedProxy(); proxy != nil {
		if _, ok := c.rpcAddrs.Load(proxy.nodeID); !ok {
			if err := proxy.Close(); err != nil {
				c.logger.Error("error while closing mr client", zap.Error(err))
			}
		}
	}
	return nil
}

func (c *connector) fetchRPCAddrs(ctx context.Context, seedRPCAddrs []string) (rpcAddrs map[types.NodeID]string, err error) {
	for ctx.Err() == nil {
		for _, rpcAddr := range seedRPCAddrs {
			rpcAddrs, err = c.connectMRAndFetchRPCAddrs(ctx, rpcAddr)
			if err == nil && len(rpcAddrs) > 0 {
				return rpcAddrs, nil
			}
			time.Sleep(c.options.rpcAddrsFetchRetryInterval)
		}
	}
	return nil, err
}

func (c *connector) connectMRAndFetchRPCAddrs(ctx context.Context, rpcAddr string) (map[types.NodeID]string, error) {
	mrmcl, err := mrc.NewMetadataRepositoryManagementClient(rpcAddr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := mrmcl.Close(); err != nil {
			c.logger.Warn("error while closing mrc management client", zap.Error(err))
		}
	}()
	return getRPCAddrs(ctx, mrmcl, c.clusterID)
}

func getRPCAddrs(ctx context.Context, mrmcl mrc.MetadataRepositoryManagementClient, clusterID types.ClusterID) (map[types.NodeID]string, error) {
	rsp, err := mrmcl.GetClusterInfo(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	clusterInfo := rsp.GetClusterInfo()
	return makeRPCAddrs(clusterInfo), nil
}

func (c *connector) connectedProxy() *mrProxy {
	if proxy := c.connectedMRProxy.Load(); proxy != nil {
		return proxy.(*mrProxy)
	}
	return nil
}

func (c *connector) connect() (*mrProxy, error) {
	proxy, err, _ := c.group.Do("connect", func() (interface{}, error) {
		if proxy := c.connectedProxy(); proxy != nil && !proxy.disconnected.Load() {
			return proxy, nil
		}

		var (
			err   error
			mrcl  mrc.MetadataRepositoryClient
			mrmcl mrc.MetadataRepositoryManagementClient
			proxy *mrProxy
		)

		c.rpcAddrs.Range(func(nodeID interface{}, addr interface{}) bool {
			if addr == "" || nodeID.(types.NodeID) == types.InvalidNodeID {
				return true
			}
			mrcl, mrmcl, err = connectToMR(c.clusterID, addr.(string))
			if err != nil {
				c.logger.Debug("could not connect to MR", zap.Error(err), zap.Any("node_id", nodeID), zap.Any("addr", addr))
				return true
			}
			proxy = &mrProxy{
				cl:     mrcl,
				mcl:    mrmcl,
				nodeID: nodeID.(types.NodeID),
				c:      c,
			}
			proxy.disconnected.Store(false)
			c.connectedMRProxy.Store(proxy)
			return false
		})
		return proxy, err
	})
	return proxy.(*mrProxy), err
}

func (c *connector) updateRPCAddrs(newAddrs map[types.NodeID]string) {
	c.rpcAddrs.Range(func(nodeID interface{}, addr interface{}) bool {
		if _, ok := newAddrs[nodeID.(types.NodeID)]; !ok {
			c.rpcAddrs.Delete(nodeID)
		}
		return true
	})
	for nodeID, addr := range newAddrs {
		c.rpcAddrs.Store(nodeID, addr)
	}
}

func makeRPCAddrs(clusterInfo *mrpb.ClusterInfo) map[types.NodeID]string {
	members := clusterInfo.GetMembers()
	addrs := make(map[types.NodeID]string, len(members))
	for nodeID, member := range members {
		endpoint := member.GetEndpoint()
		if endpoint != "" {
			addrs[nodeID] = endpoint
		}
	}
	return addrs
}

func connectToMR(clusterID types.ClusterID, addr string) (mrcl mrc.MetadataRepositoryClient, mrmcl mrc.MetadataRepositoryManagementClient, err error) {
	// TODO (jun): rpc.NewConn is a blocking function, thus it should be specified by an
	// explicit timeout parameter.
	var conn *rpc.Conn
	conn, err = rpc.NewConn(addr)
	if err != nil {
		return nil, nil, err
	}
	mrcl, err = mrc.NewMetadataRepositoryClientFromRpcConn(conn)
	if err != nil {
		goto ErrOut
	}
	mrmcl, err = mrc.NewMetadataRepositoryManagementClientFromRpcConn(conn)
	if err != nil {
		goto ErrOut
	}
	return mrcl, mrmcl, nil

ErrOut:
	if mrcl != nil {
		mrcl.Close()
	}
	if mrmcl != nil {
		mrmcl.Close()
	}
	conn.Close()
	return nil, nil, err
}