// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kakao/varlog/pkg/logc (interfaces: LogClientManager)

// Package logc is a generated GoMock package.
package logc

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	types "github.com/kakao/varlog/pkg/types"
)

// MockLogClientManager is a mock of LogClientManager interface
type MockLogClientManager struct {
	ctrl     *gomock.Controller
	recorder *MockLogClientManagerMockRecorder
}

// MockLogClientManagerMockRecorder is the mock recorder for MockLogClientManager
type MockLogClientManagerMockRecorder struct {
	mock *MockLogClientManager
}

// NewMockLogClientManager creates a new mock instance
func NewMockLogClientManager(ctrl *gomock.Controller) *MockLogClientManager {
	mock := &MockLogClientManager{ctrl: ctrl}
	mock.recorder = &MockLogClientManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockLogClientManager) EXPECT() *MockLogClientManagerMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockLogClientManager) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockLogClientManagerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockLogClientManager)(nil).Close))
}

// GetOrConnect mocks base method
func (m *MockLogClientManager) GetOrConnect(arg0 types.StorageNodeID, arg1 string) (LogIOClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOrConnect", arg0, arg1)
	ret0, _ := ret[0].(LogIOClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOrConnect indicates an expected call of GetOrConnect
func (mr *MockLogClientManagerMockRecorder) GetOrConnect(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOrConnect", reflect.TypeOf((*MockLogClientManager)(nil).GetOrConnect), arg0, arg1)
}
