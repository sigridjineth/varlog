// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kakao/varlog/internal/storagenode_deprecated/reportcommitter (interfaces: Server)

// Package reportcommitter is a generated GoMock package.
package reportcommitter

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	grpc "google.golang.org/grpc"

	snpb "github.com/kakao/varlog/proto/snpb"
)

// MockServer is a mock of Server interface.
type MockServer struct {
	ctrl     *gomock.Controller
	recorder *MockServerMockRecorder
}

// MockServerMockRecorder is the mock recorder for MockServer.
type MockServerMockRecorder struct {
	mock *MockServer
}

// NewMockServer creates a new mock instance.
func NewMockServer(ctrl *gomock.Controller) *MockServer {
	mock := &MockServer{ctrl: ctrl}
	mock.recorder = &MockServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockServer) EXPECT() *MockServerMockRecorder {
	return m.recorder
}

// Commit mocks base method.
func (m *MockServer) Commit(arg0 snpb.LogStreamReporter_CommitServer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit.
func (mr *MockServerMockRecorder) Commit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockServer)(nil).Commit), arg0)
}

// GetReport mocks base method.
func (m *MockServer) GetReport(arg0 snpb.LogStreamReporter_GetReportServer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetReport", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetReport indicates an expected call of GetReport.
func (mr *MockServerMockRecorder) GetReport(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReport", reflect.TypeOf((*MockServer)(nil).GetReport), arg0)
}

// Register mocks base method.
func (m *MockServer) Register(arg0 *grpc.Server) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Register", arg0)
}

// Register indicates an expected call of Register.
func (mr *MockServerMockRecorder) Register(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockServer)(nil).Register), arg0)
}