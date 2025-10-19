package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/godilite/qa-server/api/v1"
	"github.com/godilite/qa-server/internal/config"
	handler "github.com/godilite/qa-server/internal/grpc"
	"github.com/godilite/qa-server/internal/repository"
	"github.com/godilite/qa-server/internal/service"
	"github.com/godilite/qa-server/pkg/cache"
	dbbuilder "github.com/godilite/qa-server/pkg/database"
	grpcsrv "github.com/godilite/qa-server/pkg/grpc/server"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type App struct {
	logger     *zap.Logger
	dbPool     *sql.DB
	cache      *cache.Cache
	grpcServer *grpcsrv.Server
}

func NewApp(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*App, error) {
	dbPool, err := dbbuilder.New(
		dbbuilder.WithDriver(cfg.DBDriver),
		dbbuilder.WithDataSource(cfg.DBPath),
	)
	if err != nil {
		return nil, fmt.Errorf("database init failed: %w", err)
	}
	logger.Info("Database pool initialized", zap.String("path", cfg.DBPath))

	cacheClient, err := cache.New(ctx,
		cache.WithAddress(cfg.RedisAddr),
	)
	if err != nil {
		return nil, fmt.Errorf("cache init failed: %w", err)
	}
	logger.Info("Cache client initialized", zap.String("addr", cfg.RedisAddr))

	scoringRepo := repository.NewRatingScoreRepository(dbPool)

	scoringService := service.NewScoringService(scoringRepo, logger)

	grpcHandlers := handler.NewGRPCHandlers(scoringService, cacheClient, logger, 10*time.Minute)

	grpcServer, err := grpcsrv.New(
		grpcsrv.WithPort(cfg.GRPCPort),
		grpcsrv.WithLogger(logger),
		grpcsrv.WithReflection(cfg.GRPCReflectionEnabled),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	grpcServer.RegisterService(func(s *grpc.Server) {
		pb.RegisterTicketScoringServer(s, grpcHandlers)
	})

	return &App{
		logger:     logger,
		dbPool:     dbPool,
		cache:      cacheClient,
		grpcServer: grpcServer,
	}, nil
}

// Run starts the application and blocks until a shutdown signal is received.
func (a *App) Run() error {
	a.logger.Info("application starting")

	a.grpcServer.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.logger.Info("application shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	a.grpcServer.Stop()

	if err := a.cache.Close(); err != nil {
		a.logger.Error("cache shutdown error", zap.Error(err))
	}
	if err := a.dbPool.Close(); err != nil {
		a.logger.Error("database shutdown error", zap.Error(err))
	}

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			a.logger.Warn("shutdown completed but deadline exceeded")
		}
	default:
		a.logger.Info("graceful shutdown completed successfully")
	}

	_ = a.logger.Sync()
	return nil
}
