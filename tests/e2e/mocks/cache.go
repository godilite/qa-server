package mocks

import (
	"context"
	"database/sql"
	"time"
)

type InMemoryCache struct{}

func (c *InMemoryCache) Get(ctx context.Context, key string, dest any) error {
	return sql.ErrNoRows
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value any, exp time.Duration) error {
	return nil
}

func (c *InMemoryCache) Close() error {
	return nil
}

type TrackingCache struct {
	GetCalls int
	SetCalls int
	data     map[string]CacheEntry
}

type CacheEntry struct {
	Value  any
	Expiry time.Time
}

func NewTrackingCache() *TrackingCache {
	return &TrackingCache{
		data: make(map[string]CacheEntry),
	}
}

func (c *TrackingCache) Get(ctx context.Context, key string, dest any) error {
	c.GetCalls++
	if entry, exists := c.data[key]; exists && time.Now().Before(entry.Expiry) {
		return nil
	}
	return sql.ErrNoRows
}

func (c *TrackingCache) Set(ctx context.Context, key string, value any, exp time.Duration) error {
	c.SetCalls++
	c.data[key] = CacheEntry{
		Value:  value,
		Expiry: time.Now().Add(exp),
	}
	return nil
}

func (c *TrackingCache) Close() error {
	return nil
}
