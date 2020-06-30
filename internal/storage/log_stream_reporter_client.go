package storage

import (
	"context"

	"github.com/gogo/protobuf/types"
	"github.com/kakao/varlog/pkg/varlog"
	pb "github.com/kakao/varlog/proto/storage_node"
)

// LogStreamReporterClient contains the functionality of bi-directional communication about local
// log stream and global log stream.
type LogStreamReporterClient interface {
	GetReport(context.Context) (*pb.LocalLogStreamDescriptor, error)
	Commit(context.Context, *pb.GlobalLogStreamDescriptor) error
	Close() error
}

type logStreamReporterClient struct {
	rpcConn   *varlog.RpcConn
	rpcClient pb.LogStreamReporterServiceClient
}

func NewLogStreamReporterClient(address string) (LogStreamReporterClient, error) {
	rpcConn, err := varlog.NewRpcConn(address)
	if err != nil {
		return nil, err
	}
	return NewLogStreamReporterClientFromRpcConn(rpcConn)
}

func NewLogStreamReporterClientFromRpcConn(rpcConn *varlog.RpcConn) (LogStreamReporterClient, error) {
	return &logStreamReporterClient{
		rpcConn:   rpcConn,
		rpcClient: pb.NewLogStreamReporterServiceClient(rpcConn.Conn),
	}, nil
}

func (c *logStreamReporterClient) GetReport(ctx context.Context) (*pb.LocalLogStreamDescriptor, error) {
	return c.rpcClient.GetReport(ctx, &types.Empty{})
}

func (c *logStreamReporterClient) Commit(ctx context.Context, gls *pb.GlobalLogStreamDescriptor) error {
	_, err := c.rpcClient.Commit(ctx, gls)
	return err
}

func (c *logStreamReporterClient) Close() error {
	return c.rpcConn.Close()
}
