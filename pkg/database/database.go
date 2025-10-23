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
	ConnMaxIdleTime time.Duration
	RetryAttempts   int
	RetryDelay      time.Duration
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

func WithMaxIdleConns(count int) Option {
	return func(o *Options) { o.MaxIdleConns = count }
}

func WithConnMaxLifetime(duration time.Duration) Option {
	return func(o *Options) { o.ConnMaxLifetime = duration }
}

func WithConnMaxIdleTime(duration time.Duration) Option {
	return func(o *Options) { o.ConnMaxIdleTime = duration }
}

func WithRetry(attempts int, delay time.Duration) Option {
	return func(o *Options) {
		o.RetryAttempts = attempts
		o.RetryDelay = delay
	}
}

// New creates a new database connection pool using the provided options.
func New(opts ...Option) (*sql.DB, error) {
	options := &Options{
		Driver:          "sqlite3",
		DataSource:      ":memory:",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
		RetryAttempts:   3,
		RetryDelay:      time.Second,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Validate options
	if options.Driver == "" {
		return nil, fmt.Errorf("database driver cannot be empty")
	}
	if options.DataSource == "" {
		return nil, fmt.Errorf("database data source cannot be empty")
	}

	var db *sql.DB
	var err error

	// Retry connection with exponential backoff
	for i := 0; i < options.RetryAttempts; i++ {
		db, err = sql.Open(options.Driver, options.DataSource)
		if err == nil {
			// Configure connection pool
			db.SetMaxOpenConns(options.MaxOpenConns)
			db.SetMaxIdleConns(options.MaxIdleConns)
			db.SetConnMaxLifetime(options.ConnMaxLifetime)
			db.SetConnMaxIdleTime(options.ConnMaxIdleTime)

			if err = db.Ping(); err == nil {
				return db, nil
			}

			// Close failed connection
			db.Close()
		}

		if i < options.RetryAttempts-1 {
			waitTime := time.Duration(i+1) * options.RetryDelay
			time.Sleep(waitTime)
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", options.RetryAttempts, err)
}
