package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/godilite/qa-server/internal/repository/models"
)

type RatingScoreRepository struct {
	db *sql.DB
}

func NewRatingScoreRepository(db *sql.DB) *RatingScoreRepository {
	return &RatingScoreRepository{db: db}
}

// GetOverallRatings fetches weighted score computed entirely in SQL.
func (s *RatingScoreRepository) GetOverallRatings(ctx context.Context, start, end time.Time) (models.OverallRatingResult, error) {
	const query = `
		SELECT
			CASE 
				WHEN SUM(rc.weight) > 0 
				THEN SUM(CAST(r.rating AS REAL) * 20.0 * rc.weight) / SUM(rc.weight)
				ELSE 0
			END AS score,
			COUNT(r.id) AS count
		FROM ratings AS r
		JOIN rating_categories AS rc ON r.rating_category_id = rc.id
		WHERE r.created_at >= ? AND r.created_at <= ?
	`

	var score sql.NullFloat64
	var count sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, start, end).Scan(&score, &count)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.OverallRatingResult{Score: 0, Count: 0}, nil
		}
		return models.OverallRatingResult{}, fmt.Errorf("query GetOverallRatings: %w", err)
	}

	// SQL handles all computation - just return the results
	result := models.OverallRatingResult{
		Score: 0,
		Count: 0,
	}

	if count.Valid {
		result.Count = count.Int64
	}
	if score.Valid {
		result.Score = score.Float64
	}

	return result, nil
}

// GetRatingsInPeriod aggregates ratings by category and daily or weekly period with SQL-computed scores.
func (s *RatingScoreRepository) GetRatingsInPeriod(ctx context.Context, start, end time.Time, isWeekly bool) ([]models.AggregatedCategoryData, error) {
	periodFormat := "%Y-%m-%d"
	if isWeekly {
		periodFormat = "%Y-W%W"
	}

	const query = `
		SELECT
			rc.name AS category,
			strftime(?, r.created_at) AS period,
			CASE 
				WHEN SUM(rc.weight) > 0 
				THEN SUM(CAST(r.rating AS REAL) * 20.0 * rc.weight) / SUM(rc.weight)
				ELSE 0
			END AS period_score,
			SUM(CAST(r.rating AS REAL) * rc.weight) AS total_weighted_rating,
			SUM(rc.weight) AS total_weight,
			COUNT(r.id) AS rating_count
		FROM ratings AS r
		JOIN rating_categories AS rc ON r.rating_category_id = rc.id
		WHERE r.created_at >= ? AND r.created_at <= ?
		GROUP BY category, period
		ORDER BY category, period
	`

	rows, err := s.db.QueryContext(ctx, query, periodFormat, start, end)
	if err != nil {
		return nil, fmt.Errorf("query GetRatingsInPeriod: %w", err)
	}
	defer rows.Close()

	var results []models.AggregatedCategoryData
	for rows.Next() {
		var r models.AggregatedCategoryData
		if err := rows.Scan(&r.Category, &r.Period, &r.PeriodScore, &r.TotalWeightedEvaluation, &r.TotalWeight, &r.EvaluationCount); err != nil {
			return nil, fmt.Errorf("scan GetRatingsInPeriod row: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GetRatingsInPeriod: %w", err)
	}
	return results, nil
}

// GetScoresByTicket aggregates scores grouped by ticket and category with SQL-computed scores.
func (s *RatingScoreRepository) GetScoresByTicket(ctx context.Context, start, end time.Time) ([]models.TicketCategoryScore, error) {
	const query = `
		SELECT
			r.ticket_id,
			rc.name AS category,
			CASE
				WHEN SUM(rc.weight) > 0
				THEN SUM(CAST(r.rating AS REAL) * 20.0 * rc.weight) / SUM(rc.weight)
				ELSE 0
			END AS score
		FROM ratings AS r
		JOIN rating_categories AS rc ON r.rating_category_id = rc.id
		WHERE r.created_at >= ? AND r.created_at <= ?
		GROUP BY r.ticket_id, rc.name
		ORDER BY r.ticket_id, rc.name
	`

	rows, err := s.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("query GetScoresByTicket: %w", err)
	}
	defer rows.Close()

	var results []models.TicketCategoryScore
	for rows.Next() {
		var tcs models.TicketCategoryScore
		if err := rows.Scan(&tcs.TicketID, &tcs.Category, &tcs.Score); err != nil {
			return nil, fmt.Errorf("scan GetScoresByTicket row: %w", err)
		}
		results = append(results, tcs)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GetScoresByTicket: %w", err)
	}
	return results, nil
}
