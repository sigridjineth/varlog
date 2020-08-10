package metadata_repository

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/kakao/varlog/internal/storage"
	types "github.com/kakao/varlog/pkg/varlog/types"
	snpb "github.com/kakao/varlog/proto/storage_node"
	varlogpb "github.com/kakao/varlog/proto/varlog"
)

type EmptyReporterClient struct {
}

func (rc *EmptyReporterClient) GetReport(ctx context.Context) (*snpb.LocalLogStreamDescriptor, error) {
	return &snpb.LocalLogStreamDescriptor{}, nil
}

func (rc *EmptyReporterClient) Commit(ctx context.Context, gls *snpb.GlobalLogStreamDescriptor) error {
	return nil
}

func (rc *EmptyReporterClient) Close() error {
	return nil
}

type EmptyReporterClientFactory struct {
}

func NewEmptyReporterClientFactory() *EmptyReporterClientFactory {
	return &EmptyReporterClientFactory{}
}

func (rcf *EmptyReporterClientFactory) GetClient(*varlogpb.StorageNodeDescriptor) (storage.LogStreamReporterClient, error) {
	return &EmptyReporterClient{}, nil
}

type DummyReporterClientStatus int32

const (
	DUMMY_REPORTERCLIENT_STATUS_RUNNING DummyReporterClientStatus = iota
	DUMMY_REPORTERCLIENT_STATUS_CLOSED
	DUMMY_REPORTERCLIENT_STATUS_CRASH
)

type DummyReporterClient struct {
	storageNodeID      types.StorageNodeID
	knownHighWatermark types.GLSN

	logStreamID           types.LogStreamID
	uncommittedLLSNOffset types.LLSN
	uncommittedLLSNLength uint64

	manual bool
	mu     sync.Mutex

	status  DummyReporterClientStatus
	factory *DummyReporterClientFactory
}

type DummyReporterClientFactory struct {
	manual bool
	m      sync.Map
}

func NewDummyReporterClientFactory(manual bool) *DummyReporterClientFactory {
	a := &DummyReporterClientFactory{
		manual: manual,
	}

	return a
}

func (a *DummyReporterClientFactory) GetClient(sn *varlogpb.StorageNodeDescriptor) (storage.LogStreamReporterClient, error) {
	status := DUMMY_REPORTERCLIENT_STATUS_RUNNING

	cli := &DummyReporterClient{
		manual:                a.manual,
		storageNodeID:         sn.StorageNodeID,
		logStreamID:           types.LogStreamID(sn.StorageNodeID),
		uncommittedLLSNOffset: types.MinLLSN,
		uncommittedLLSNLength: 0,
		status:                status,
		factory:               a,
	}

	f, _ := a.m.LoadOrStore(sn.StorageNodeID, cli)

	return f.(*DummyReporterClient), nil
}

func (r *DummyReporterClient) GetReport(ctx context.Context) (*snpb.LocalLogStreamDescriptor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == DUMMY_REPORTERCLIENT_STATUS_CRASH {
		return nil, errors.New("crash")
	} else if r.status == DUMMY_REPORTERCLIENT_STATUS_CLOSED {
		return nil, errors.New("closed")
	}

	if !r.manual {
		r.uncommittedLLSNLength++
	}

	lls := &snpb.LocalLogStreamDescriptor{
		StorageNodeID: r.storageNodeID,
		HighWatermark: r.knownHighWatermark,
		Uncommit: []*snpb.LocalLogStreamDescriptor_LogStreamUncommitReport{
			{
				LogStreamID:           r.logStreamID,
				UncommittedLLSNOffset: r.uncommittedLLSNOffset,
				UncommittedLLSNLength: r.uncommittedLLSNLength,
			},
		},
	}

	return lls, nil
}

func (r *DummyReporterClient) Commit(ctx context.Context, glsn *snpb.GlobalLogStreamDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == DUMMY_REPORTERCLIENT_STATUS_CRASH {
		return errors.New("crash")
	} else if r.status == DUMMY_REPORTERCLIENT_STATUS_CLOSED {
		return errors.New("closed")
	}

	if !r.knownHighWatermark.Invalid() &&
		glsn.PrevHighWatermark != r.knownHighWatermark {
		return nil
	}

	r.knownHighWatermark = glsn.HighWatermark

	for _, result := range glsn.CommitResult {
		if result.LogStreamID != r.logStreamID {
			return errors.New("invalid log stream ID")
		}

		r.uncommittedLLSNOffset += types.LLSN(result.CommittedGLSNLength)
		r.uncommittedLLSNLength -= result.CommittedGLSNLength
	}

	return nil
}

func (r *DummyReporterClient) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != DUMMY_REPORTERCLIENT_STATUS_CRASH {
		r.status = DUMMY_REPORTERCLIENT_STATUS_CLOSED
		r.factory.m.Delete(r.storageNodeID)
	}

	return nil
}

func (a *DummyReporterClientFactory) lookupClient(snID types.StorageNodeID) *DummyReporterClient {
	f, ok := a.m.Load(snID)
	if !ok {
		return nil
	}

	return f.(*DummyReporterClient)
}

func (r *DummyReporterClient) increaseUncommitted() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.uncommittedLLSNLength++
}

func (r *DummyReporterClient) numUncommitted() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.uncommittedLLSNLength
}

func (a *DummyReporterClientFactory) crashRPC(snID types.StorageNodeID) {
	f, ok := a.m.Load(snID)
	if !ok {
		fmt.Printf("notfound\n")
		return
	}

	cli := f.(*DummyReporterClient)

	cli.mu.Lock()
	defer cli.mu.Unlock()

	cli.status = DUMMY_REPORTERCLIENT_STATUS_CRASH
}

func (a *DummyReporterClientFactory) recoverRPC(snID types.StorageNodeID) {
	f, ok := a.m.Load(snID)
	if !ok {
		return
	}

	old := f.(*DummyReporterClient)

	old.mu.Lock()
	defer old.mu.Unlock()

	cli := &DummyReporterClient{
		manual:                old.manual,
		storageNodeID:         old.storageNodeID,
		logStreamID:           old.logStreamID,
		uncommittedLLSNOffset: old.uncommittedLLSNOffset,
		uncommittedLLSNLength: old.uncommittedLLSNLength,
		status:                DUMMY_REPORTERCLIENT_STATUS_RUNNING,
		factory:               old.factory,
	}

	a.m.Store(snID, cli)
}
