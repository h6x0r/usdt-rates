// Package config handles application configuration from flags and environment variables
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	GRPCPort    string
	MetricsPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	GrinexAPIURL string
	GrinexMarket string

	OTELEndpoint string
}

// Load reads configuration from command-line flags and environment variables.
// Priority: env vars > flags > defaults.
func Load() (*Config, error) {
	fs := flag.NewFlagSet("usdt-rates", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	grpcPort := fs.String("grpc-port", "50051", "gRPC server port")
	metricsPort := fs.String("metrics-port", "9090", "Prometheus metrics port")
	dbHost := fs.String("db-host", "localhost", "PostgreSQL host")
	dbPort := fs.String("db-port", "5432", "PostgreSQL port")
	dbUser := fs.String("db-user", "postgres", "PostgreSQL user")
	dbPassword := fs.String("db-password", "postgres", "PostgreSQL password")
	dbName := fs.String("db-name", "usdt_rates", "PostgreSQL database name")
	dbSSLMode := fs.String("db-sslmode", "disable", "PostgreSQL SSL mode")
	grinexAPIURL := fs.String("grinex-api-url", "https://api.grinex.io", "Grinex API base URL")
	grinexMarket := fs.String("grinex-market", "usdtrub", "Grinex market pair")
	otelEndpoint := fs.String("otel-endpoint", "localhost:4317", "OpenTelemetry collector endpoint")

	// Parse flags; ignore errors from unknown flags in test binaries
	_ = fs.Parse(os.Args[1:])

	// Build config: flags override defaults, env vars override flags
	cfg := &Config{
		GRPCPort:     resolve(*grpcPort, "GRPC_PORT", "50051"),
		MetricsPort:  resolve(*metricsPort, "METRICS_PORT", "9090"),
		DBHost:       resolve(*dbHost, "DB_HOST", "localhost"),
		DBPort:       resolve(*dbPort, "DB_PORT", "5432"),
		DBUser:       resolve(*dbUser, "DB_USER", "postgres"),
		DBPassword:   resolve(*dbPassword, "DB_PASSWORD", "postgres"),
		DBName:       resolve(*dbName, "DB_NAME", "usdt_rates"),
		DBSSLMode:    resolve(*dbSSLMode, "DB_SSLMODE", "disable"),
		GrinexAPIURL: resolve(*grinexAPIURL, "GRINEX_API_URL", "https://api.grinex.io"),
		GrinexMarket: resolve(*grinexMarket, "GRINEX_MARKET", "usdtrub"),
		OTELEndpoint: resolve(*otelEndpoint, "OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// resolve returns the value using priority: env var > flag value > default
func resolve(flagVal, envKey, defaultVal string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if flagVal != defaultVal {
		return flagVal
	}
	return defaultVal
}

// validate checks that all configuration values are valid
func (c *Config) validate() error {
	if err := validatePort(c.GRPCPort, "grpc-port"); err != nil {
		return err
	}
	if err := validatePort(c.MetricsPort, "metrics-port"); err != nil {
		return err
	}
	if err := validatePort(c.DBPort, "db-port"); err != nil {
		return err
	}
	if c.DBHost == "" {
		return fmt.Errorf("db-host must not be empty")
	}
	if c.DBUser == "" {
		return fmt.Errorf("db-user must not be empty")
	}
	if c.DBName == "" {
		return fmt.Errorf("db-name must not be empty")
	}
	if c.GrinexAPIURL == "" {
		return fmt.Errorf("grinex-api-url must not be empty")
	}
	if c.GrinexMarket == "" {
		return fmt.Errorf("grinex-market must not be empty")
	}
	if c.OTELEndpoint == "" {
		return fmt.Errorf("otel-endpoint must not be empty")
	}
	return nil
}

func validatePort(port, name string) error {
	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("%s must be a valid number: %w", name, err)
	}
	if p < 1 || p > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", name, p)
	}
	return nil
}

// DSN returns PostgreSQL connection string in lib/pq key=value format.
// Single quotes in password are escaped by doubling them per libpq convention.
func (c *Config) DSN() string {
	escapedPassword := strings.ReplaceAll(c.DBPassword, "'", "''")
	return fmt.Sprintf("host=%s port=%s user=%s password='%s' dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, escapedPassword, c.DBName, c.DBSSLMode)
}
