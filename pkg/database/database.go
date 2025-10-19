package database

import (
	"database/sql"
	"fmt"
	"time"
)

type Options struct {
	Driver          string
	DataSource      string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Option func(*Options)

func WithDriver(driver string) Option {
	return func(o *Options) { o.Driver = driver }
}

func WithDataSource(dsn string) Option {
	return func(o *Options) { o.DataSource = dsn }
}

func WithMaxOpenConns(count int) Option {
	return func(o *Options) { o.MaxOpenConns = count }
}

// New creates a new database connection pool using the provided options.
func New(opts ...Option) (*sql.DB, error) {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	db, err := sql.Open(options.Driver, options.DataSource)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(options.MaxOpenConns)
	db.SetMaxIdleConns(options.MaxIdleConns)
	db.SetConnMaxLifetime(options.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
