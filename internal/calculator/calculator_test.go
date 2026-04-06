package calculator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopN(t *testing.T) {
	prices := []float64{10.0, 20.0, 30.0, 40.0, 50.0}

	tests := []struct {
		name    string
		prices  []float64
		n       int
		want    float64
		wantErr error
	}{
		{"first element", prices, 0, 10.0, nil},
		{"middle element", prices, 2, 30.0, nil},
		{"last element", prices, 4, 50.0, nil},
		{"negative index", prices, -1, 0, ErrIndexOutOfRange},
		{"index out of range", prices, 5, 0, ErrIndexOutOfRange},
		{"empty slice", []float64{}, 0, 0, ErrEmptyPrices},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TopN(tt.prices, tt.n)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAvgNM(t *testing.T) {
	prices := []float64{10.0, 20.0, 30.0, 40.0, 50.0}

	tests := []struct {
		name    string
		prices  []float64
		n, m    int
		want    float64
		wantErr error
	}{
		{"full range", prices, 0, 4, 30.0, nil},
		{"single element", prices, 2, 2, 30.0, nil},
		{"partial range", prices, 1, 3, 30.0, nil},
		{"first two", prices, 0, 1, 15.0, nil},
		{"invalid range n>m", prices, 3, 1, 0, ErrInvalidRange},
		{"out of range", prices, 0, 5, 0, ErrIndexOutOfRange},
		{"negative index", prices, -1, 2, 0, ErrIndexOutOfRange},
		{"empty slice", []float64{}, 0, 0, 0, ErrEmptyPrices},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AvgNM(tt.prices, tt.n, tt.m)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAvgNM_Precision(t *testing.T) {
	prices := []float64{10.1, 20.2, 30.3}
	got, err := AvgNM(prices, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, 20.2, got)
}
