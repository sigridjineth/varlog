// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kakao/varlog/internal/storagenode/replication (interfaces: Client)

// Package replication is a generated GoMock package.
package replication

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	types "github.com/kakao/varlog/pkg/types"
	snpb "github.com/kakao/varlog/proto/snpb"
	varlogpb "github.com/kakao/varlog/proto/varlogpb"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockClient) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockClientMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockClient)(nil).Close))
}

// PeerStorageNodeID mocks base method.
func (m *MockClient) PeerStorageNodeID() types.StorageNodeID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeerStorageNodeID")
	ret0, _ := ret[0].(types.StorageNodeID)
	return ret0
}

// PeerStorageNodeID indicates an expected call of PeerStorageNodeID.
func (mr *MockClientMockRecorder) PeerStorageNodeID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeerStorageNodeID", reflect.TypeOf((*MockClient)(nil).PeerStorageNodeID))
}

// Replicate mocks base method.
func (m *MockClient) Replicate(arg0 context.Context, arg1 types.LLSN, arg2 []byte, arg3 func(error)) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Replicate", arg0, arg1, arg2, arg3)
}

// Replicate indicates an expected call of Replicate.
func (mr *MockClientMockRecorder) Replicate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Replicate", reflect.TypeOf((*MockClient)(nil).Replicate), arg0, arg1, arg2, arg3)
}

// SyncInit mocks base method.
func (m *MockClient) SyncInit(arg0 context.Context, arg1 snpb.SyncRange) (snpb.SyncRange, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncInit", arg0, arg1)
	ret0, _ := ret[0].(snpb.SyncRange)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SyncInit indicates an expected call of SyncInit.
func (mr *MockClientMockRecorder) SyncInit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncInit", reflect.TypeOf((*MockClient)(nil).SyncInit), arg0, arg1)
}

// SyncReplicate mocks base method.
func (m *MockClient) SyncReplicate(arg0 context.Context, arg1 varlogpb.Replica, arg2 snpb.SyncPayload) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncReplicate", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncReplicate indicates an expected call of SyncReplicate.
func (mr *MockClientMockRecorder) SyncReplicate(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncReplicate", reflect.TypeOf((*MockClient)(nil).SyncReplicate), arg0, arg1, arg2)
}
