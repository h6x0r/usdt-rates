package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	// RatesRequests counts total gRPC requests
	RatesRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usdt_rates_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	// RatesLatency tracks gRPC request duration
	RatesLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usdt_rates_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method"},
	)
)

// StartMetricsServer starts the Prometheus metrics HTTP server.
// Returns the server and an error channel that signals startup failures.
func StartMetricsServer(port string, logger *zap.Logger) (*http.Server, <-chan error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		logger.Info("metrics server starting", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server failed", zap.Error(err))
			errCh <- err
		}
	}()

	return srv, errCh
}
