package service

import (
	"context"
	"time"

	"github.com/godilite/qa-server/internal/repository/models"
)

// RatingScoreRepository defines the interface for database operations for service.
type RatingScoreRepository interface {
	GetOverallRatings(ctx context.Context, start, end time.Time) (float64, int64, error)
	GetRatingsInPeriod(ctx context.Context, start, end time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error)
	GetScoresByTicket(ctx context.Context, start, end time.Time) ([]models.TicketCategoryScore, error)
}
