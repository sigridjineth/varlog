// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kakao/varlog/pkg/logc (interfaces: LogIOClient)

// Package logc is a generated GoMock package.
package logc

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	types "github.com/kakao/varlog/pkg/types"
	snpb "github.com/kakao/varlog/proto/snpb"
	varlogpb "github.com/kakao/varlog/proto/varlogpb"
)

// MockLogIOClient is a mock of LogIOClient interface.
type MockLogIOClient struct {
	ctrl     *gomock.Controller
	recorder *MockLogIOClientMockRecorder
}

// MockLogIOClientMockRecorder is the mock recorder for MockLogIOClient.
type MockLogIOClientMockRecorder struct {
	mock *MockLogIOClient
}

// NewMockLogIOClient creates a new mock instance.
func NewMockLogIOClient(ctrl *gomock.Controller) *MockLogIOClient {
	mock := &MockLogIOClient{ctrl: ctrl}
	mock.recorder = &MockLogIOClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLogIOClient) EXPECT() *MockLogIOClientMockRecorder {
	return m.recorder
}

// Append mocks base method.
func (m *MockLogIOClient) Append(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID, arg3 [][]byte, arg4 ...varlogpb.StorageNode) ([]snpb.AppendResult, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2, arg3}
	for _, a := range arg4 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Append", varargs...)
	ret0, _ := ret[0].([]snpb.AppendResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Append indicates an expected call of Append.
func (mr *MockLogIOClientMockRecorder) Append(arg0, arg1, arg2, arg3 interface{}, arg4 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2, arg3}, arg4...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Append", reflect.TypeOf((*MockLogIOClient)(nil).Append), varargs...)
}

// Close mocks base method.
func (m *MockLogIOClient) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockLogIOClientMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockLogIOClient)(nil).Close))
}

// LogStreamMetadata mocks base method.
func (m *MockLogIOClient) LogStreamMetadata(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID) (varlogpb.LogStreamDescriptor, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LogStreamMetadata", arg0, arg1, arg2)
	ret0, _ := ret[0].(varlogpb.LogStreamDescriptor)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LogStreamMetadata indicates an expected call of LogStreamMetadata.
func (mr *MockLogIOClientMockRecorder) LogStreamMetadata(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LogStreamMetadata", reflect.TypeOf((*MockLogIOClient)(nil).LogStreamMetadata), arg0, arg1, arg2)
}

// Read mocks base method.
func (m *MockLogIOClient) Read(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID, arg3 types.GLSN) (*varlogpb.LogEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*varlogpb.LogEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockLogIOClientMockRecorder) Read(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockLogIOClient)(nil).Read), arg0, arg1, arg2, arg3)
}

// Subscribe mocks base method.
func (m *MockLogIOClient) Subscribe(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID, arg3, arg4 types.GLSN) (<-chan SubscribeResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(<-chan SubscribeResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Subscribe indicates an expected call of Subscribe.
func (mr *MockLogIOClientMockRecorder) Subscribe(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockLogIOClient)(nil).Subscribe), arg0, arg1, arg2, arg3, arg4)
}

// SubscribeTo mocks base method.
func (m *MockLogIOClient) SubscribeTo(arg0 context.Context, arg1 types.TopicID, arg2 types.LogStreamID, arg3, arg4 types.LLSN) (<-chan SubscribeResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeTo", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(<-chan SubscribeResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeTo indicates an expected call of SubscribeTo.
func (mr *MockLogIOClientMockRecorder) SubscribeTo(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeTo", reflect.TypeOf((*MockLogIOClient)(nil).SubscribeTo), arg0, arg1, arg2, arg3, arg4)
}

// TrimDeprecated mocks base method.
func (m *MockLogIOClient) TrimDeprecated(arg0 context.Context, arg1 types.TopicID, arg2 types.GLSN) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TrimDeprecated", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// TrimDeprecated indicates an expected call of TrimDeprecated.
func (mr *MockLogIOClientMockRecorder) TrimDeprecated(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TrimDeprecated", reflect.TypeOf((*MockLogIOClient)(nil).TrimDeprecated), arg0, arg1, arg2)
}
