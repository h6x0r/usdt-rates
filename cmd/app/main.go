// Package main is the entrypoint for the USDT rates gRPC service
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"

	"usdt-rates/internal/client"
	"usdt-rates/internal/config"
	"usdt-rates/internal/repository"
	"usdt-rates/internal/server"
	"usdt-rates/internal/service"
)

const shutdownTimeout = 15 * time.Second

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	// Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("invalid configuration", zap.Error(err))
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer func() { _ = db.Close() }()

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}
	logger.Info("connected to database")

	// Run migrations
	if err := runMigrations(db); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}
	logger.Info("migrations completed")

	// Initialize OpenTelemetry tracer with timeout
	tp, err := initTracer(cfg.OTELEndpoint)
	if err != nil {
		logger.Warn("failed to initialize tracer, continuing without tracing", zap.Error(err))
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tp.Shutdown(shutdownCtx); err != nil {
				logger.Error("failed to shutdown tracer", zap.Error(err))
			}
		}()
	}

	// Initialize components
	grinexClient := client.NewGrinexClient(cfg.GrinexAPIURL, cfg.GrinexMarket, logger)
	repo := repository.NewRatesRepository(db, logger)
	svc := service.NewRatesService(grinexClient, repo, logger)

	// Start metrics server
	metricsSrv, metricsErr := server.StartMetricsServer(cfg.MetricsPort, logger)

	// Start gRPC server
	grpcServer := server.NewGRPCServer(svc, cfg.GRPCPort, logger)

	// Graceful shutdown with timeout
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))

		// Stop gRPC with timeout to avoid deadlock on hanging requests
		done := make(chan struct{})
		go func() {
			grpcServer.Stop()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("gRPC server stopped gracefully")
		case <-time.After(shutdownTimeout):
			logger.Warn("graceful shutdown timed out, forcing stop")
			grpcServer.ForceStop()
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics server shutdown failed", zap.Error(err))
		}
	}()

	// Check if metrics server started successfully
	select {
	case err := <-metricsErr:
		logger.Fatal("metrics server failed to start", zap.Error(err))
	case <-time.After(100 * time.Millisecond):
		// Metrics server started OK
	}

	if err := grpcServer.Start(); err != nil {
		logger.Fatal("gRPC server failed", zap.Error(err))
	}
}

// runMigrations applies database migrations
func runMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// initTracer sets up the OpenTelemetry trace provider
func initTracer(endpoint string) (*sdktrace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("usdt-rates"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}
