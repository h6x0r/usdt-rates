// Package repository provides PostgreSQL persistence for USDT rate data
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Rate represents a stored USDT rate record
type Rate struct {
	ID        int64
	Ask       float64
	Bid       float64
	Method    string
	CreatedAt time.Time
}

// RatesRepository handles persistence of USDT rates
type RatesRepository struct {
	db     *sql.DB
	logger *zap.Logger
	tracer trace.Tracer
}

// NewRatesRepository creates a new repository instance
func NewRatesRepository(db *sql.DB, logger *zap.Logger) *RatesRepository {
	return &RatesRepository{
		db:     db,
		logger: logger,
		tracer: otel.Tracer("repository"),
	}
}

// SaveRate stores a rate record in the database
func (r *RatesRepository) SaveRate(ctx context.Context, ask, bid float64, method string, ts time.Time) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "RatesRepository.SaveRate")
	defer span.End()

	span.SetAttributes(
		attribute.Float64("ask", ask),
		attribute.Float64("bid", bid),
		attribute.String("method", method),
	)

	query := `INSERT INTO rates (ask, bid, method, created_at) VALUES ($1, $2, $3, $4) RETURNING id`

	var id int64
	err := r.db.QueryRowContext(ctx, query, ask, bid, method, ts).Scan(&id)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to save rate: %w", err)
	}

	r.logger.Debug("rate saved",
		zap.Int64("id", id),
		zap.Float64("ask", ask),
		zap.Float64("bid", bid),
		zap.String("method", method),
	)

	return id, nil
}

// Ping checks database connectivity
func (r *RatesRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
