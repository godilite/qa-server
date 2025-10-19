package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godilite/qa-server/internal/repository/models"
	"github.com/godilite/qa-server/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestNewScoringService tests the constructor
func TestNewScoringService(t *testing.T) {
	t.Run("valid parameters", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{}
		logger := zap.NewNop()

		service := NewScoringService(mockRepo, logger)

		assert.NotNil(t, service)
		assert.Equal(t, mockRepo, service.storage)
		assert.Equal(t, logger, service.logger)
	})

	t.Run("nil storage panics", func(t *testing.T) {
		logger := zap.NewNop()

		assert.Panics(t, func() {
			NewScoringService(nil, logger)
		})
	})

	t.Run("nil logger gets default", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{}

		service := NewScoringService(mockRepo, nil)

		assert.NotNil(t, service)
		assert.NotNil(t, service.logger)
	})
}

// TestGetOverallScore tests the GetOverallScore method
func TestGetOverallScore(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("successful calculation", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				assert.Equal(t, start, s)
				assert.Equal(t, end, e)
				return 85.5, 100, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		score, err := service.GetOverallScore(ctx, start, end)

		assert.NoError(t, err)
		assert.Equal(t, 85.5, score)
	})

	t.Run("no ratings found", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				return 0, 0, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		score, err := service.GetOverallScore(ctx, start, end)

		assert.ErrorIs(t, err, ErrNoRatings)
		assert.Equal(t, 0.0, score)
	})

	t.Run("storage failure", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				return 0, 0, errors.New("database connection failed")
			},
		}

		service := NewScoringService(mockRepo, logger)
		score, err := service.GetOverallScore(ctx, start, end)

		assert.ErrorIs(t, err, ErrStorageFailure)
		assert.Contains(t, err.Error(), "database connection failed")
		assert.Equal(t, 0.0, score)
	})
}

// TestGetAggregatedCategoryScores tests category score aggregation
func TestGetAggregatedCategoryScores(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	t.Run("successful daily aggregation", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetRatingsInPeriodFunc: func(ctx context.Context, s, e time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error) {
				assert.Equal(t, start, s)
				assert.Equal(t, end, e)
				assert.False(t, isWeekly)

				return []models.AggregatedCategoryData{
					{Category: "Tone", Period: "2025-01-01", TotalWeightedEvaluation: 4.0, TotalWeight: 1.0, EvaluationCount: 1},
					{Category: "Tone", Period: "2025-01-02", TotalWeightedEvaluation: 3.0, TotalWeight: 1.0, EvaluationCount: 1},
					{Category: "Grammar", Period: "2025-01-01", TotalWeightedEvaluation: 5.0, TotalWeight: 1.0, EvaluationCount: 1},
				}, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetAggregatedCategoryScores(ctx, start, end)

		assert.NoError(t, err)
		assert.Len(t, results, 2)

		var toneResult *AggregatedCategoryScores
		for i := range results {
			if results[i].CategoryName == "Tone" {
				toneResult = &results[i]
				break
			}
		}
		assert.NotNil(t, toneResult)
		assert.Equal(t, "Tone", toneResult.CategoryName)
		assert.Equal(t, 2, toneResult.TotalRatings)
		assert.Len(t, toneResult.PeriodScores, 2)
		assert.Equal(t, 70.0, toneResult.OverallCategoryScore)
	})

	t.Run("weekly aggregation for long period", func(t *testing.T) {
		longEnd := start.AddDate(0, 2, 0)

		mockRepo := &mocks.MockRatingScoreRepository{
			GetRatingsInPeriodFunc: func(ctx context.Context, s, e time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error) {
				assert.True(t, isWeekly)
				return []models.AggregatedCategoryData{
					{Category: "Tone", Period: "2025-W01", TotalWeightedEvaluation: 10.0, TotalWeight: 2.0, EvaluationCount: 2},
				}, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetAggregatedCategoryScores(ctx, start, longEnd)

		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Tone", results[0].CategoryName)
		assert.Equal(t, 100.0, results[0].OverallCategoryScore)
	})

	t.Run("no ratings found", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetRatingsInPeriodFunc: func(ctx context.Context, s, e time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error) {
				return []models.AggregatedCategoryData{}, nil // Empty result
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetAggregatedCategoryScores(ctx, start, end)

		assert.ErrorIs(t, err, ErrNoRatings)
		assert.Nil(t, results)
	})

	t.Run("storage failure", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetRatingsInPeriodFunc: func(ctx context.Context, s, e time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error) {
				return nil, errors.New("query timeout")
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetAggregatedCategoryScores(ctx, start, end)

		assert.ErrorIs(t, err, ErrStorageFailure)
		assert.Contains(t, err.Error(), "query timeout")
		assert.Nil(t, results)
	})
}

// TestGetScoresByTicket tests ticket score pivoting
func TestGetScoresByTicket(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("successful pivot", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetScoresByTicketFunc: func(ctx context.Context, s, e time.Time) ([]models.TicketCategoryScore, error) {
				assert.Equal(t, start, s)
				assert.Equal(t, end, e)

				return []models.TicketCategoryScore{
					{TicketID: 101, Category: "Tone", Score: 85.0},
					{TicketID: 101, Category: "Grammar", Score: 92.0},
					{TicketID: 102, Category: "Tone", Score: 78.0},
					{TicketID: 102, Category: "GDPR", Score: 95.0},
				}, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetScoresByTicket(ctx, start, end)

		assert.NoError(t, err)
		assert.Len(t, results, 2) // Two tickets: 101, 102

		// Verify ticket 101 has correct categories
		var ticket101 *TicketScores
		for i := range results {
			if results[i].TicketID == 101 {
				ticket101 = &results[i]
				break
			}
		}
		assert.NotNil(t, ticket101)
		assert.Equal(t, int64(101), ticket101.TicketID)
		assert.Len(t, ticket101.CategoryScores, 2)
		assert.Equal(t, 85.0, ticket101.CategoryScores["Tone"])
		assert.Equal(t, 92.0, ticket101.CategoryScores["Grammar"])
	})

	t.Run("no tickets found", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetScoresByTicketFunc: func(ctx context.Context, s, e time.Time) ([]models.TicketCategoryScore, error) {
				return []models.TicketCategoryScore{}, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetScoresByTicket(ctx, start, end)

		assert.ErrorIs(t, err, ErrNoRatings)
		assert.Nil(t, results)
	})

	t.Run("storage failure", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetScoresByTicketFunc: func(ctx context.Context, s, e time.Time) ([]models.TicketCategoryScore, error) {
				return nil, errors.New("connection lost")
			},
		}

		service := NewScoringService(mockRepo, logger)
		results, err := service.GetScoresByTicket(ctx, start, end)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fetch scores by ticket")
		assert.Contains(t, err.Error(), "connection lost")
		assert.Nil(t, results)
	})
}

// TestGetPeriodOverPeriodScoreChange tests period comparison logic
func TestGetPeriodOverPeriodScoreChange(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)

	// Calculate expected previous period
	duration := end.Sub(start)
	expectedPrevEnd := start.Add(-time.Nanosecond)
	expectedPrevStart := expectedPrevEnd.Add(-duration + time.Nanosecond)

	t.Run("positive change", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				// Current period
				if s.Equal(start) && e.Equal(end) {
					return 90.0, 100, nil
				}
				// Previous period
				if s.Equal(expectedPrevStart) && e.Equal(expectedPrevEnd) {
					return 80.0, 90, nil
				}
				return 0, 0, errors.New("unexpected time range")
			},
		}

		service := NewScoringService(mockRepo, logger)
		result, err := service.GetPeriodOverPeriodScoreChange(ctx, start, end)

		assert.NoError(t, err)
		assert.Equal(t, 90.0, result.CurrentPeriodScore)
		assert.Equal(t, 80.0, result.PreviousPeriodScore)
		assert.InDelta(t, 12.5, result.ChangePercentage, 0.01) // ((90-80)/80)*100 = 12.5%
	})

	t.Run("negative change", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				if s.Equal(start) && e.Equal(end) {
					return 70.0, 100, nil
				}
				if s.Equal(expectedPrevStart) && e.Equal(expectedPrevEnd) {
					return 80.0, 90, nil
				}
				return 0, 0, errors.New("unexpected time range")
			},
		}

		service := NewScoringService(mockRepo, logger)
		result, err := service.GetPeriodOverPeriodScoreChange(ctx, start, end)

		assert.NoError(t, err)
		assert.Equal(t, 70.0, result.CurrentPeriodScore)
		assert.Equal(t, 80.0, result.PreviousPeriodScore)
		assert.InDelta(t, -12.5, result.ChangePercentage, 0.01)
	})

	t.Run("no previous ratings", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				if s.Equal(start) && e.Equal(end) {
					return 90.0, 100, nil
				}
				if s.Equal(expectedPrevStart) && e.Equal(expectedPrevEnd) {
					return 0, 0, nil
				}
				return 0, 0, errors.New("unexpected time range")
			},
		}

		service := NewScoringService(mockRepo, logger)
		result, err := service.GetPeriodOverPeriodScoreChange(ctx, start, end)

		assert.NoError(t, err)
		assert.Equal(t, 90.0, result.CurrentPeriodScore)
		assert.Equal(t, 0.0, result.PreviousPeriodScore)
		assert.Equal(t, 100.0, result.ChangePercentage)
	})

	t.Run("current period failure", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				if s.Equal(start) && e.Equal(end) {
					return 0, 0, errors.New("db connection failed")
				}
				return 0, 0, nil
			},
		}

		service := NewScoringService(mockRepo, logger)
		result, err := service.GetPeriodOverPeriodScoreChange(ctx, start, end)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current score")
		assert.Contains(t, err.Error(), "db connection failed")
		assert.Equal(t, PeriodChange{}, result)
	})

	t.Run("previous period storage failure", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				if s.Equal(start) && e.Equal(end) {
					return 90.0, 100, nil
				}
				if s.Equal(expectedPrevStart) && e.Equal(expectedPrevEnd) {
					return 0, 0, errors.New("db timeout")
				}
				return 0, 0, errors.New("unexpected time range")
			},
		}

		service := NewScoringService(mockRepo, logger)
		result, err := service.GetPeriodOverPeriodScoreChange(ctx, start, end)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "previous score")
		assert.Contains(t, err.Error(), "db timeout")
		assert.Equal(t, PeriodChange{}, result)
	})

	t.Run("zero previous score with positive current", func(t *testing.T) {
		mockRepo := &mocks.MockRatingScoreRepository{
			GetOverallRatingsFunc: func(ctx context.Context, s, e time.Time) (float64, int64, error) {
				if s.Equal(start) && e.Equal(end) {
					return 50.0, 100, nil
				}
				if s.Equal(expectedPrevStart) && e.Equal(expectedPrevEnd) {
					return 0.0, 0, nil
				}
				return 0, 0, errors.New("unexpected time range")
			},
		}

		service := NewScoringService(mockRepo, logger)
		result, err := service.GetPeriodOverPeriodScoreChange(ctx, start, end)

		assert.NoError(t, err)
		assert.Equal(t, 50.0, result.CurrentPeriodScore)
		assert.Equal(t, 0.0, result.PreviousPeriodScore)
		assert.Equal(t, 100.0, result.ChangePercentage)
	})
}

// Test utility functions
func TestIsAtLeastOneMonth(t *testing.T) {
	t.Run("exactly one month", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

		result := isAtLeastOneMonth(start, end)
		assert.True(t, result)
	})

	t.Run("less than one month", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 25, 0, 0, 0, 0, time.UTC)

		result := isAtLeastOneMonth(start, end)
		assert.False(t, result)
	})

	t.Run("more than one month", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

		result := isAtLeastOneMonth(start, end)
		assert.True(t, result)
	})
}

func TestIsWeeklyAggregation(t *testing.T) {
	t.Run("short period - daily", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

		result := isWeeklyAggregation(start, end)
		assert.False(t, result)
	})

	t.Run("long period - weekly", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

		result := isWeeklyAggregation(start, end)
		assert.True(t, result)
	})

	t.Run("exactly 28 days - weekly", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(28 * 24 * time.Hour)

		result := isWeeklyAggregation(start, end)
		assert.True(t, result)
	})
}
