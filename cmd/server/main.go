package main

import (
	"context"
	"log"

	"github.com/godilite/qa-server/internal/app"
	"github.com/godilite/qa-server/internal/config"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load(".env")

	cfg := config.LoadFromEnv()

	logger, err := config.NewLogger(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	ctx := context.Background()
	application, err := app.NewApp(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize application", zap.Error(err))
	}

	if err := application.Run(); err != nil {
		logger.Fatal("Application exited with error", zap.Error(err))
	}
}
