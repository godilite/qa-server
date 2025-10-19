package grpc

import (
	"context"
	"time"

	"github.com/godilite/qa-server/internal/service"
)

// Cacher defines the interface for cache operations.
type Cacher interface {
	Close() error
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
}

type ScoringService interface {
	GetOverallScore(ctx context.Context, start, end time.Time) (float64, error)
	GetScoresByTicket(ctx context.Context, start, end time.Time) ([]service.TicketScores, error)
	GetPeriodOverPeriodScoreChange(ctx context.Context, start, end time.Time) (service.PeriodChange, error)
	GetAggregatedCategoryScores(ctx context.Context, start, end time.Time) ([]service.AggregatedCategoryScores, error)
}
