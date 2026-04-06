// Package server provides gRPC and HTTP server implementations
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "usdt-rates/api/proto/rates"
	"usdt-rates/internal/calculator"
	"usdt-rates/internal/service"
)

// GRPCServer wraps the gRPC server and rates service
type GRPCServer struct {
	pb.UnimplementedRatesServiceServer
	server  *grpc.Server
	service *service.RatesService
	logger  *zap.Logger
	port    string
}

// NewGRPCServer creates and configures a new gRPC server
func NewGRPCServer(svc *service.RatesService, port string, logger *zap.Logger) *GRPCServer {
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(logger),
			metricsInterceptor(),
		),
	)

	s := &GRPCServer{
		server:  srv,
		service: svc,
		logger:  logger,
		port:    port,
	}

	pb.RegisterRatesServiceServer(srv, s)
	reflection.Register(srv)

	return s
}

// Start begins listening for gRPC connections
func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", s.port, err)
	}

	s.logger.Info("gRPC server starting", zap.String("port", s.port))
	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	s.logger.Info("gRPC server shutting down gracefully")
	s.server.GracefulStop()
}

// ForceStop immediately stops the gRPC server
func (s *GRPCServer) ForceStop() {
	s.logger.Warn("gRPC server force stopping")
	s.server.Stop()
}

// GetRates implements the GetRates RPC method
func (s *GRPCServer) GetRates(ctx context.Context, req *pb.GetRatesRequest) (*pb.GetRatesResponse, error) {
	// Validate inputs at the gRPC boundary
	method := req.GetMethod()
	if method != "topN" && method != "avgNM" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid method %q, use 'topN' or 'avgNM'", method)
	}
	if req.GetN() < 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "parameter N must be >= 0")
	}
	if method == "avgNM" && req.GetM() < 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "parameter M must be >= 0")
	}

	result, err := s.service.GetRates(ctx, method, int(req.GetN()), int(req.GetM()))
	if err != nil {
		s.logger.Error("GetRates failed",
			zap.String("method", req.GetMethod()),
			zap.Int32("n", req.GetN()),
			zap.Int32("m", req.GetM()),
			zap.Error(err),
		)
		return nil, toGRPCError(err)
	}

	return &pb.GetRatesResponse{
		Ask:       strconv.FormatFloat(result.Ask, 'f', 8, 64),
		Bid:       strconv.FormatFloat(result.Bid, 'f', 8, 64),
		Timestamp: timestamppb.New(result.Timestamp),
	}, nil
}

// HealthCheck implements the HealthCheck RPC method
func (s *GRPCServer) HealthCheck(ctx context.Context, _ *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	if err := s.service.HealthCheck(ctx); err != nil {
		s.logger.Warn("health check failed", zap.Error(err))
		return &pb.HealthCheckResponse{Status: "NOT_SERVING"}, nil
	}
	return &pb.HealthCheckResponse{Status: "SERVING"}, nil
}

// toGRPCError converts service errors to proper gRPC status codes
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, calculator.ErrInvalidMethod):
		return grpcstatus.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, calculator.ErrIndexOutOfRange):
		return grpcstatus.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, calculator.ErrInvalidRange):
		return grpcstatus.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, calculator.ErrEmptyPrices):
		return grpcstatus.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return grpcstatus.Error(codes.DeadlineExceeded, "request timeout")
	case errors.Is(err, context.Canceled):
		return grpcstatus.Error(codes.Canceled, "request canceled")
	default:
		errMsg := err.Error()
		// Wrapped context errors from libraries (e.g., lib/pq)
		if strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "context deadline") {
			return grpcstatus.Error(codes.DeadlineExceeded, "request timeout")
		}
		if strings.Contains(errMsg, "context canceled") {
			return grpcstatus.Error(codes.Canceled, "request canceled")
		}
		// External service failures
		if strings.Contains(errMsg, "failed to fetch order book") || strings.Contains(errMsg, "empty order book") {
			return grpcstatus.Error(codes.Unavailable, "exchange data temporarily unavailable")
		}
		return grpcstatus.Error(codes.Internal, "internal error")
	}
}

// loggingInterceptor logs each gRPC request with duration
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		} else {
			logger.Info("gRPC request",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
			)
		}
		return resp, err
	}
}

// metricsInterceptor records Prometheus metrics for each gRPC request
func metricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start).Seconds()

		status := "success"
		if err != nil {
			status = "error"
		}

		RatesRequests.WithLabelValues(info.FullMethod, status).Inc()
		RatesLatency.WithLabelValues(info.FullMethod).Observe(duration)

		return resp, err
	}
}
