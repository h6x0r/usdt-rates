// Package calculator provides rate calculation methods for order book data
package calculator

import (
	"errors"
	"math"
)

// Sentinel errors for calculation operations
var (
	ErrInvalidMethod   = errors.New("invalid calculation method, use 'topN' or 'avgNM'")
	ErrIndexOutOfRange = errors.New("index out of range")
	ErrInvalidRange    = errors.New("invalid range: N must be <= M")
	ErrEmptyPrices     = errors.New("empty prices slice")
)

// TopN returns the element at position n (0-based) from the prices slice
func TopN(prices []float64, n int) (float64, error) {
	if len(prices) == 0 {
		return 0, ErrEmptyPrices
	}
	if n < 0 || n >= len(prices) {
		return 0, ErrIndexOutOfRange
	}
	return prices[n], nil
}

// AvgNM returns the average of elements from index n to m (inclusive, 0-based)
func AvgNM(prices []float64, n, m int) (float64, error) {
	if len(prices) == 0 {
		return 0, ErrEmptyPrices
	}
	if n > m {
		return 0, ErrInvalidRange
	}
	if n < 0 || m >= len(prices) {
		return 0, ErrIndexOutOfRange
	}

	sum := 0.0
	count := 0
	for i := n; i <= m; i++ {
		sum += prices[i]
		count++
	}

	result := sum / float64(count)
	// Round to 8 decimal places to avoid floating point artifacts
	return math.Round(result*1e8) / 1e8, nil
}
