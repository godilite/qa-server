package server

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor creates a gRPC unary interceptor for request/response logging.
func LoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		logger.Info("gRPC request started",
			zap.String("method", info.FullMethod),
			zap.String("client_addr", "client"))

		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			st, _ := status.FromError(err)
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.String("status_code", st.Code().String()),
				zap.String("status_message", st.Message()),
				zap.Error(err))
		} else {
			logger.Info("gRPC request completed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.String("status_code", codes.OK.String()))
		}

		return resp, err
	}
}
