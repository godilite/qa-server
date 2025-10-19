package mocks

import (
	"context"
	"errors"
	"time"
)

// MockCacher is a mock implementation of the cache interface
// for testing the handler layer. It uses function-based mocking for flexibility.
type MockCacher struct {
	GetFunc   func(ctx context.Context, key string, dest any) error
	SetFunc   func(ctx context.Context, key string, value any, expiration time.Duration) error
	CloseFunc func() error
}

// Get implements the cache interface
func (m *MockCacher) Get(ctx context.Context, key string, dest any) error {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key, dest)
	}
	return errors.New("cache miss")
}

// Set implements the cache interface
func (m *MockCacher) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, expiration)
	}
	return nil
}

// Close implements the cache interface
func (m *MockCacher) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
