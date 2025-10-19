package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	pb "github.com/godilite/qa-server/api/v1"
	"github.com/godilite/qa-server/internal/service"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultCacheDuration = 10 * time.Minute
	defaultGRPCTimeout   = 10 * time.Second
)

type CacheKeyType string

const (
	cacheKeyOverallScore       CacheKeyType = "grpc:overall_quality_score"
	cacheKeyTicketScores       CacheKeyType = "grpc:scores_by_ticket"
	cacheKeyPeriodChange       CacheKeyType = "grpc:period_over_period_score_change"
	cacheKeyAggregatedCategory CacheKeyType = "grpc:aggregated_category_scores"
)

type GRPCHandlers struct {
	pb.UnimplementedTicketScoringServer
	scoring  ScoringService
	cache    Cacher
	logger   *zap.Logger
	sfGroup  singleflight.Group
	cacheTTL time.Duration
}

// NewGRPCHandlers initializes the gRPC handlers.
func NewGRPCHandlers(scoring ScoringService, cache Cacher, logger *zap.Logger, ttl time.Duration) *GRPCHandlers {
	if scoring == nil {
		panic("nil ScoringService provided to NewGRPCHandlers")
	}
	if ttl <= 0 {
		ttl = defaultCacheDuration
	}
	return &GRPCHandlers{
		scoring:  scoring,
		cache:    cache,
		logger:   logger.Named("grpc-handler"),
		cacheTTL: ttl,
	}
}

func (s *GRPCHandlers) parseAndValidate(req *pb.TimePeriodRequest) (start, end time.Time, err error) {
	start = req.GetStartDate().AsTime()
	end = req.GetEndDate().AsTime()

	if start.IsZero() || end.IsZero() {
		err = status.Error(codes.InvalidArgument, "start and end dates are required")
		return
	}

	if end.Before(start) {
		err = status.Error(codes.InvalidArgument, "end date must be after start date")
		return
	}

	return
}

func normalizeKey(prefix CacheKeyType, start, end time.Time) string {
	s := start.UTC().Truncate(24 * time.Hour).Format("2006-01-02")
	e := end.UTC().Truncate(24 * time.Hour).Format("2006-01-02")
	return fmt.Sprintf("%s:%s:%s", prefix, s, e)
}

func (s *GRPCHandlers) handleError(ctx context.Context, op string, err error) error {
	switch ctx.Err() {
	case context.Canceled:
		s.logger.Warn("request canceled", zap.String("op", op))
		return status.Error(codes.Canceled, "request canceled")
	case context.DeadlineExceeded:
		s.logger.Warn("request timeout", zap.String("op", op))
		return status.Error(codes.DeadlineExceeded, "request timed out")
	}

	switch {
	case errors.Is(err, service.ErrNoRatings):
		s.logger.Info("no ratings found", zap.String("op", op))
		return status.Error(codes.NotFound, "no ratings found for the given period")
	case errors.Is(err, service.ErrStorageFailure):
		s.logger.Error("storage failure", zap.String("op", op), zap.Error(err))
		return status.Error(codes.Internal, "database error")
	default:
		s.logger.Error("unexpected error", zap.String("op", op), zap.Error(err))
		return status.Errorf(codes.Internal, "%s failed: %v", op, err)
	}
}

func (s *GRPCHandlers) GetOverallQualityScore(ctx context.Context, req *pb.TimePeriodRequest) (*pb.OverallQualityScoreResponse, error) {
	start, end, err := s.parseAndValidate(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGRPCTimeout)
	defer cancel()

	cacheKey := normalizeKey(cacheKeyOverallScore, start, end)

	score, err := FindAndCache(ctx, s.cache, &s.sfGroup, string(cacheKey), s.cacheTTL, s.logger, func(fetchCtx context.Context) (float64, error) {
		return s.scoring.GetOverallScore(fetchCtx, start, end)
	})
	if err != nil {
		return nil, s.handleError(ctx, "GetOverallQualityScore", err)
	}

	return &pb.OverallQualityScoreResponse{Score: score}, nil
}

func (s *GRPCHandlers) GetScoresByTicket(ctx context.Context, req *pb.TimePeriodRequest) (*pb.ScoresByTicketResponse, error) {
	start, end, err := s.parseAndValidate(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGRPCTimeout)
	defer cancel()

	cacheKey := normalizeKey(cacheKeyTicketScores, start, end)

	scores, err := FindAndCache(ctx, s.cache, &s.sfGroup, string(cacheKey), s.cacheTTL, s.logger, func(fetchCtx context.Context) ([]service.TicketScores, error) {
		return s.scoring.GetScoresByTicket(fetchCtx, start, end)
	})
	if err != nil {
		return nil, s.handleError(ctx, "GetScoresByTicket", err)
	}

	pbScores := make([]*pb.TicketScore, len(scores))
	for i, score := range scores {
		pbScores[i] = &pb.TicketScore{
			TicketId:       score.TicketID,
			CategoryScores: score.CategoryScores,
		}
	}

	return &pb.ScoresByTicketResponse{TicketScores: pbScores}, nil
}

func (s *GRPCHandlers) GetPeriodOverPeriodScoreChange(ctx context.Context, req *pb.TimePeriodRequest) (*pb.PeriodOverPeriodScoreChangeResponse, error) {
	start, end, err := s.parseAndValidate(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGRPCTimeout)
	defer cancel()

	cacheKey := normalizeKey(cacheKeyPeriodChange, start, end)

	change, err := FindAndCache(ctx, s.cache, &s.sfGroup, string(cacheKey), s.cacheTTL, s.logger, func(fetchCtx context.Context) (service.PeriodChange, error) {
		return s.scoring.GetPeriodOverPeriodScoreChange(fetchCtx, start, end)
	})
	if err != nil {
		return nil, s.handleError(ctx, "GetPeriodOverPeriodScoreChange", err)
	}

	return &pb.PeriodOverPeriodScoreChangeResponse{
		CurrentPeriodScore:  change.CurrentPeriodScore,
		PreviousPeriodScore: change.PreviousPeriodScore,
		ChangePercentage:    change.ChangePercentage,
	}, nil
}

func (s *GRPCHandlers) GetAggregatedCategoryScores(ctx context.Context, req *pb.TimePeriodRequest) (*pb.AggregatedCategoryScoresResponse, error) {
	start, end, err := s.parseAndValidate(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGRPCTimeout)
	defer cancel()

	cacheKey := normalizeKey(cacheKeyAggregatedCategory, start, end)

	results, err := FindAndCache(ctx, s.cache, &s.sfGroup, string(cacheKey), s.cacheTTL, s.logger, func(fetchCtx context.Context) ([]service.AggregatedCategoryScores, error) {
		return s.scoring.GetAggregatedCategoryScores(fetchCtx, start, end)
	})
	if err != nil {
		return nil, s.handleError(ctx, "GetAggregatedCategoryScores", err)
	}

	pbScores := s.mapToProtoCategoryScores(results)
	return &pb.AggregatedCategoryScoresResponse{CategoryScores: pbScores}, nil
}

func (s *GRPCHandlers) mapToProtoCategoryScores(scores []service.AggregatedCategoryScores) []*pb.CategoryScore {
	out := make([]*pb.CategoryScore, len(scores))
	for i, cat := range scores {
		periods := make([]*pb.PeriodScore, len(cat.PeriodScores))
		for j, p := range cat.PeriodScores {
			periods[j] = &pb.PeriodScore{
				Period: p.Period,
				Score:  p.Score,
			}
		}
		out[i] = &pb.CategoryScore{
			CategoryName:         cat.CategoryName,
			TotalRatings:         int64(cat.TotalRatings),
			OverallCategoryScore: cat.OverallCategoryScore,
			PeriodScores:         periods,
		}
	}
	return out
}
