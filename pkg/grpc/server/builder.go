package server

import (
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
	port       int
	logger     *zap.Logger
	reflection bool
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

type Server struct {
	grpcServer *grpc.Server
	lis        net.Listener
	logger     *zap.Logger
}

// New creates a new gRPC server using the builder options.
func New(opts ...Option) (*Server, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", options.port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()

	if options.reflection {
		reflection.Register(grpcServer)
	}

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	return &Server{
		grpcServer: grpcServer,
		lis:        lis,
		logger:     options.logger.Named("grpc-server"),
	}, nil
}

// RegisterService allows the main application to register its specific service.
func (s *Server) RegisterService(registerFunc func(s *grpc.Server)) {
	registerFunc(s.grpcServer)
}

// Start runs the server in a goroutine.
func (s *Server) Start() {
	s.logger.Info("gRPC server starting", zap.String("addr", s.lis.Addr().String()))
	go func() {
		if err := s.grpcServer.Serve(s.lis); err != nil {
			s.logger.Error("gRPC server failed", zap.Error(err))
		}
	}()
	s.logger.Info("gRPC server started", zap.String("addr", s.lis.Addr().String()))
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	s.logger.Info("gRPC server shutting down")
	s.grpcServer.GracefulStop()
}
