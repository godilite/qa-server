package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/godilite/qa-server/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

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
		ticket_id INTEGER NOT NULL,
		rating INTEGER NOT NULL,
		rating_category_id INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY (rating_category_id) REFERENCES rating_categories(id)
	);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	return db
}

func seedTestData(t *testing.T, db *sql.DB, baseTime time.Time) {
	t.Helper()

	_, err := db.Exec(`
	INSERT INTO rating_categories (name, weight)
	VALUES ('Spelling', 1.0), ('Grammar', 0.7), ('GDPR', 1.2);
	`)
	require.NoError(t, err)

	ratings := []struct {
		ticketID int
		rating   int
		category int
		offset   time.Duration
	}{
		{ticketID: 1001, rating: 5, category: 1, offset: 0},
		{ticketID: 1001, rating: 4, category: 2, offset: 0},
		{ticketID: 1002, rating: 3, category: 1, offset: 0},
		{ticketID: 1002, rating: 5, category: 3, offset: 0},
		{ticketID: 1003, rating: 2, category: 1, offset: 24 * time.Hour},
	}

	for _, r := range ratings {
		ts := baseTime.Add(r.offset).Format(time.RFC3339)
		_, err := db.Exec(`
			INSERT INTO ratings (ticket_id, rating, rating_category_id, created_at)
			VALUES (?, ?, ?, ?);
		`, r.ticketID, r.rating, r.category, ts)
		require.NoError(t, err)
	}
}

func TestRatingScoreRepository_Integration(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	baseTime := time.Date(2025, 10, 18, 10, 0, 0, 0, time.UTC)
	seedTestData(t, db, baseTime)

	repo := repository.NewRatingScoreRepository(db)
	start := baseTime.Add(-time.Hour)
	end := baseTime.Add(48 * time.Hour)

	t.Run("GetOverallRatings", func(t *testing.T) {
		result, err := repo.GetOverallRatings(ctx, start, end)
		require.NoError(t, err)
		require.Greater(t, result.Count, int64(0))
		require.GreaterOrEqual(t, result.Score, 0.0)
	})

	t.Run("GetRatingsInPeriod - daily", func(t *testing.T) {
		results, err := repo.GetRatingsInPeriod(ctx, start, end, false)
		require.NoError(t, err)

		require.NotEmpty(t, results)
		require.GreaterOrEqual(t, len(results), 2)

		days := make(map[string]bool)
		for _, r := range results {
			days[r.Period] = true
		}
		require.Len(t, days, 2, "expected aggregation over two distinct days")
	})

	t.Run("GetRatingsInPeriod - weekly", func(t *testing.T) {
		results, err := repo.GetRatingsInPeriod(ctx, start, end, true)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		for _, r := range results {
			require.Contains(t, r.Period, "W")
		}
	})

	t.Run("GetScoresByTicket", func(t *testing.T) {
		results, err := repo.GetScoresByTicket(ctx, start, end)
		require.NoError(t, err)

		require.Len(t, results, 5)
		var found bool
		for _, r := range results {
			if r.TicketID == 1001 && r.Category == "Grammar" {
				require.GreaterOrEqual(t, r.Score, 0.0)
				found = true
			}
		}
		require.True(t, found, "expected Grammar category for ticket 1001")
	})
}
