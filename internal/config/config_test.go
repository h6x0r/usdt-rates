package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "testuser",
		DBPassword: "testpass",
		DBName:     "testdb",
		DBSSLMode:  "disable",
	}

	dsn := cfg.DSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=testuser")
	assert.Contains(t, dsn, "password='testpass'")
	assert.Contains(t, dsn, "dbname=testdb")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestDSN_SpecialCharsInPassword(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "user",
		DBPassword: "p@ss'word",
		DBName:     "db",
		DBSSLMode:  "disable",
	}

	dsn := cfg.DSN()
	assert.Contains(t, dsn, "password='p@ss''word'")
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		GRPCPort:     "50051",
		MetricsPort:  "9090",
		DBHost:       "localhost",
		DBPort:       "5432",
		DBUser:       "postgres",
		DBPassword:   "postgres",
		DBName:       "testdb",
		DBSSLMode:    "disable",
		GrinexAPIURL: "https://api.grinex.io",
		GrinexMarket: "usdtrub",
		OTELEndpoint: "localhost:4317",
	}

	err := cfg.validate()
	require.NoError(t, err)
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		GRPCPort:     "invalid",
		MetricsPort:  "9090",
		DBHost:       "localhost",
		DBPort:       "5432",
		DBUser:       "postgres",
		DBPassword:   "postgres",
		DBName:       "testdb",
		DBSSLMode:    "disable",
		GrinexAPIURL: "https://api.grinex.io",
		GrinexMarket: "usdtrub",
	}

	err := cfg.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grpc-port must be a valid number")
}

func TestValidate_PortOutOfRange(t *testing.T) {
	cfg := &Config{
		GRPCPort:     "99999",
		MetricsPort:  "9090",
		DBHost:       "localhost",
		DBPort:       "5432",
		DBUser:       "postgres",
		DBPassword:   "postgres",
		DBName:       "testdb",
		DBSSLMode:    "disable",
		GrinexAPIURL: "https://api.grinex.io",
		GrinexMarket: "usdtrub",
	}

	err := cfg.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be between 1 and 65535")
}

func TestValidate_EmptyDBHost(t *testing.T) {
	cfg := &Config{
		GRPCPort:     "50051",
		MetricsPort:  "9090",
		DBHost:       "",
		DBPort:       "5432",
		DBUser:       "postgres",
		DBPassword:   "postgres",
		DBName:       "testdb",
		DBSSLMode:    "disable",
		GrinexAPIURL: "https://api.grinex.io",
		GrinexMarket: "usdtrub",
	}

	err := cfg.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db-host must not be empty")
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "50051", cfg.GRPCPort)
	assert.Equal(t, "localhost", cfg.DBHost)
	assert.Equal(t, "5432", cfg.DBPort)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("DB_HOST", "envhost")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("GRPC_PORT", "8080")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "envhost", cfg.DBHost)
	assert.Equal(t, "6543", cfg.DBPort)
	assert.Equal(t, "8080", cfg.GRPCPort)
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("GRPC_PORT", "abc")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grpc-port must be a valid number")
}

func TestResolve(t *testing.T) {
	// Env takes priority over flag
	t.Setenv("TEST_KEY", "from_env")
	val := resolve("from_flag", "TEST_KEY", "default")
	assert.Equal(t, "from_env", val)

	// Flag takes priority over default when env not set
	t.Setenv("TEST_KEY2", "")
	val = resolve("from_flag", "TEST_KEY2", "default")
	assert.Equal(t, "from_flag", val)

	// Default when neither flag nor env
	val = resolve("default", "NONEXISTENT_KEY", "default")
	assert.Equal(t, "default", val)
}
