// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kakao/varlog/internal/storagenode (interfaces: Scanner,WriteBatch,CommitBatch,Storage)

// Package storagenode is a generated GoMock package.
package storagenode

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	types "github.com/kakao/varlog/pkg/types"
)

// MockScanner is a mock of Scanner interface
type MockScanner struct {
	ctrl     *gomock.Controller
	recorder *MockScannerMockRecorder
}

// MockScannerMockRecorder is the mock recorder for MockScanner
type MockScannerMockRecorder struct {
	mock *MockScanner
}

// NewMockScanner creates a new mock instance
func NewMockScanner(ctrl *gomock.Controller) *MockScanner {
	mock := &MockScanner{ctrl: ctrl}
	mock.recorder = &MockScannerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockScanner) EXPECT() *MockScannerMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockScanner) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockScannerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockScanner)(nil).Close))
}

// Next mocks base method
func (m *MockScanner) Next() ScanResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(ScanResult)
	return ret0
}

// Next indicates an expected call of Next
func (mr *MockScannerMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockScanner)(nil).Next))
}

// MockWriteBatch is a mock of WriteBatch interface
type MockWriteBatch struct {
	ctrl     *gomock.Controller
	recorder *MockWriteBatchMockRecorder
}

// MockWriteBatchMockRecorder is the mock recorder for MockWriteBatch
type MockWriteBatchMockRecorder struct {
	mock *MockWriteBatch
}

// NewMockWriteBatch creates a new mock instance
func NewMockWriteBatch(ctrl *gomock.Controller) *MockWriteBatch {
	mock := &MockWriteBatch{ctrl: ctrl}
	mock.recorder = &MockWriteBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockWriteBatch) EXPECT() *MockWriteBatchMockRecorder {
	return m.recorder
}

// Apply mocks base method
func (m *MockWriteBatch) Apply() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Apply")
	ret0, _ := ret[0].(error)
	return ret0
}

// Apply indicates an expected call of Apply
func (mr *MockWriteBatchMockRecorder) Apply() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Apply", reflect.TypeOf((*MockWriteBatch)(nil).Apply))
}

// Close mocks base method
func (m *MockWriteBatch) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockWriteBatchMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockWriteBatch)(nil).Close))
}

// Put mocks base method
func (m *MockWriteBatch) Put(arg0 types.LLSN, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockWriteBatchMockRecorder) Put(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockWriteBatch)(nil).Put), arg0, arg1)
}

// MockCommitBatch is a mock of CommitBatch interface
type MockCommitBatch struct {
	ctrl     *gomock.Controller
	recorder *MockCommitBatchMockRecorder
}

// MockCommitBatchMockRecorder is the mock recorder for MockCommitBatch
type MockCommitBatchMockRecorder struct {
	mock *MockCommitBatch
}

// NewMockCommitBatch creates a new mock instance
func NewMockCommitBatch(ctrl *gomock.Controller) *MockCommitBatch {
	mock := &MockCommitBatch{ctrl: ctrl}
	mock.recorder = &MockCommitBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockCommitBatch) EXPECT() *MockCommitBatchMockRecorder {
	return m.recorder
}

// Apply mocks base method
func (m *MockCommitBatch) Apply() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Apply")
	ret0, _ := ret[0].(error)
	return ret0
}

// Apply indicates an expected call of Apply
func (mr *MockCommitBatchMockRecorder) Apply() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Apply", reflect.TypeOf((*MockCommitBatch)(nil).Apply))
}

// Close mocks base method
func (m *MockCommitBatch) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockCommitBatchMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockCommitBatch)(nil).Close))
}

// Put mocks base method
func (m *MockCommitBatch) Put(arg0 types.LLSN, arg1 types.GLSN) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockCommitBatchMockRecorder) Put(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockCommitBatch)(nil).Put), arg0, arg1)
}

// MockStorage is a mock of Storage interface
type MockStorage struct {
	ctrl     *gomock.Controller
	recorder *MockStorageMockRecorder
}

// MockStorageMockRecorder is the mock recorder for MockStorage
type MockStorageMockRecorder struct {
	mock *MockStorage
}

// NewMockStorage creates a new mock instance
func NewMockStorage(ctrl *gomock.Controller) *MockStorage {
	mock := &MockStorage{ctrl: ctrl}
	mock.recorder = &MockStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockStorage) EXPECT() *MockStorageMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockStorage) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockStorageMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockStorage)(nil).Close))
}

// Commit mocks base method
func (m *MockStorage) Commit(arg0 types.LLSN, arg1 types.GLSN) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit
func (mr *MockStorageMockRecorder) Commit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockStorage)(nil).Commit), arg0, arg1)
}

// DeleteCommitted mocks base method
func (m *MockStorage) DeleteCommitted(arg0 types.GLSN) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteCommitted", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteCommitted indicates an expected call of DeleteCommitted
func (mr *MockStorageMockRecorder) DeleteCommitted(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCommitted", reflect.TypeOf((*MockStorage)(nil).DeleteCommitted), arg0)
}

// DeleteUncommitted mocks base method
func (m *MockStorage) DeleteUncommitted(arg0 types.LLSN) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteUncommitted", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteUncommitted indicates an expected call of DeleteUncommitted
func (mr *MockStorageMockRecorder) DeleteUncommitted(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteUncommitted", reflect.TypeOf((*MockStorage)(nil).DeleteUncommitted), arg0)
}

// Name mocks base method
func (m *MockStorage) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name
func (mr *MockStorageMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockStorage)(nil).Name))
}

// NewCommitBatch mocks base method
func (m *MockStorage) NewCommitBatch() CommitBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewCommitBatch")
	ret0, _ := ret[0].(CommitBatch)
	return ret0
}

// NewCommitBatch indicates an expected call of NewCommitBatch
func (mr *MockStorageMockRecorder) NewCommitBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewCommitBatch", reflect.TypeOf((*MockStorage)(nil).NewCommitBatch))
}

// NewWriteBatch mocks base method
func (m *MockStorage) NewWriteBatch() WriteBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewWriteBatch")
	ret0, _ := ret[0].(WriteBatch)
	return ret0
}

// NewWriteBatch indicates an expected call of NewWriteBatch
func (mr *MockStorageMockRecorder) NewWriteBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewWriteBatch", reflect.TypeOf((*MockStorage)(nil).NewWriteBatch))
}

// Path mocks base method
func (m *MockStorage) Path() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Path")
	ret0, _ := ret[0].(string)
	return ret0
}

// Path indicates an expected call of Path
func (mr *MockStorageMockRecorder) Path() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Path", reflect.TypeOf((*MockStorage)(nil).Path))
}

// Read mocks base method
func (m *MockStorage) Read(arg0 types.GLSN) (types.LogEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", arg0)
	ret0, _ := ret[0].(types.LogEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read
func (mr *MockStorageMockRecorder) Read(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockStorage)(nil).Read), arg0)
}

// RestoreLogStreamContext mocks base method
func (m *MockStorage) RestoreLogStreamContext(arg0 *LogStreamContext) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RestoreLogStreamContext", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// RestoreLogStreamContext indicates an expected call of RestoreLogStreamContext
func (mr *MockStorageMockRecorder) RestoreLogStreamContext(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RestoreLogStreamContext", reflect.TypeOf((*MockStorage)(nil).RestoreLogStreamContext), arg0)
}

// RestoreStorage mocks base method
func (m *MockStorage) RestoreStorage(arg0 types.LLSN, arg1 types.GLSN) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RestoreStorage", arg0, arg1)
}

// RestoreStorage indicates an expected call of RestoreStorage
func (mr *MockStorageMockRecorder) RestoreStorage(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RestoreStorage", reflect.TypeOf((*MockStorage)(nil).RestoreStorage), arg0, arg1)
}

// Scan mocks base method
func (m *MockStorage) Scan(arg0, arg1 types.GLSN) (Scanner, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Scan", arg0, arg1)
	ret0, _ := ret[0].(Scanner)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Scan indicates an expected call of Scan
func (mr *MockStorageMockRecorder) Scan(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Scan", reflect.TypeOf((*MockStorage)(nil).Scan), arg0, arg1)
}

// StoreCommitContext mocks base method
func (m *MockStorage) StoreCommitContext(arg0 CommitContext) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreCommitContext", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// StoreCommitContext indicates an expected call of StoreCommitContext
func (mr *MockStorageMockRecorder) StoreCommitContext(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreCommitContext", reflect.TypeOf((*MockStorage)(nil).StoreCommitContext), arg0)
}

// Write mocks base method
func (m *MockStorage) Write(arg0 types.LLSN, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Write", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Write indicates an expected call of Write
func (mr *MockStorageMockRecorder) Write(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Write", reflect.TypeOf((*MockStorage)(nil).Write), arg0, arg1)
}
