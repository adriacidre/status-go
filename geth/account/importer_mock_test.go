// Code generated by MockGen. DO NOT EDIT.
// Source: geth/account/importer.go

// Package account is a generated GoMock package.
package account

import (
	gomock "github.com/golang/mock/gomock"
	extkeys "github.com/status-im/status-go/extkeys"
	reflect "reflect"
)

// MockextendedKeyImporter is a mock of extendedKeyImporter interface
type MockextendedKeyImporter struct {
	ctrl     *gomock.Controller
	recorder *MockextendedKeyImporterMockRecorder
}

// MockextendedKeyImporterMockRecorder is the mock recorder for MockextendedKeyImporter
type MockextendedKeyImporterMockRecorder struct {
	mock *MockextendedKeyImporter
}

// NewMockextendedKeyImporter creates a new mock instance
func NewMockextendedKeyImporter(ctrl *gomock.Controller) *MockextendedKeyImporter {
	mock := &MockextendedKeyImporter{ctrl: ctrl}
	mock.recorder = &MockextendedKeyImporterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockextendedKeyImporter) EXPECT() *MockextendedKeyImporterMockRecorder {
	return m.recorder
}

// Import mocks base method
func (m *MockextendedKeyImporter) Import(keyStore accountKeyStorer, extKey *extkeys.ExtendedKey, password string) (string, string, error) {
	ret := m.ctrl.Call(m, "Import", keyStore, extKey, password)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Import indicates an expected call of Import
func (mr *MockextendedKeyImporterMockRecorder) Import(keyStore, extKey, password interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Import", reflect.TypeOf((*MockextendedKeyImporter)(nil).Import), keyStore, extKey, password)
}