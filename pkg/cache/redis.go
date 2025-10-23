package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

type Options struct {
	Address  string
	Password string
	DB       int
}

type Option func(*Options)

func WithAddress(addr string) Option {
	return func(o *Options) {
		o.Address = addr
	}
}

func WithPassword(pass string) Option {
	return func(o *Options) {
		o.Password = pass
	}
}

func WithDB(db int) Option {
	return func(o *Options) {
		o.DB = db
	}
}

func New(ctx context.Context, opts ...Option) (*Cache, error) {
	options := &Options{
		Address:  "localhost:6379",
		Password: "",
		DB:       0,
	}

	for _, opt := range opts {
		opt(options)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     options.Address,
		Password: options.Password,
		DB:       options.DB,
	})

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return &Cache{client: client}, nil
}

func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *Cache) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *Cache) Close() error {
	return c.client.Close()
}

type FetchFunc[T any] func(ctx context.Context) (T, error)

func FindAndCache[T any](ctx context.Context, cache *Cache, key string, cacheDuration time.Duration, fn FetchFunc[T]) FetchFunc[T] {
	return func(ctx context.Context) (T, error) {
		var zero T

		var cached T
		if err := cache.Get(ctx, key, &cached); err == nil {
			go func() {
				result, err := fn(context.Background())
				if err != nil {
					log.Printf("background fetch failed: %v", err)
					return
				}
				if err := cache.Set(context.Background(), key, result, cacheDuration); err != nil {
					log.Printf("background cache set failed: %v", err)
				}
			}()

			return cached, nil
		}

		result, err := fn(ctx)
		if err != nil {
			return zero, err
		}

		if err := cache.Set(ctx, key, result, cacheDuration); err != nil {
			return zero, err
		}

		return result, nil
	}
}
