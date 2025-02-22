// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// AuthCallback is an autogenerated mock type for the AuthCallback type
type AuthCallback struct {
	mock.Mock
}

// OnSetupComplete provides a mock function with given fields: status, err
func (_m *AuthCallback) OnSetupComplete(status int, err string) {
	_m.Called(status, err)
}

type mockConstructorTestingTNewAuthCallback interface {
	mock.TestingT
	Cleanup(func())
}

// NewAuthCallback creates a new instance of AuthCallback. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAuthCallback(t mockConstructorTestingTNewAuthCallback) *AuthCallback {
	mock := &AuthCallback{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
