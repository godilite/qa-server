package server

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestLoggingInterceptor(t *testing.T) {
	logger := zaptest.NewLogger(t)

	interceptor := LoggingInterceptor(logger)

	successHandler := func(ctx context.Context, req any) (any, error) {
		return "success", nil
	}

	errorHandler := func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "test error")
	}

	// Test successful request
	t.Run("successful request", func(t *testing.T) {
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		resp, err := interceptor(context.Background(), "test request", info, successHandler)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resp != "success" {
			t.Errorf("Expected 'success', got %v", resp)
		}
	})

	// Test error request
	t.Run("error request", func(t *testing.T) {
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		_, err := interceptor(context.Background(), "test request", info, errorHandler)
		if err == nil {
			t.Error("Expected an error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Error("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestServerBuilderWithLogging(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test server with logging enabled
	server, err := New(
		WithPort(50051),
		WithLogger(logger),
		WithLogging(true),
	)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() {
		ctx := context.Background()
		if err := server.Shutdown(ctx); err != nil {
			t.Logf("Server shutdown error: %v", err)
		}
	}()

	// Verify server was created successfully
	if server.grpcServer == nil {
		t.Error("gRPC server should not be nil")
	}
	if server.logger == nil {
		t.Error("Logger should not be nil")
	}
	if server.healthServer == nil {
		t.Error("Health server should not be nil")
	}

	// Test that health check works
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.NewClient(server.lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	// Start server in background
	server.Start()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test health check
	healthClient := healthpb.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("Expected SERVING status, got %v", resp.Status)
	}
}
