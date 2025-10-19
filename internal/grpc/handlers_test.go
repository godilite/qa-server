package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/godilite/qa-server/api/v1"
	"github.com/godilite/qa-server/internal/grpc/mocks"
	"github.com/godilite/qa-server/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestNewGRPCHandlers tests the constructor
func TestNewGRPCHandlers(t *testing.T) {
	t.Run("valid parameters", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{}
		mockCache := &mocks.MockCacher{}
		logger := zap.NewNop()
		ttl := 5 * time.Minute

		handlers := NewGRPCHandlers(mockScoring, mockCache, logger, ttl)

		assert.NotNil(t, handlers)
		assert.Equal(t, mockScoring, handlers.scoring)
		assert.Equal(t, mockCache, handlers.cache)
		assert.Equal(t, ttl, handlers.cacheTTL)
		assert.NotNil(t, handlers.logger)
	})

	t.Run("nil scoring service panics", func(t *testing.T) {
		mockCache := &mocks.MockCacher{}
		logger := zap.NewNop()

		assert.Panics(t, func() {
			NewGRPCHandlers(nil, mockCache, logger, time.Minute)
		})
	})

	t.Run("zero TTL uses default", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{}
		mockCache := &mocks.MockCacher{}
		logger := zap.NewNop()

		handlers := NewGRPCHandlers(mockScoring, mockCache, logger, 0)

		assert.Equal(t, defaultCacheDuration, handlers.cacheTTL)
	})

	t.Run("negative TTL uses default", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{}
		mockCache := &mocks.MockCacher{}
		logger := zap.NewNop()

		handlers := NewGRPCHandlers(mockScoring, mockCache, logger, -time.Minute)

		assert.Equal(t, defaultCacheDuration, handlers.cacheTTL)
	})
}

// TestRequestValidation tests request validation through the actual handler methods
func TestRequestValidation(t *testing.T) {
	mockScoring := &mocks.MockScoringService{
		GetOverallScoreFunc: func(ctx context.Context, start, end time.Time) (float64, error) {
			return 85.5, nil
		},
	}
	mockCache := &mocks.MockCacher{}
	handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

	t.Run("valid request", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetOverallQualityScore(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 85.5, resp.Score)
	})

	t.Run("end before start", func(t *testing.T) {
		start := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetOverallQualityScore(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "end date must be after start date")
	})

	t.Run("same start and end dates are allowed", func(t *testing.T) {
		date := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(date),
			EndDate:   timestamppb.New(date),
		}

		resp, err := handlers.GetOverallQualityScore(context.Background(), req)

		// Based on the test results, same dates actually work
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 85.5, resp.Score)
	})
}

// TestNormalizeKey tests cache key generation
func TestNormalizeKey(t *testing.T) {
	t.Run("basic key generation", func(t *testing.T) {
		start := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
		end := time.Date(2025, 1, 20, 8, 45, 12, 0, time.UTC)

		key := normalizeKey(cacheKeyOverallScore, start, end)

		expected := "grpc:overall_quality_score:2025-01-15:2025-01-20"
		assert.Equal(t, expected, key)
	})

	t.Run("time truncation to day boundaries", func(t *testing.T) {
		// Times with hours/minutes/seconds should be truncated to day boundaries
		start := time.Date(2025, 2, 1, 23, 59, 59, 999999999, time.UTC)
		end := time.Date(2025, 2, 28, 0, 0, 1, 1, time.UTC)

		key := normalizeKey(cacheKeyTicketScores, start, end)

		expected := "grpc:scores_by_ticket:2025-02-01:2025-02-28"
		assert.Equal(t, expected, key)
	})

	t.Run("different prefixes", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

		tests := []struct {
			prefix   CacheKeyType
			expected string
		}{
			{cacheKeyOverallScore, "grpc:overall_quality_score:2025-01-01:2025-01-31"},
			{cacheKeyTicketScores, "grpc:scores_by_ticket:2025-01-01:2025-01-31"},
			{cacheKeyPeriodChange, "grpc:period_over_period_score_change:2025-01-01:2025-01-31"},
			{cacheKeyAggregatedCategory, "grpc:aggregated_category_scores:2025-01-01:2025-01-31"},
		}

		for _, tt := range tests {
			key := normalizeKey(tt.prefix, start, end)
			assert.Equal(t, tt.expected, key)
		}
	})

	t.Run("timezone conversion", func(t *testing.T) {
		// Test that times in different timezones are normalized to UTC
		loc, _ := time.LoadLocation("America/New_York")
		start := time.Date(2025, 1, 1, 5, 0, 0, 0, loc) // 5 AM EST = 10 AM UTC
		end := time.Date(2025, 1, 1, 20, 0, 0, 0, loc)  // 8 PM EST = 1 AM UTC next day

		key := normalizeKey(cacheKeyOverallScore, start, end)

		expected := "grpc:overall_quality_score:2025-01-01:2025-01-02"
		assert.Equal(t, expected, key)
	})
}

// TestHandleError tests error handling and status code mapping
func TestHandleError(t *testing.T) {
	handlers := &GRPCHandlers{logger: zap.NewNop()}

	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := handlers.handleError(ctx, "test_operation", errors.New("some error"))

		assert.Error(t, err)
		assert.Equal(t, codes.Canceled, status.Code(err))
		assert.Contains(t, err.Error(), "request canceled")
	})

	t.Run("context deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // Ensure timeout

		err := handlers.handleError(ctx, "test_operation", errors.New("some error"))

		assert.Error(t, err)
		assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
		assert.Contains(t, err.Error(), "request timed out")
	})

	t.Run("no ratings error", func(t *testing.T) {
		ctx := context.Background()

		err := handlers.handleError(ctx, "test_operation", service.ErrNoRatings)

		assert.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
		assert.Contains(t, err.Error(), "no ratings found for the given period")
	})

	t.Run("storage failure error", func(t *testing.T) {
		ctx := context.Background()

		err := handlers.handleError(ctx, "test_operation", service.ErrStorageFailure)

		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Contains(t, err.Error(), "database error")
	})

	t.Run("wrapped no ratings error", func(t *testing.T) {
		ctx := context.Background()
		wrappedErr := errors.New("wrapped: " + service.ErrNoRatings.Error())

		err := handlers.handleError(ctx, "test_operation", wrappedErr)

		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err)) // Should be treated as unknown error
		assert.Contains(t, err.Error(), "test_operation failed")
	})

	t.Run("unknown error", func(t *testing.T) {
		ctx := context.Background()
		unknownErr := errors.New("database connection lost")

		err := handlers.handleError(ctx, "test_operation", unknownErr)

		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Contains(t, err.Error(), "test_operation failed")
		assert.Contains(t, err.Error(), "database connection lost")
	})
}

// TestMapToProtoCategoryScores tests data transformation
func TestMapToProtoCategoryScores(t *testing.T) {
	mockScoring := &mocks.MockScoringService{}
	mockCache := &mocks.MockCacher{}
	handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

	t.Run("empty input", func(t *testing.T) {
		input := []service.AggregatedCategoryScores{}

		result := handlers.mapToProtoCategoryScores(input)

		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("single category with no periods", func(t *testing.T) {
		input := []service.AggregatedCategoryScores{
			{
				CategoryName:         "Tone",
				TotalRatings:         5,
				OverallCategoryScore: 85.5,
				PeriodScores:         []service.PeriodScore{},
			},
		}

		result := handlers.mapToProtoCategoryScores(input)

		assert.Len(t, result, 1)
		cat := result[0]
		assert.Equal(t, "Tone", cat.CategoryName)
		assert.Equal(t, int64(5), cat.TotalRatings)
		assert.Equal(t, 85.5, cat.OverallCategoryScore)
		assert.Len(t, cat.PeriodScores, 0)
	})

	t.Run("single category with periods", func(t *testing.T) {
		input := []service.AggregatedCategoryScores{
			{
				CategoryName:         "Grammar",
				TotalRatings:         42,
				OverallCategoryScore: 87.5,
				PeriodScores: []service.PeriodScore{
					{Period: "2025-01-01", Score: 85.0},
					{Period: "2025-01-02", Score: 90.0},
				},
			},
		}

		result := handlers.mapToProtoCategoryScores(input)

		assert.Len(t, result, 1)
		cat := result[0]
		assert.Equal(t, "Grammar", cat.CategoryName)
		assert.Equal(t, int64(42), cat.TotalRatings)
		assert.Equal(t, 87.5, cat.OverallCategoryScore)
		assert.Len(t, cat.PeriodScores, 2)

		// Check first period
		assert.Equal(t, "2025-01-01", cat.PeriodScores[0].Period)
		assert.Equal(t, 85.0, cat.PeriodScores[0].Score)

		// Check second period
		assert.Equal(t, "2025-01-02", cat.PeriodScores[1].Period)
		assert.Equal(t, 90.0, cat.PeriodScores[1].Score)
	})

	t.Run("multiple categories", func(t *testing.T) {
		input := []service.AggregatedCategoryScores{
			{
				CategoryName:         "Tone",
				TotalRatings:         10,
				OverallCategoryScore: 80.0,
				PeriodScores: []service.PeriodScore{
					{Period: "2025-01-01", Score: 75.0},
				},
			},
			{
				CategoryName:         "GDPR",
				TotalRatings:         8,
				OverallCategoryScore: 95.0,
				PeriodScores: []service.PeriodScore{
					{Period: "2025-01-01", Score: 95.0},
					{Period: "2025-01-02", Score: 95.0},
				},
			},
		}

		result := handlers.mapToProtoCategoryScores(input)

		assert.Len(t, result, 2)

		// First category
		assert.Equal(t, "Tone", result[0].CategoryName)
		assert.Equal(t, int64(10), result[0].TotalRatings)
		assert.Equal(t, 80.0, result[0].OverallCategoryScore)
		assert.Len(t, result[0].PeriodScores, 1)

		// Second category
		assert.Equal(t, "GDPR", result[1].CategoryName)
		assert.Equal(t, int64(8), result[1].TotalRatings)
		assert.Equal(t, 95.0, result[1].OverallCategoryScore)
		assert.Len(t, result[1].PeriodScores, 2)
	})

	t.Run("edge case values", func(t *testing.T) {
		input := []service.AggregatedCategoryScores{
			{
				CategoryName:         "",
				TotalRatings:         0,
				OverallCategoryScore: 0.0,
				PeriodScores: []service.PeriodScore{
					{Period: "", Score: 0.0},
				},
			},
		}

		result := handlers.mapToProtoCategoryScores(input)

		assert.Len(t, result, 1)
		cat := result[0]
		assert.Equal(t, "", cat.CategoryName)
		assert.Equal(t, int64(0), cat.TotalRatings)
		assert.Equal(t, 0.0, cat.OverallCategoryScore)
		assert.Len(t, cat.PeriodScores, 1)
		assert.Equal(t, "", cat.PeriodScores[0].Period)
		assert.Equal(t, 0.0, cat.PeriodScores[0].Score)
	})
}

// TestGetOverallQualityScore tests the main handler method
func TestGetOverallQualityScore(t *testing.T) {
	t.Run("service error handling", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetOverallScoreFunc: func(ctx context.Context, start, end time.Time) (float64, error) {
				return 0, service.ErrNoRatings
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetOverallQualityScore(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.NotFound, status.Code(err))
		assert.Contains(t, err.Error(), "no ratings found")
	})
}

// TestGetScoresByTicket tests validation for GetScoresByTicket
func TestGetScoresByTicket(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetScoresByTicketFunc: func(ctx context.Context, start, end time.Time) ([]service.TicketScores, error) {
				return []service.TicketScores{
					{
						TicketID: 123,
						CategoryScores: map[string]float64{
							"Tone": 85.0,
						},
					},
				}, nil
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetScoresByTicket(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.TicketScores, 1)
		assert.Equal(t, int64(123), resp.TicketScores[0].TicketId)
	})
}

// TestErrorHandling tests error propagation from service layer
func TestErrorHandling_ServiceErrors(t *testing.T) {
	t.Run("service returns ErrNoRatings", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetOverallScoreFunc: func(ctx context.Context, start, end time.Time) (float64, error) {
				return 0, service.ErrNoRatings
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetOverallQualityScore(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.NotFound, status.Code(err))
		assert.Contains(t, err.Error(), "no ratings found")
	})

	t.Run("service returns ErrStorageFailure", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetPeriodOverPeriodScoreChangeFunc: func(ctx context.Context, start, end time.Time) (service.PeriodChange, error) {
				return service.PeriodChange{}, service.ErrStorageFailure
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetPeriodOverPeriodScoreChange(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Contains(t, err.Error(), "database error")
	})
}

// TestSuccessfulCalls tests successful handler calls with function-based mocks
func TestSuccessfulCalls(t *testing.T) {
	t.Run("GetOverallQualityScore success", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetOverallScoreFunc: func(ctx context.Context, start, end time.Time) (float64, error) {
				return 92.5, nil
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetOverallQualityScore(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 92.5, resp.Score)
	})

	t.Run("GetScoresByTicket success", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetScoresByTicketFunc: func(ctx context.Context, start, end time.Time) ([]service.TicketScores, error) {
				return []service.TicketScores{
					{
						TicketID: 123,
						CategoryScores: map[string]float64{
							"Tone":    85.0,
							"Grammar": 90.0,
						},
					},
					{
						TicketID: 456,
						CategoryScores: map[string]float64{
							"Tone": 75.0,
						},
					},
				}, nil
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetScoresByTicket(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.TicketScores, 2)
		assert.Equal(t, int64(123), resp.TicketScores[0].TicketId)
		assert.Equal(t, int64(456), resp.TicketScores[1].TicketId)
	})

	t.Run("GetPeriodOverPeriodScoreChange success", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetPeriodOverPeriodScoreChangeFunc: func(ctx context.Context, start, end time.Time) (service.PeriodChange, error) {
				return service.PeriodChange{
					CurrentPeriodScore:  90.0,
					PreviousPeriodScore: 85.0,
					ChangePercentage:    5.88,
				}, nil
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetPeriodOverPeriodScoreChange(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 90.0, resp.CurrentPeriodScore)
		assert.Equal(t, 85.0, resp.PreviousPeriodScore)
		assert.InDelta(t, 5.88, resp.ChangePercentage, 0.01)
	})

	t.Run("GetAggregatedCategoryScores success", func(t *testing.T) {
		mockScoring := &mocks.MockScoringService{
			GetAggregatedCategoryScoresFunc: func(ctx context.Context, start, end time.Time) ([]service.AggregatedCategoryScores, error) {
				return []service.AggregatedCategoryScores{
					{
						CategoryName:         "Tone",
						TotalRatings:         15,
						OverallCategoryScore: 88.0,
						PeriodScores: []service.PeriodScore{
							{Period: "2025-01-01", Score: 85.0},
							{Period: "2025-01-02", Score: 91.0},
						},
					},
				}, nil
			},
		}
		mockCache := &mocks.MockCacher{}
		handlers := NewGRPCHandlers(mockScoring, mockCache, zap.NewNop(), time.Minute)

		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handlers.GetAggregatedCategoryScores(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.CategoryScores, 1)

		cat := resp.CategoryScores[0]
		assert.Equal(t, "Tone", cat.CategoryName)
		assert.Equal(t, int64(15), cat.TotalRatings)
		assert.Equal(t, 88.0, cat.OverallCategoryScore)
		assert.Len(t, cat.PeriodScores, 2)
	})
}
