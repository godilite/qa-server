package mocks

import (
	"context"
	"errors"
	"time"

	"github.com/godilite/qa-server/internal/repository/models"
)

// MockRatingScoreRepository is a mock implementation of the RatingScoreRepository interface
// for testing the service layer.
type MockRatingScoreRepository struct {
	GetOverallRatingsFunc  func(ctx context.Context, start, end time.Time) (float64, int64, error)
	GetRatingsInPeriodFunc func(ctx context.Context, start, end time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error)
	GetScoresByTicketFunc  func(ctx context.Context, start, end time.Time) ([]models.TicketCategoryScore, error)
}

// GetOverallRatings implements the RatingScoreRepository interface
func (m *MockRatingScoreRepository) GetOverallRatings(ctx context.Context, start, end time.Time) (float64, int64, error) {
	if m.GetOverallRatingsFunc != nil {
		return m.GetOverallRatingsFunc(ctx, start, end)
	}
	return 0, 0, errors.New("GetOverallRatingsFunc not implemented")
}

// GetRatingsInPeriod implements the RatingScoreRepository interface
func (m *MockRatingScoreRepository) GetRatingsInPeriod(ctx context.Context, start, end time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error) {
	if m.GetRatingsInPeriodFunc != nil {
		return m.GetRatingsInPeriodFunc(ctx, start, end, isWeekly)
	}
	return nil, errors.New("GetRatingsInPeriodFunc not implemented")
}

// GetScoresByTicket implements the RatingScoreRepository interface
func (m *MockRatingScoreRepository) GetScoresByTicket(ctx context.Context, start, end time.Time) ([]models.TicketCategoryScore, error) {
	if m.GetScoresByTicketFunc != nil {
		return m.GetScoresByTicketFunc(ctx, start, end)
	}
	return nil, errors.New("GetScoresByTicketFunc not implemented")
}
