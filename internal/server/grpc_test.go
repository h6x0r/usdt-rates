package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	pb "usdt-rates/api/proto/rates"
	"usdt-rates/internal/calculator"
	"usdt-rates/internal/client"
	"usdt-rates/internal/service"
)

// mockProvider implements service.OrderBookProvider for testing
type mockProvider struct {
	ob  *client.OrderBook
	err error
}

func (m *mockProvider) GetOrderBook(_ context.Context) (*client.OrderBook, error) {
	return m.ob, m.err
}

// mockStore implements service.RateStore for testing
type mockStore struct {
	saveErr error
	pingErr error
}

func (m *mockStore) SaveRate(_ context.Context, _, _ float64, _ string, _ time.Time) (int64, error) {
	return 1, m.saveErr
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.pingErr
}

func newTestServer(provider service.OrderBookProvider, store service.RateStore) *GRPCServer {
	svc := service.NewRatesService(provider, store, zap.NewNop())
	return NewGRPCServer(svc, "0", zap.NewNop())
}

// --- toGRPCError tests ---

func TestToGRPCError_InvalidMethod(t *testing.T) {
	err := toGRPCError(calculator.ErrInvalidMethod)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
}

func TestToGRPCError_IndexOutOfRange(t *testing.T) {
	err := toGRPCError(calculator.ErrIndexOutOfRange)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
}

func TestToGRPCError_InvalidRange(t *testing.T) {
	err := toGRPCError(calculator.ErrInvalidRange)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
}

func TestToGRPCError_EmptyPrices(t *testing.T) {
	err := toGRPCError(calculator.ErrEmptyPrices)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
}

func TestToGRPCError_DeadlineExceeded(t *testing.T) {
	err := toGRPCError(context.DeadlineExceeded)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DeadlineExceeded")
}

func TestToGRPCError_Canceled(t *testing.T) {
	err := toGRPCError(context.Canceled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Canceled")
}

func TestToGRPCError_WrappedDeadline(t *testing.T) {
	wrapped := errors.New("failed to save rate: pq: deadline exceeded")
	err := toGRPCError(wrapped)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DeadlineExceeded")
}

func TestToGRPCError_WrappedContextCanceled(t *testing.T) {
	wrapped := errors.New("failed: context canceled by client")
	err := toGRPCError(wrapped)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Canceled")
}

func TestToGRPCError_FetchOrderBook(t *testing.T) {
	err := toGRPCError(errors.New("failed to fetch order book: connection refused"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unavailable")
}

func TestToGRPCError_EmptyOrderBook(t *testing.T) {
	err := toGRPCError(errors.New("empty order book: asks=0, bids=0"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unavailable")
}

func TestToGRPCError_InternalError(t *testing.T) {
	err := toGRPCError(errors.New("something unexpected"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Internal")
}

// --- GetRates RPC tests ---

func TestGetRates_Success(t *testing.T) {
	provider := &mockProvider{ob: &client.OrderBook{
		Asks: []client.OrderBookEntry{{Price: 95.5, Volume: 100}},
		Bids: []client.OrderBookEntry{{Price: 95.4, Volume: 50}},
	}}
	store := &mockStore{}
	srv := newTestServer(provider, store)

	resp, err := srv.GetRates(context.Background(), &pb.GetRatesRequest{
		Method: "topN", N: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, "95.50000000", resp.Ask)
	assert.Equal(t, "95.40000000", resp.Bid)
	assert.NotNil(t, resp.Timestamp)
}

func TestGetRates_InvalidMethod(t *testing.T) {
	srv := newTestServer(&mockProvider{}, &mockStore{})

	_, err := srv.GetRates(context.Background(), &pb.GetRatesRequest{
		Method: "bad",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
}

func TestGetRates_NegativeN(t *testing.T) {
	srv := newTestServer(&mockProvider{}, &mockStore{})

	_, err := srv.GetRates(context.Background(), &pb.GetRatesRequest{
		Method: "topN", N: -1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "N must be >= 0")
}

func TestGetRates_NegativeM(t *testing.T) {
	srv := newTestServer(&mockProvider{}, &mockStore{})

	_, err := srv.GetRates(context.Background(), &pb.GetRatesRequest{
		Method: "avgNM", N: 0, M: -1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "M must be >= 0")
}

func TestGetRates_EmptyMethod(t *testing.T) {
	srv := newTestServer(&mockProvider{}, &mockStore{})

	_, err := srv.GetRates(context.Background(), &pb.GetRatesRequest{
		Method: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
}

// --- HealthCheck RPC tests ---

func TestHealthCheck_Serving(t *testing.T) {
	store := &mockStore{}
	srv := newTestServer(nil, store)

	resp, err := srv.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, "SERVING", resp.Status)
}

func TestHealthCheck_NotServing(t *testing.T) {
	store := &mockStore{pingErr: errors.New("db down")}
	srv := newTestServer(nil, store)

	resp, err := srv.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, "NOT_SERVING", resp.Status)
}
