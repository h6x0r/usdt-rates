package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"usdt-rates/internal/calculator"
	"usdt-rates/internal/client"
)

// mockOrderBookProvider is a test double for OrderBookProvider
type mockOrderBookProvider struct {
	orderBook *client.OrderBook
	err       error
}

func (m *mockOrderBookProvider) GetOrderBook(_ context.Context) (*client.OrderBook, error) {
	return m.orderBook, m.err
}

// mockRateStore is a test double for RateStore
type mockRateStore struct {
	savedAsk    float64
	savedBid    float64
	savedMethod string
	saveErr     error
	pingErr     error
}

func (m *mockRateStore) SaveRate(_ context.Context, ask, bid float64, method string, _ time.Time) (int64, error) {
	m.savedAsk = ask
	m.savedBid = bid
	m.savedMethod = method
	return 1, m.saveErr
}

func (m *mockRateStore) Ping(_ context.Context) error {
	return m.pingErr
}

func TestGetRates_TopN(t *testing.T) {
	ob := &client.OrderBook{
		Asks: []client.OrderBookEntry{
			{Price: 95.50, Volume: 100},
			{Price: 95.60, Volume: 200},
			{Price: 95.70, Volume: 150},
		},
		Bids: []client.OrderBookEntry{
			{Price: 95.40, Volume: 50},
			{Price: 95.30, Volume: 120},
			{Price: 95.20, Volume: 80},
		},
	}

	provider := &mockOrderBookProvider{orderBook: ob}
	store := &mockRateStore{}
	svc := NewRatesService(provider, store, zap.NewNop())

	result, err := svc.GetRates(context.Background(), "topN", 1, 0)
	require.NoError(t, err)

	assert.Equal(t, 95.60, result.Ask)
	assert.Equal(t, 95.30, result.Bid)
	assert.Equal(t, 95.60, store.savedAsk)
	assert.Equal(t, 95.30, store.savedBid)
	assert.Equal(t, "topN", store.savedMethod)
	assert.False(t, result.Timestamp.IsZero())
}

func TestGetRates_AvgNM(t *testing.T) {
	ob := &client.OrderBook{
		Asks: []client.OrderBookEntry{
			{Price: 10.0, Volume: 100},
			{Price: 20.0, Volume: 200},
			{Price: 30.0, Volume: 150},
		},
		Bids: []client.OrderBookEntry{
			{Price: 40.0, Volume: 50},
			{Price: 50.0, Volume: 120},
			{Price: 60.0, Volume: 80},
		},
	}

	provider := &mockOrderBookProvider{orderBook: ob}
	store := &mockRateStore{}
	svc := NewRatesService(provider, store, zap.NewNop())

	result, err := svc.GetRates(context.Background(), "avgNM", 0, 2)
	require.NoError(t, err)

	assert.Equal(t, 20.0, result.Ask)
	assert.Equal(t, 50.0, result.Bid)
}

func TestGetRates_InvalidMethod(t *testing.T) {
	ob := &client.OrderBook{
		Asks: []client.OrderBookEntry{{Price: 95.50}},
		Bids: []client.OrderBookEntry{{Price: 95.40}},
	}

	provider := &mockOrderBookProvider{orderBook: ob}
	store := &mockRateStore{}
	svc := NewRatesService(provider, store, zap.NewNop())

	_, err := svc.GetRates(context.Background(), "invalid", 0, 0)
	require.ErrorIs(t, err, calculator.ErrInvalidMethod)
}

func TestGetRates_ClientError(t *testing.T) {
	provider := &mockOrderBookProvider{err: errors.New("connection refused")}
	store := &mockRateStore{}
	svc := NewRatesService(provider, store, zap.NewNop())

	_, err := svc.GetRates(context.Background(), "topN", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get order book")
}

func TestGetRates_StoreError(t *testing.T) {
	ob := &client.OrderBook{
		Asks: []client.OrderBookEntry{{Price: 95.50}},
		Bids: []client.OrderBookEntry{{Price: 95.40}},
	}

	provider := &mockOrderBookProvider{orderBook: ob}
	store := &mockRateStore{saveErr: errors.New("db connection lost")}
	svc := NewRatesService(provider, store, zap.NewNop())

	_, err := svc.GetRates(context.Background(), "topN", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save rate")
}

func TestGetRates_EmptyOrderBook(t *testing.T) {
	ob := &client.OrderBook{
		Asks: []client.OrderBookEntry{},
		Bids: []client.OrderBookEntry{},
	}

	provider := &mockOrderBookProvider{orderBook: ob}
	store := &mockRateStore{}
	svc := NewRatesService(provider, store, zap.NewNop())

	_, err := svc.GetRates(context.Background(), "topN", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty order book")
}

func TestGetRates_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := &mockOrderBookProvider{err: context.Canceled}
	store := &mockRateStore{}
	svc := NewRatesService(provider, store, zap.NewNop())

	_, err := svc.GetRates(ctx, "topN", 0, 0)
	require.Error(t, err)
}

func TestHealthCheck_OK(t *testing.T) {
	store := &mockRateStore{}
	svc := NewRatesService(nil, store, zap.NewNop())

	err := svc.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestHealthCheck_DBDown(t *testing.T) {
	store := &mockRateStore{pingErr: errors.New("db unreachable")}
	svc := NewRatesService(nil, store, zap.NewNop())

	err := svc.HealthCheck(context.Background())
	require.Error(t, err)
}
