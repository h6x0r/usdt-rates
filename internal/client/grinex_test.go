package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGetOrderBook_Success(t *testing.T) {
	resp := map[string]interface{}{
		"asks": [][]interface{}{
			{"95.50", "100.0"},
			{"95.60", "200.0"},
			{"95.70", "150.0"},
		},
		"bids": [][]interface{}{
			{"95.40", "50.0"},
			{"95.30", "120.0"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v2/peatio/public/markets/usdtrub/order-book")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	logger := zap.NewNop()
	client := NewGrinexClient(srv.URL, "usdtrub", logger)

	ob, err := client.GetOrderBook(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ob)

	assert.Len(t, ob.Asks, 3)
	assert.Len(t, ob.Bids, 2)
	assert.Equal(t, 95.50, ob.Asks[0].Price)
	assert.Equal(t, 100.0, ob.Asks[0].Volume)
	assert.Equal(t, 95.40, ob.Bids[0].Price)
}

func TestGetOrderBook_ObjectFormat(t *testing.T) {
	resp := map[string]interface{}{
		"asks": []map[string]interface{}{
			{"price": "95.50", "volume": "100.0"},
			{"price": "95.60", "volume": "200.0"},
		},
		"bids": []map[string]interface{}{
			{"price": "95.40", "volume": "50.0"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	logger := zap.NewNop()
	client := NewGrinexClient(srv.URL, "usdtrub", logger)

	ob, err := client.GetOrderBook(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ob)

	assert.Len(t, ob.Asks, 2)
	assert.Equal(t, 95.50, ob.Asks[0].Price)
	assert.Equal(t, 95.60, ob.Asks[1].Price)
	assert.Len(t, ob.Bids, 1)
	assert.Equal(t, 95.40, ob.Bids[0].Price)
}

func TestGetOrderBook_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	logger := zap.NewNop()
	client := NewGrinexClient(srv.URL, "usdtrub", logger)

	ob, err := client.GetOrderBook(context.Background())
	require.Error(t, err)
	assert.Nil(t, ob)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestGetOrderBook_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"asks": "invalid"}`))
	}))
	defer srv.Close()

	logger := zap.NewNop()
	client := NewGrinexClient(srv.URL, "usdtrub", logger)

	ob, err := client.GetOrderBook(context.Background())
	require.Error(t, err)
	assert.Nil(t, ob)
}

func TestParseRawEntry_ArrayFormat(t *testing.T) {
	raw := json.RawMessage(`["95.50", "100.0"]`)
	entry, err := parseRawEntry(raw)
	require.NoError(t, err)
	assert.Equal(t, 95.50, entry.Price)
	assert.Equal(t, 100.0, entry.Volume)
}

func TestParseRawEntry_ObjectFormat(t *testing.T) {
	raw := json.RawMessage(`{"price": "95.50", "volume": "100.0"}`)
	entry, err := parseRawEntry(raw)
	require.NoError(t, err)
	assert.Equal(t, 95.50, entry.Price)
	assert.Equal(t, 100.0, entry.Volume)
}

func TestParseRawEntry_InvalidFormat(t *testing.T) {
	raw := json.RawMessage(`"just a string"`)
	_, err := parseRawEntry(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported entry format")
}

func TestParseArrayEntry_InvalidLength(t *testing.T) {
	_, err := parseArrayEntry([]interface{}{"95.50"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected at least 2 elements")
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    float64
		wantErr bool
	}{
		{"float64", 95.5, 95.5, false},
		{"string", "95.5", 95.5, false},
		{"int", 95, 95.0, false},
		{"int64", int64(95), 95.0, false},
		{"unsupported", true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toFloat64(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
