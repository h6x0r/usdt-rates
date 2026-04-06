// Package service implements core business logic for USDT rate operations
package service

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"usdt-rates/internal/calculator"
	"usdt-rates/internal/client"
)

const requestTimeout = 30 * time.Second

// RateResult holds the calculated rate response
type RateResult struct {
	Ask       float64
	Bid       float64
	Timestamp time.Time
}

// OrderBookProvider defines the interface for fetching order book data
type OrderBookProvider interface {
	GetOrderBook(ctx context.Context) (*client.OrderBook, error)
}

// RateStore defines the interface for persisting rates
type RateStore interface {
	SaveRate(ctx context.Context, ask, bid float64, method string, ts time.Time) (int64, error)
	Ping(ctx context.Context) error
}

// RatesService handles USDT rate operations
type RatesService struct {
	client OrderBookProvider
	repo   RateStore
	logger *zap.Logger
	tracer trace.Tracer
}

// NewRatesService creates a new rates service
func NewRatesService(client OrderBookProvider, repo RateStore, logger *zap.Logger) *RatesService {
	return &RatesService{
		client: client,
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("rates-service"),
	}
}

// GetRates fetches rates from Grinex, calculates the result, and stores it in the database
func (s *RatesService) GetRates(ctx context.Context, method string, n, m int) (*RateResult, error) {
	// Only apply service timeout if the incoming context has no shorter deadline
	if deadline, ok := ctx.Deadline(); !ok || deadline.After(time.Now().Add(requestTimeout)) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestTimeout)
		defer cancel()
	}

	ctx, span := s.tracer.Start(ctx, "RatesService.GetRates")
	defer span.End()

	span.SetAttributes(
		attribute.String("method", method),
		attribute.Int("n", n),
		attribute.Int("m", m),
	)

	// Fetch order book from Grinex
	orderBook, err := s.client.GetOrderBook(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get order book: %w", err)
	}

	if len(orderBook.Asks) == 0 || len(orderBook.Bids) == 0 {
		return nil, fmt.Errorf("empty order book: asks=%d, bids=%d", len(orderBook.Asks), len(orderBook.Bids))
	}

	// Extract prices
	askPrices := make([]float64, len(orderBook.Asks))
	for i, entry := range orderBook.Asks {
		askPrices[i] = entry.Price
	}

	bidPrices := make([]float64, len(orderBook.Bids))
	for i, entry := range orderBook.Bids {
		bidPrices[i] = entry.Price
	}

	// Calculate rates based on the method
	var ask, bid float64

	switch method {
	case "topN":
		ask, err = calculator.TopN(askPrices, n)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate ask topN: %w", err)
		}
		bid, err = calculator.TopN(bidPrices, n)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate bid topN: %w", err)
		}
	case "avgNM":
		ask, err = calculator.AvgNM(askPrices, n, m)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate ask avgNM: %w", err)
		}
		bid, err = calculator.AvgNM(bidPrices, n, m)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate bid avgNM: %w", err)
		}
	default:
		return nil, calculator.ErrInvalidMethod
	}

	now := time.Now().UTC()

	// Store the rate in the database
	_, err = s.repo.SaveRate(ctx, ask, bid, method, now)
	if err != nil {
		return nil, fmt.Errorf("failed to save rate: %w", err)
	}

	return &RateResult{
		Ask:       ask,
		Bid:       bid,
		Timestamp: now,
	}, nil
}

// HealthCheck verifies that the service and its dependencies are operational
func (s *RatesService) HealthCheck(ctx context.Context) error {
	return s.repo.Ping(ctx)
}
