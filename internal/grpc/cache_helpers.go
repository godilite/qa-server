package grpc

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type FetchFunc[T any] func(ctx context.Context) (T, error)

const (
	defaultFetchTimeout = 15 * time.Second
	defaultSetTimeout   = 5 * time.Second
)

// addTTLJitter adds up to ±30s random jitter to TTL to avoid mass expiration.
func addTTLJitter(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	jitter := time.Duration(rand.Intn(30)-15) * time.Second
	return ttl + jitter
}

func triggerBackgroundRefresh[T any](
	c Cacher,
	sf *singleflight.Group,
	key string,
	ttl time.Duration,
	logger *zap.Logger,
	fn FetchFunc[T],
) {
	go func() {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

		_, _, _ = sf.Do(key+":refresh", func() (any, error) {
			ctx, cancel := context.WithTimeout(context.Background(), defaultFetchTimeout)
			defer cancel()

			value, err := fn(ctx)
			if err != nil {
				logger.Warn("background refresh failed",
					zap.String("key", key),
					zap.Error(err))
				return nil, err
			}

			setCtx, cancelSet := context.WithTimeout(context.Background(), defaultSetTimeout)
			defer cancelSet()

			ttlWithJitter := addTTLJitter(ttl)
			if err := c.Set(setCtx, key, value, ttlWithJitter); err != nil {
				logger.Warn("failed to update cache in background",
					zap.String("key", key),
					zap.Error(err))
			} else {
				logger.Debug("cache refreshed in background",
					zap.String("key", key),
					zap.Duration("ttl", ttlWithJitter))
			}

			return value, nil
		})
	}()
}

func fetchAndCacheInBackground[T any](
	ctx context.Context,
	c Cacher,
	key string,
	ttl time.Duration,
	logger *zap.Logger,
	fn FetchFunc[T],
) (T, error) {
	var zero T

	value, err := fn(ctx)
	if err != nil {
		logger.Error("fetch failed", zap.String("key", key), zap.Error(err))
		return zero, err
	}

	go func(v T) {
		setCtx, cancel := context.WithTimeout(context.Background(), defaultSetTimeout)
		defer cancel()

		ttlWithJitter := addTTLJitter(ttl)
		if err := c.Set(setCtx, key, v, ttlWithJitter); err != nil {
			logger.Warn("failed to set cache on miss", zap.String("key", key), zap.Error(err))
		} else {
			logger.Debug("cache populated on miss", zap.String("key", key))
		}
	}(value)

	return value, nil
}

// FindAndCache implements read-through caching with singleflight and refresh-ahead logic.
func FindAndCache[T any](
	ctx context.Context,
	c Cacher,
	sf *singleflight.Group,
	key string,
	ttl time.Duration,
	logger *zap.Logger,
	fn FetchFunc[T],
) (T, error) {
	var zero T
	if logger == nil {
		logger = zap.NewNop()
	}

	var cached T
	err := c.Get(ctx, key, &cached)
	switch {
	case err == nil:
		logger.Debug("cache hit", zap.String("key", key))
		triggerBackgroundRefresh(c, sf, key, ttl, logger, fn)
		return cached, nil

	case errors.Is(err, redis.Nil):
		logger.Debug("cache miss", zap.String("key", key))

	default:
		logger.Warn("cache get error (treating as miss)", zap.String("key", key), zap.Error(err))
	}

	v, err, shared := sf.Do(key, func() (any, error) {
		return fetchAndCacheInBackground(ctx, c, key, ttl, logger, fn)
	})
	if err != nil {
		return zero, err
	}

	value, ok := v.(T)
	if !ok {
		logger.Error("singleflight type mismatch", zap.String("key", key))
		return zero, fmt.Errorf("type mismatch for key %q", key)
	}

	if shared {
		logger.Debug("singleflight shared result", zap.String("key", key))
	}

	return value, nil
}
