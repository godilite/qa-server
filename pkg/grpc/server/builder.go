package server

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Option func(*Options)

type Options struct {
	port              int
	logger            *zap.Logger
	reflection        bool
	unaryInterceptors []grpc.UnaryServerInterceptor
	enableLogging     bool
}

func WithPort(port int) Option {
	return func(o *Options) {
		o.port = port
	}
}

func WithLogger(logger *zap.Logger) Option {
	return func(o *Options) {
		o.logger = logger
	}
}

func WithReflection(enabled bool) Option {
	return func(o *Options) {
		o.reflection = enabled
	}
}

func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(o *Options) {
		o.unaryInterceptors = append(o.unaryInterceptors, interceptors...)
	}
}

func WithLogging(enabled bool) Option {
	return func(o *Options) {
		o.enableLogging = enabled
	}
}

type Server struct {
	grpcServer   *grpc.Server
	lis          net.Listener
	logger       *zap.Logger
	healthServer *health.Server
}

// New creates a new gRPC server using the builder options.
func New(opts ...Option) (*Server, error) {
	options := &Options{
		port:       50051,
		logger:     zap.NewNop(),
		reflection: false,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Validate port range
	if options.port < 1 || options.port > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 1 and 65535", options.port)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", options.port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", options.port, err)
	}

	logger := options.logger
	if logger == nil {
		logger = zap.NewNop()
	}

	serverOpts := []grpc.ServerOption{}

	var interceptors []grpc.UnaryServerInterceptor
	if options.enableLogging {
		interceptors = append(interceptors, LoggingInterceptor(logger))
	}
	interceptors = append(interceptors, options.unaryInterceptors...)

	if len(interceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	grpcServer := grpc.NewServer(serverOpts...)

	if options.reflection {
		reflection.Register(grpcServer)
	}

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	return &Server{
		grpcServer:   grpcServer,
		lis:          lis,
		logger:       logger.Named("grpc-server"),
		healthServer: healthServer,
	}, nil
}

// RegisterService allows the main application to register its specific service.
func (s *Server) RegisterService(registerFunc func(s *grpc.Server)) {
	registerFunc(s.grpcServer)
}

// RegisterServiceWithHealth registers a service and sets its health status.
func (s *Server) RegisterServiceWithHealth(serviceName string, registerFunc func(s *grpc.Server)) {
	registerFunc(s.grpcServer)

	if s.healthServer != nil && serviceName != "" {
		s.healthServer.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)
		s.logger.Info("registered service with health check", zap.String("service", serviceName))
	}
}

// SetServiceHealth updates the health status of a specific service.
func (s *Server) SetServiceHealth(serviceName string, status healthpb.HealthCheckResponse_ServingStatus) {
	if s.healthServer != nil {
		s.healthServer.SetServingStatus(serviceName, status)
		s.logger.Info("updated service health",
			zap.String("service", serviceName),
			zap.String("status", status.String()))
	}
}

// Start runs the server in a goroutine and returns immediately.
func (s *Server) Start() {
	s.logger.Info("gRPC server starting", zap.String("addr", s.lis.Addr().String()))

	go func() {
		if err := s.grpcServer.Serve(s.lis); err != nil {
			s.logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	s.logger.Info("gRPC server started", zap.String("addr", s.lis.Addr().String()))
}

// Shutdown gracefully shuts down the server with a timeout context.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("gRPC server shutting down")

	if s.healthServer != nil {
		s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	}

	done := make(chan struct{})

	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("gRPC server stopped")
		return nil
	case <-ctx.Done():
		s.logger.Warn("forced shutdown due to timeout")
		s.grpcServer.Stop()
		return ctx.Err()
	}
}

// Addr returns the server's listening address.
func (s *Server) Addr() net.Addr {
	return s.lis.Addr()
}
