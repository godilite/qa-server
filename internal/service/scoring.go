package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"
)

const (
	dbTimeout = 1 * time.Second
)

// ScoringService handles rating aggregation and scoring.
type ScoringService struct {
	storage RatingScoreRepository
	logger  *zap.Logger
}

// NewScoringService creates a new ScoringService instance.
func NewScoringService(storage RatingScoreRepository, logger *zap.Logger) *ScoringService {
	if storage == nil {
		panic("storage must not be nil")
	}
	if logger == nil {
		l, _ := zap.NewProduction()
		logger = l
	}
	return &ScoringService{
		storage: storage,
		logger:  logger,
	}
}

var (
	ErrNoRatings      = errors.New("no ratings found")
	ErrStorageFailure = errors.New("storage failure")
)

func isAtLeastOneMonth(start, end time.Time) bool {
	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	oneMonthLater := s.AddDate(0, 1, 0)
	return !oneMonthLater.After(e)
}

func isWeeklyAggregation(start, end time.Time) bool {
	if isAtLeastOneMonth(start, end) {
		return true
	}
	if end.Sub(start) >= 28*24*time.Hour {
		return true
	}
	return false
}

// GetOverallScore returns the overall weighted score for the requested window.
func (s *ScoringService) GetOverallScore(ctx context.Context, start, end time.Time) (float64, error) {

	dbCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	result, err := s.storage.GetOverallRatings(dbCtx, start, end)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	if result.Count == 0 {
		return 0, ErrNoRatings
	}

	s.logger.Info("fetched overall score",
		zap.Float64("score", result.Score),
		zap.Int64("count", result.Count),
		zap.Time("start", start),
		zap.Time("end", end))

	return result.Score, nil
}

// GetAggregatedCategoryScores returns per-category (daily or weekly) aggregates.
func (s *ScoringService) GetAggregatedCategoryScores(ctx context.Context, start, end time.Time) ([]AggregatedCategoryScores, error) {

	dbCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	weekly := isWeeklyAggregation(start, end)
	rows, err := s.storage.GetRatingsInPeriod(dbCtx, start, end, weekly)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	if len(rows) == 0 {
		return nil, ErrNoRatings
	}

	resultsMap := make(map[string]*AggregatedCategoryScores)
	overallStats := make(map[string]struct {
		totalWeighted float64
		totalWeight   float64
	})

	for _, r := range rows {
		c := r.Category
		if _, ok := resultsMap[c]; !ok {
			resultsMap[c] = &AggregatedCategoryScores{
				CategoryName: c,
				PeriodScores: make([]PeriodScore, 0),
			}
		}

		resultsMap[c].PeriodScores = append(resultsMap[c].PeriodScores, PeriodScore{
			Period: r.Period,
			Score:  r.PeriodScore,
		})
		resultsMap[c].TotalRatings += r.EvaluationCount

		stats := overallStats[c]
		stats.totalWeighted += r.TotalWeightedEvaluation
		stats.totalWeight += r.TotalWeight
		overallStats[c] = stats
	}

	results := make([]AggregatedCategoryScores, 0, len(resultsMap))
	for cat, v := range resultsMap {
		sort.Slice(v.PeriodScores, func(i, j int) bool {
			return v.PeriodScores[i].Period < v.PeriodScores[j].Period
		})

		stats := overallStats[cat]
		if stats.totalWeight > 0 {
			v.OverallCategoryScore = (stats.totalWeighted * 20.0) / stats.totalWeight
		}
		results = append(results, *v)
	}
	return results, nil
}

// GetScoresByTicket pivots pre-aggregated per-ticket rows into TicketScores.
func (s *ScoringService) GetScoresByTicket(ctx context.Context, start, end time.Time) ([]TicketScores, error) {
	dbCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	rows, err := s.storage.GetScoresByTicket(dbCtx, start, end)
	if err != nil {
		s.logger.Error("failed to fetch scores by ticket", zap.Error(err))
		return nil, fmt.Errorf("fetch scores by ticket: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNoRatings
	}

	pivot := make(map[int64]map[string]float64)
	for _, r := range rows {
		if _, ok := pivot[r.TicketID]; !ok {
			pivot[r.TicketID] = make(map[string]float64)
		}
		pivot[r.TicketID][r.Category] = r.Score
	}

	out := make([]TicketScores, 0, len(pivot))
	for tid, m := range pivot {
		out = append(out, TicketScores{
			TicketID:       tid,
			CategoryScores: m,
		})
	}

	return out, nil
}

// GetPeriodOverPeriodScoreChange calculates the score change vs the previous period.
func (s *ScoringService) GetPeriodOverPeriodScoreChange(ctx context.Context, start, end time.Time) (PeriodChange, error) {

	currentScore, err := s.GetOverallScore(ctx, start, end)
	if err != nil {
		return PeriodChange{}, fmt.Errorf("current score: %w", err)
	}

	duration := end.Sub(start)
	prevEnd := start.Add(-time.Nanosecond)
	prevStart := prevEnd.Add(-duration + time.Nanosecond)

	previousScore, err := s.GetOverallScore(ctx, prevStart, prevEnd)
	if err != nil {
		if errors.Is(err, ErrNoRatings) {
			return PeriodChange{
				CurrentPeriodScore:  currentScore,
				PreviousPeriodScore: 0,
				ChangePercentage:    100.0,
			}, nil
		}
		return PeriodChange{}, fmt.Errorf("previous score: %w", err)
	}

	var change float64
	if previousScore > 0 {
		change = ((currentScore - previousScore) / previousScore) * 100.0
	} else if currentScore > 0 {
		change = 100.0
	}

	return PeriodChange{
		CurrentPeriodScore:  currentScore,
		PreviousPeriodScore: previousScore,
		ChangePercentage:    change,
	}, nil
}
