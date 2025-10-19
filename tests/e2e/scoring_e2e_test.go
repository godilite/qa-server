//go:build e2e

package e2e

import (
	"context"
	"database/sql"
	"testing"
	"time"

	pb "github.com/godilite/qa-server/api/v1"
	"github.com/godilite/qa-server/internal/grpc"
	"github.com/godilite/qa-server/internal/repository"
	"github.com/godilite/qa-server/internal/service"
	"github.com/godilite/qa-server/tests/e2e/mocks"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Test constants for consistent date handling
var (
	testBaseDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	testEndDate  = time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	schema := `
	CREATE TABLE rating_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		weight REAL NOT NULL
	);
	CREATE TABLE ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ticket_id INTEGER,
		rating INTEGER,
		rating_category_id INTEGER,
		created_at TEXT
	);`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Seed data
	_, err = db.Exec(`
	INSERT INTO rating_categories (name, weight) VALUES
	('Tone', 1.0), ('Grammar', 2.0), ('GDPR', 1.5);

	INSERT INTO ratings (ticket_id, rating, rating_category_id, created_at) VALUES
	-- Current period ratings (2025-01-01)
	(101, 4, 1, '2025-01-01T12:00:00Z'),
	(101, 5, 2, '2025-01-01T12:00:00Z'),
	(101, 3, 3, '2025-01-01T12:00:00Z'),
	(102, 3, 1, '2025-01-01T13:00:00Z'),
	(102, 4, 2, '2025-01-01T13:00:00Z'),
	(103, 5, 1, '2025-01-01T14:00:00Z'),
	
	-- Previous period ratings (2024-12-01) for period-over-period testing
	(201, 3, 1, '2024-12-01T12:00:00Z'),
	(201, 3, 2, '2024-12-01T12:00:00Z'),
	(202, 2, 1, '2024-12-01T13:00:00Z');
	`)
	require.NoError(t, err)

	return db
}

func TestE2E_GetOverallQualityScore(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	resp, err := handler.GetOverallQualityScore(ctx, req)
	require.NoError(t, err, "Handler should not return error")
	assert.Greater(t, resp.Score, 0.0, "Score should be positive")

	// With our test data, we should get a reasonable score
	// The calculation involves weighted averages across categories
	assert.LessOrEqual(t, resp.Score, 100.0, "Score should not exceed 100")
	assert.GreaterOrEqual(t, resp.Score, 50.0, "Score should be reasonable for test data")
}

func TestE2E_GetAggregatedCategoryScores(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	resp, err := handler.GetAggregatedCategoryScores(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.CategoryScores)
	names := []string{}
	for _, c := range resp.CategoryScores {
		names = append(names, c.CategoryName)
		// Each category should have a positive score and rating count
		require.Greater(t, c.OverallCategoryScore, 0.0)
		require.Greater(t, c.TotalRatings, int64(0))
	}
	require.ElementsMatch(t, []string{"Tone", "Grammar", "GDPR"}, names)
}

func TestE2E_GetScoresByTicket(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	resp, err := handler.GetScoresByTicket(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.TicketScores, 3) // We now have tickets 101, 102, 103

	// Verify ticket IDs and that each has category scores
	ticketIDs := make([]int64, 0, len(resp.TicketScores))
	for _, ts := range resp.TicketScores {
		ticketIDs = append(ticketIDs, ts.TicketId)
		require.NotEmpty(t, ts.CategoryScores, "Each ticket should have category scores")
	}
	require.ElementsMatch(t, []int64{101, 102, 103}, ticketIDs)
}

func TestE2E_GetPeriodOverPeriodScoreChange(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()
	// Test current period (2025-01-01) vs previous period calculation
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	resp, err := handler.GetPeriodOverPeriodScoreChange(ctx, req)
	require.NoError(t, err)

	// We should get meaningful current period data
	require.Greater(t, resp.CurrentPeriodScore, 0.0, "Current period should have data")

	// Previous period might be 0 if no data exists for calculated previous period
	// The period-over-period calculation uses the same duration backwards
	require.GreaterOrEqual(t, resp.PreviousPeriodScore, 0.0, "Previous period score should be non-negative")

	// Change percentage should be calculated
	// If previous period is 0, change should reflect this appropriately
	if resp.PreviousPeriodScore > 0 {
		require.NotEqual(t, resp.ChangePercentage, 0.0, "Change percentage should be calculated when both periods have data")
	} else {
		// When previous period is 0, the implementation might return a special value
		t.Logf("Previous period has no data, change percentage: %f", resp.ChangePercentage)
	}
}

func TestE2E_PeriodOverPeriodWithProperData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()

	// Test with a 7-day period where we control the previous period data
	start := testBaseDate
	end := start.Add(7 * 24 * time.Hour) // 7 days

	// Add more data in the previous period (7 days before start)
	previousStart := start.Add(-7 * 24 * time.Hour)
	_, err := db.Exec(`
		INSERT INTO ratings (ticket_id, rating, rating_category_id, created_at) VALUES
		(301, 3, 1, ?),
		(301, 4, 2, ?),
		(302, 2, 1, ?);
	`, previousStart.Format("2006-01-02T15:04:05Z"),
		previousStart.Format("2006-01-02T15:04:05Z"),
		previousStart.Add(time.Hour).Format("2006-01-02T15:04:05Z"))
	require.NoError(t, err)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	resp, err := handler.GetPeriodOverPeriodScoreChange(ctx, req)
	require.NoError(t, err)

	// Now both periods should have data
	require.Greater(t, resp.CurrentPeriodScore, 0.0, "Current period should have data")
	require.Greater(t, resp.PreviousPeriodScore, 0.0, "Previous period should have data")

	// Change percentage should be meaningful
	require.NotEqual(t, resp.ChangePercentage, 0.0, "Change percentage should be calculated")

	t.Logf("Current: %.2f, Previous: %.2f, Change: %.2f%%",
		resp.CurrentPeriodScore, resp.PreviousPeriodScore, resp.ChangePercentage)
}

func TestE2E_CachingBehavior(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	logger := zap.NewNop()
	svc := service.NewScoringService(repo, logger)

	// Create a tracking cache implementation
	trackedCache := mocks.NewTrackingCache()

	handler := grpc.NewGRPCHandlers(svc, trackedCache, logger, 1*time.Minute)

	ctx := context.Background()
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	// First call should miss cache and set it
	resp1, err1 := handler.GetOverallQualityScore(ctx, req)
	require.NoError(t, err1)

	initialGetCalls := trackedCache.GetCalls

	// Second call should hit cache (if caching is working)
	resp2, err2 := handler.GetOverallQualityScore(ctx, req)
	require.NoError(t, err2)

	// Responses should be identical
	require.Equal(t, resp1.Score, resp2.Score)

	// Cache should have been checked again
	require.Greater(t, trackedCache.GetCalls, initialGetCalls, "Cache should be checked on second call")

	t.Logf("Cache stats - Gets: %d, Sets: %d", trackedCache.GetCalls, trackedCache.SetCalls)
}

func TestE2E_PerformanceBaseline(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add more test data for performance testing
	_, err := db.Exec(`
		INSERT INTO ratings (ticket_id, rating, rating_category_id, created_at) VALUES
		(401, 4, 1, '2025-01-01T10:00:00Z'),
		(401, 5, 2, '2025-01-01T10:00:00Z'),
		(402, 3, 1, '2025-01-01T11:00:00Z'),
		(403, 4, 2, '2025-01-01T12:00:00Z'),
		(404, 5, 1, '2025-01-01T13:00:00Z'),
		(405, 3, 3, '2025-01-01T14:00:00Z'),
		(406, 4, 1, '2025-01-01T15:00:00Z'),
		(407, 5, 2, '2025-01-01T16:00:00Z'),
		(408, 2, 3, '2025-01-01T17:00:00Z'),
		(409, 4, 1, '2025-01-01T18:00:00Z');
	`)
	require.NoError(t, err)

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	// Performance test - sequential calls to avoid SQLite concurrency issues
	startTime := time.Now()

	const numCalls = 5 // Reduced for SQLite limitations

	// Test sequential calls instead of concurrent to avoid SQLite concurrency issues
	for i := 0; i < numCalls; i++ {
		_, err := handler.GetOverallQualityScore(ctx, req)
		require.NoError(t, err, "GetOverallQualityScore call %d should succeed", i+1)

		_, err = handler.GetAggregatedCategoryScores(ctx, req)
		require.NoError(t, err, "GetAggregatedCategoryScores call %d should succeed", i+1)

		_, err = handler.GetScoresByTicket(ctx, req)
		require.NoError(t, err, "GetScoresByTicket call %d should succeed", i+1)

		_, err = handler.GetPeriodOverPeriodScoreChange(ctx, req)
		require.NoError(t, err, "GetPeriodOverPeriodScoreChange call %d should succeed", i+1)
	}

	duration := time.Since(startTime)
	t.Logf("Completed %d sequential calls across 4 endpoints in %v", numCalls*4, duration)

	// Performance baseline - should complete in reasonable time
	require.Less(t, duration, 2*time.Second, "Performance should be reasonable for sequential calls")
}

func TestE2E_ErrorScenarios(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()

	t.Run("no data in period", func(t *testing.T) {
		// Request data for a period with no ratings
		start := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(24 * time.Hour)

		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handler.GetOverallQualityScore(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)

		// Should return NotFound error for no ratings
		require.Contains(t, err.Error(), "no ratings found")
	})

	t.Run("invalid date range", func(t *testing.T) {
		// End date before start date
		start := testBaseDate
		end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC) // Day before start date

		req := &pb.TimePeriodRequest{
			StartDate: timestamppb.New(start),
			EndDate:   timestamppb.New(end),
		}

		resp, err := handler.GetOverallQualityScore(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)

		// Should return InvalidArgument error
		require.Contains(t, err.Error(), "end date must be after start date")
	})
}

func TestE2E_FullWorkflow(t *testing.T) {
	// This test validates the complete end-to-end workflow
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRatingScoreRepository(db)
	cache := &mocks.InMemoryCache{}
	logger := zap.NewNop()

	svc := service.NewScoringService(repo, logger)
	handler := grpc.NewGRPCHandlers(svc, cache, logger, 5*time.Minute)

	ctx := context.Background()
	start := testBaseDate
	end := start.Add(24 * time.Hour)

	req := &pb.TimePeriodRequest{
		StartDate: timestamppb.New(start),
		EndDate:   timestamppb.New(end),
	}

	// Test all endpoints work together and provide consistent data
	t.Run("overall score matches category aggregation", func(t *testing.T) {
		overallResp, err := handler.GetOverallQualityScore(ctx, req)
		require.NoError(t, err)

		categoryResp, err := handler.GetAggregatedCategoryScores(ctx, req)
		require.NoError(t, err)

		// The overall score should be related to category scores
		// (exact calculation depends on weighting, but should be in same ballpark)
		var totalWeightedScore float64
		var totalWeight float64

		for _, cat := range categoryResp.CategoryScores {
			// Use category weight from our test data
			var weight float64
			switch cat.CategoryName {
			case "Tone":
				weight = 1.0
			case "Grammar":
				weight = 2.0
			case "GDPR":
				weight = 1.5
			}
			totalWeightedScore += cat.OverallCategoryScore * weight
			totalWeight += weight
		}

		expectedOverall := totalWeightedScore / totalWeight
		require.InDelta(t, expectedOverall, overallResp.Score, 5.0,
			"Overall score should approximate weighted category average")
	})

	t.Run("ticket scores sum to category totals", func(t *testing.T) {
		ticketResp, err := handler.GetScoresByTicket(ctx, req)
		require.NoError(t, err)

		categoryResp, err := handler.GetAggregatedCategoryScores(ctx, req)
		require.NoError(t, err)

		// Count ratings per category from ticket data
		categoryRatingCounts := make(map[string]int)
		for _, ticket := range ticketResp.TicketScores {
			for category := range ticket.CategoryScores {
				categoryRatingCounts[category]++
			}
		}

		// Verify counts match category aggregation
		for _, cat := range categoryResp.CategoryScores {
			expectedCount := categoryRatingCounts[cat.CategoryName]
			require.Equal(t, int64(expectedCount), cat.TotalRatings,
				"Category %s rating count should match ticket aggregation", cat.CategoryName)
		}
	})
}
