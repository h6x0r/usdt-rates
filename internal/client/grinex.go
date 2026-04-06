// Package client provides HTTP clients for external exchange APIs
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

const (
	httpTimeout       = 30 * time.Second
	maxRetries        = 3
	retryWaitTime     = 500 * time.Millisecond
	retryMaxWaitTime  = 5 * time.Second
	maxResponseLogLen = 500
)

// OrderBookEntry represents a single order in the order book
type OrderBookEntry struct {
	Price  float64
	Volume float64
}

// OrderBook holds asks and bids from the exchange
type OrderBook struct {
	Asks []OrderBookEntry
	Bids []OrderBookEntry
}

// GrinexClient fetches market data from Grinex exchange
type GrinexClient struct {
	client  *resty.Client
	baseURL string
	market  string
	logger  *zap.Logger
}

// orderBookResponse represents the raw API response.
// Supports both array format [[price, volume], ...] and object format [{"price":"...", "volume":"..."}, ...].
type orderBookResponse struct {
	Asks []json.RawMessage `json:"asks"`
	Bids []json.RawMessage `json:"bids"`
}

// orderBookEntryObject represents a Peatio-style object entry
type orderBookEntryObject struct {
	Price  interface{} `json:"price"`
	Volume interface{} `json:"volume"`
}

// NewGrinexClient creates a new Grinex API client
func NewGrinexClient(baseURL, market string, logger *zap.Logger) *GrinexClient {
	c := resty.New().
		SetTimeout(httpTimeout).
		SetRetryCount(maxRetries).
		SetRetryWaitTime(retryWaitTime).
		SetRetryMaxWaitTime(retryMaxWaitTime).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			// Retry on server errors and rate limiting
			return r.StatusCode() >= 500 || r.StatusCode() == 429
		})

	return &GrinexClient{
		client:  c,
		baseURL: baseURL,
		market:  market,
		logger:  logger,
	}
}

// GetOrderBook fetches the current order book for the configured market
func (g *GrinexClient) GetOrderBook(ctx context.Context) (*OrderBook, error) {
	tracer := otel.Tracer("grinex-client")
	ctx, span := tracer.Start(ctx, "GrinexClient.GetOrderBook")
	defer span.End()

	span.SetAttributes(
		attribute.String("market", g.market),
		attribute.String("base_url", g.baseURL),
	)

	url := fmt.Sprintf("%s/api/v2/peatio/public/markets/%s/order-book", g.baseURL, g.market)

	g.logger.Debug("fetching order book from Grinex",
		zap.String("url", url),
		zap.String("market", g.market),
	)

	var resp orderBookResponse
	httpResp, err := g.client.R().
		SetContext(ctx).
		SetResult(&resp).
		SetQueryParam("asks_limit", "50").
		SetQueryParam("bids_limit", "50").
		Get(url)

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to fetch order book: %w", err)
	}

	if httpResp.StatusCode() != 200 {
		body := httpResp.String()
		if len(body) > maxResponseLogLen {
			body = body[:maxResponseLogLen] + "..."
		}
		err := fmt.Errorf("unexpected status code: %d, body: %s", httpResp.StatusCode(), body)
		span.RecordError(err)
		return nil, err
	}

	orderBook, err := parseOrderBook(&resp)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	g.logger.Debug("order book fetched successfully",
		zap.Int("asks_count", len(orderBook.Asks)),
		zap.Int("bids_count", len(orderBook.Bids)),
	)

	return orderBook, nil
}

// parseOrderBook converts raw API response to typed OrderBook
func parseOrderBook(resp *orderBookResponse) (*OrderBook, error) {
	ob := &OrderBook{
		Asks: make([]OrderBookEntry, 0, len(resp.Asks)),
		Bids: make([]OrderBookEntry, 0, len(resp.Bids)),
	}

	for _, raw := range resp.Asks {
		entry, err := parseRawEntry(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ask entry: %w", err)
		}
		ob.Asks = append(ob.Asks, entry)
	}

	for _, raw := range resp.Bids {
		entry, err := parseRawEntry(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bid entry: %w", err)
		}
		ob.Bids = append(ob.Bids, entry)
	}

	return ob, nil
}

// parseRawEntry handles both array format [price, volume] and object format {"price":..., "volume":...}
func parseRawEntry(raw json.RawMessage) (OrderBookEntry, error) {
	// Try array format first: [price, volume]
	var arr []interface{}
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) >= 2 {
		return parseArrayEntry(arr)
	}

	// Try object format: {"price":"...", "volume":"..."}
	var obj orderBookEntryObject
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Price != nil {
		price, err := toFloat64(obj.Price)
		if err != nil {
			return OrderBookEntry{}, fmt.Errorf("invalid price: %w", err)
		}
		volume, err := toFloat64(obj.Volume)
		if err != nil {
			return OrderBookEntry{}, fmt.Errorf("invalid volume: %w", err)
		}
		return OrderBookEntry{Price: price, Volume: volume}, nil
	}

	return OrderBookEntry{}, fmt.Errorf("unsupported entry format: %s", string(raw))
}

// parseArrayEntry extracts price and volume from an array entry [price, volume]
func parseArrayEntry(raw []interface{}) (OrderBookEntry, error) {
	if len(raw) < 2 {
		return OrderBookEntry{}, fmt.Errorf("invalid entry: expected at least 2 elements, got %d", len(raw))
	}

	price, err := toFloat64(raw[0])
	if err != nil {
		return OrderBookEntry{}, fmt.Errorf("invalid price: %w", err)
	}

	volume, err := toFloat64(raw[1])
	if err != nil {
		return OrderBookEntry{}, fmt.Errorf("invalid volume: %w", err)
	}

	return OrderBookEntry{Price: price, Volume: volume}, nil
}

// toFloat64 converts various numeric representations to float64
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}
