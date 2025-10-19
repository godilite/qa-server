package service

import (
	"context"
	"testing"
	"time"

	"github.com/godilite/qa-server/internal/repository"
	dbbuilder "github.com/godilite/qa-server/pkg/database"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func setupRealDB(tb testing.TB) *repository.RatingScoreRepository {
	tb.Helper()

	db, err := dbbuilder.New(
		dbbuilder.WithDriver("sqlite3"),
		dbbuilder.WithDataSource(":memory:"),
		dbbuilder.WithMaxOpenConns(1),
	)
	if err != nil {
		tb.Fatalf("failed to create db pool via builder: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE rating_categories (id INTEGER PRIMARY KEY, name TEXT, weight REAL);
		CREATE TABLE ratings (
			id INTEGER PRIMARY KEY,
			rating_category_id INTEGER,
			rating INTEGER,
			created_at TEXT,
			ticket_id INTEGER
		);
		INSERT INTO rating_categories (id, name, weight) VALUES (1, 'Tone', 3.0);
		INSERT INTO ratings (rating_category_id, rating, created_at, ticket_id)
			VALUES (1, 5, '2025-10-15T10:30:00Z', 101),
			       (1, 4, '2025-10-16T10:30:00Z', 102),
			       (1, 5, '2025-10-17T10:30:00Z', 103);
	`)
	if err != nil {
		db.Close()
		tb.Fatalf("failed to seed db: %v", err)
	}

	tb.Cleanup(func() { db.Close() })

	repo := repository.NewRatingScoreRepository(db)
	return repo
}

func BenchmarkGetOverallScore(b *testing.B) {
	start := time.Now().Add(-72 * time.Hour)
	end := time.Now()
	logger := zap.NewNop()
	repo := setupRealDB(b)

	svc := NewScoringService(repo, logger)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = svc.GetOverallScore(context.Background(), start, end)
	}
}
