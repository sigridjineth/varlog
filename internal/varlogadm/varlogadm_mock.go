// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kakao/varlog/internal/varlogadm (interfaces: ClusterMetadataView,StorageNodeManager)

// Package varlogadm is a generated GoMock package.
package varlogadm

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	snc "github.com/kakao/varlog/pkg/snc"
	types "github.com/kakao/varlog/pkg/types"
	snpb "github.com/kakao/varlog/proto/snpb"
	varlogpb "github.com/kakao/varlog/proto/varlogpb"
)

// MockClusterMetadataView is a mock of ClusterMetadataView interface.
type MockClusterMetadataView struct {
	ctrl     *gomock.Controller
	recorder *MockClusterMetadataViewMockRecorder
}

// MockClusterMetadataViewMockRecorder is the mock recorder for MockClusterMetadataView.
type MockClusterMetadataViewMockRecorder struct {
	mock *MockClusterMetadataView
}

// NewMockClusterMetadataView creates a new mock instance.
func NewMockClusterMetadataView(ctrl *gomock.Controller) *MockClusterMetadataView {
	mock := &MockClusterMetadataView{ctrl: ctrl}
	mock.recorder = &MockClusterMetadataViewMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClusterMetadataView) EXPECT() *MockClusterMetadataViewMockRecorder {
	return m.recorder
}

// ClusterMetadata mocks base method.
func (m *MockClusterMetadataView) ClusterMetadata(arg0 context.Context) (*varlogpb.MetadataDescriptor, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClusterMetadata", arg0)
	ret0, _ := ret[0].(*varlogpb.MetadataDescriptor)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ClusterMetadata indicates an expected call of ClusterMetadata.
func (mr *MockClusterMetadataViewMockRecorder) ClusterMetadata(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClusterMetadata", reflect.TypeOf((*MockClusterMetadataView)(nil).ClusterMetadata), arg0)
}

// StorageNode mocks base method.
func (m *MockClusterMetadataView) StorageNode(arg0 context.Context, arg1 types.StorageNodeID) (*varlogpb.StorageNodeDescriptor, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageNode", arg0, arg1)
	ret0, _ := ret[0].(*varlogpb.StorageNodeDescriptor)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageNode indicates an expected call of StorageNode.
func (mr *MockClusterMetadataViewMockRecorder) StorageNode(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageNode", reflect.TypeOf((*MockClusterMetadataView)(nil).StorageNode), arg0, arg1)
}

// MockStorageNodeManager is a mock of StorageNodeManager interface.
type MockStorageNodeManager struct {
	ctrl     *gomock.Controller
	recorder *MockStorageNodeManagerMockRecorder
}

// MockStorageNodeManagerMockRecorder is the mock recorder for MockStorageNodeManager.
type MockStorageNodeManagerMockRecorder struct {
	mock *MockStorageNodeManager
}

// NewMockStorageNodeManager creates a new mock instance.
func NewMockStorageNodeManager(ctrl *gomock.Controller) *MockStorageNodeManager {
	mock := &MockStorageNodeManager{ctrl: ctrl}
	mock.recorder = &MockStorageNodeManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStorageNodeManager) EXPECT() *MockStorageNodeManagerMockRecorder {
	return m.recorder
}

// AddLogStream mocks base method.
func (m *MockStorageNodeManager) AddLogStream(arg0 context.Context, arg1 *varlogpb.LogStreamDescriptor) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddLogStream", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddLogStream indicates an expected call of AddLogStream.
func (mr *MockStorageNodeManagerMockRecorder) AddLogStream(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLogStream", reflect.TypeOf((*MockStorageNodeManager)(nil).AddLogStream), arg0, arg1)
}

// AddLogStreamReplica mocks base method.
func (m *MockStorageNodeManager) AddLogStreamReplica(arg0 context.Context, arg1 types.StorageNodeID, arg2 types.TopicID, arg3 types.LogStreamID, arg4 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddLogStreamReplica", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddLogStreamReplica indicates an expected call of AddLogStreamReplica.
func (mr *MockStorageNodeManagerMockRecorder) AddLogStreamReplica(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLogStreamReplica", reflect.TypeOf((*MockStorageNodeManager)(nil).AddLogStreamReplica), arg0, arg1, arg2, arg3, arg4)
}

// AddStorageNode mocks base method.
func (m *MockStorageNodeManager) AddStorageNode(arg0 snc.StorageNodeManagementClient) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddStorageNode", arg0)
}

// AddStorageNode indicates an expected call of AddStorageNode.
func (mr *MockStorageNodeManagerMockRecorder) AddStorageNode(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddStorageNode", reflect.TypeOf((*MockStorageNodeManager)(nil).AddStorageNode), arg0)
}

// Close mocks base method.
func (m *MockStorageNodeManager) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockStorageNodeManagerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockStorageNodeManager)(nil).Close))
}

// Contains mocks base method.
func (m *MockStorageNodeManager) Contains(arg0 types.StorageNodeID) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Contains", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Contains indicates an expected call of Contains.
func (mr *MockStorageNodeManagerMockRecorder) Contains(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Contains", reflect.TypeOf((*MockStorageNodeManager)(nil).Contains), arg0)
}

// ContainsAddress mocks base method.
func (m *MockStorageNodeManager) ContainsAddress(arg0 string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainsAddress", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// ContainsAddress indicates an expected call of ContainsAddress.
func (mr *MockStorageNodeManagerMockRecorder) ContainsAddress(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainsAddress", reflect.TypeOf((*MockStorageNodeManager)(nil).ContainsAddress), arg0)
}

// GetMetadata mocks base method.
func (m *MockStorageNodeManager) GetMetadata(arg0 context.Context, arg1 types.StorageNodeID) (*varlogpb.StorageNodeMetadataDescriptor, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMetadata", arg0, arg1)
	ret0, _ := ret[0].(*varlogpb.StorageNodeMetadataDescriptor)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMetadata indicates an expected call of GetMetadata.
func (mr *MockStorageNodeManagerMockRecorder) GetMetadata(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMetadata", reflect.TypeOf((*MockStorageNodeManager)(nil).GetMetadata), arg0, arg1)
}

// GetMetadataByAddr mocks base method.
func (m *MockStorageNodeManager) GetMetadataByAddr(arg0 context.Context, arg1 string) (snc.StorageNodeManagementClient, *varlogpb.StorageNodeMetadataDescriptor, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMetadataByAddr", arg0, arg1)
	ret0, _ := ret[0].(snc.StorageNodeManagementClient)
	ret1, _ := ret[1].(*varlogpb.StorageNodeMetadataDescriptor)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetMetadataByAddr indicates an expected call of GetMetadataByAddr.
func (mr *MockStorageNodeManagerMockRecorder) GetMetadataByAddr(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMetadataByAddr", reflect.TypeOf((*MockStorageNodeManager)(nil).GetMetadataByAddr), arg0, arg1)
}

// Refresh mocks base method.
func (m *MockStorageNodeManager) Refresh(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Refresh", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Refresh indicates an expected call of Refresh.
func (mr *MockStorageNodeManagerMockRecorder) Refresh(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Refresh", reflect.TypeOf((*MockStorageNodeManager)(nil).Refresh), arg0)
}

// RemoveLogStream mocks base method.
func (m *MockStorageNodeManager) RemoveLogStream(arg0 context.Context, arg1 types.StorageNodeID, arg2 types.TopicID, arg3 types.LogStreamID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveLogStream", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveLogStream indicates an expected call of RemoveLogStream.
func (mr *MockStorageNodeManagerMockRecorder) RemoveLogStream(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveLogStream", reflect.TypeOf((*MockStorageNodeManager)(nil).RemoveLogStream), arg0, arg1, arg2, arg3)
}

// RemoveStorageNode mocks base method.
func (m *MockStorageNodeManager) RemoveStorageNode(arg0 types.StorageNodeID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RemoveStorageNode", arg0)
}

// RemoveStorageNode indicates an expected call of RemoveStorageNode.
func (mr *MockStorageNodeManagerMockRecorder) RemoveStorageNode(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveStorageNode", reflect.TypeOf((*MockStorageNodeManager)(nil).RemoveStorageNode), arg0)
}

// Seal mocks base method.
func (m *MockStorageNodeManager) Seal(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID, arg3 types.GLSN) ([]varlogpb.LogStreamMetadataDescriptor, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Seal", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([]varlogpb.LogStreamMetadataDescriptor)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Seal indicates an expected call of Seal.
func (mr *MockStorageNodeManagerMockRecorder) Seal(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Seal", reflect.TypeOf((*MockStorageNodeManager)(nil).Seal), arg0, arg1, arg2, arg3)
}

// Sync mocks base method.
func (m *MockStorageNodeManager) Sync(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID, arg3, arg4 types.StorageNodeID, arg5 types.GLSN) (*snpb.SyncStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sync", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(*snpb.SyncStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Sync indicates an expected call of Sync.
func (mr *MockStorageNodeManagerMockRecorder) Sync(arg0, arg1, arg2, arg3, arg4, arg5 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sync", reflect.TypeOf((*MockStorageNodeManager)(nil).Sync), arg0, arg1, arg2, arg3, arg4, arg5)
}

// Unseal mocks base method.
func (m *MockStorageNodeManager) Unseal(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unseal", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unseal indicates an expected call of Unseal.
func (mr *MockStorageNodeManagerMockRecorder) Unseal(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unseal", reflect.TypeOf((*MockStorageNodeManager)(nil).Unseal), arg0, arg1, arg2)
}