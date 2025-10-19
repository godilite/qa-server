package mocks

import (
	"context"
	"errors"
	"time"

	"github.com/godilite/qa-server/internal/service"
)

// MockScoringService is a mock implementation of the ScoringService interface
// for testing the handler layer. It uses function-based mocking for flexibility.
type MockScoringService struct {
	GetOverallScoreFunc                func(ctx context.Context, start, end time.Time) (float64, error)
	GetScoresByTicketFunc              func(ctx context.Context, start, end time.Time) ([]service.TicketScores, error)
	GetPeriodOverPeriodScoreChangeFunc func(ctx context.Context, start, end time.Time) (service.PeriodChange, error)
	GetAggregatedCategoryScoresFunc    func(ctx context.Context, start, end time.Time) ([]service.AggregatedCategoryScores, error)
}

// GetOverallScore implements the ScoringService interface
func (m *MockScoringService) GetOverallScore(ctx context.Context, start, end time.Time) (float64, error) {
	if m.GetOverallScoreFunc != nil {
		return m.GetOverallScoreFunc(ctx, start, end)
	}
	return 0, errors.New("GetOverallScoreFunc not implemented")
}

// GetScoresByTicket implements the ScoringService interface
func (m *MockScoringService) GetScoresByTicket(ctx context.Context, start, end time.Time) ([]service.TicketScores, error) {
	if m.GetScoresByTicketFunc != nil {
		return m.GetScoresByTicketFunc(ctx, start, end)
	}
	return nil, errors.New("GetScoresByTicketFunc not implemented")
}

// GetPeriodOverPeriodScoreChange implements the ScoringService interface
func (m *MockScoringService) GetPeriodOverPeriodScoreChange(ctx context.Context, start, end time.Time) (service.PeriodChange, error) {
	if m.GetPeriodOverPeriodScoreChangeFunc != nil {
		return m.GetPeriodOverPeriodScoreChangeFunc(ctx, start, end)
	}
	return service.PeriodChange{}, errors.New("GetPeriodOverPeriodScoreChangeFunc not implemented")
}

// GetAggregatedCategoryScores implements the ScoringService interface
func (m *MockScoringService) GetAggregatedCategoryScores(ctx context.Context, start, end time.Time) ([]service.AggregatedCategoryScores, error) {
	if m.GetAggregatedCategoryScoresFunc != nil {
		return m.GetAggregatedCategoryScoresFunc(ctx, start, end)
	}
	return nil, errors.New("GetAggregatedCategoryScoresFunc not implemented")
}
