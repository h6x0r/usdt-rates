# USDT Rates gRPC Service

gRPC service for fetching USDT exchange rates from the Grinex exchange. The service retrieves order book data, calculates ask/bid prices using configurable methods, and stores results in PostgreSQL.

## Features

- **gRPC API** — `GetRates` and `HealthCheck` methods
- **Grinex integration** — fetches spot rates via HTTP (resty)
- **Rate calculation** — `topN` (specific position) and `avgNM` (average over range)
- **PostgreSQL storage** — every rate request is persisted with timestamp
- **Database migrations** — automatic schema management via golang-migrate
- **OpenTelemetry tracing** — distributed request tracing
- **Prometheus metrics** — request counters and latency histograms
- **Structured logging** — via zap
- **Graceful shutdown** — handles SIGINT/SIGTERM

## Quick Start

```bash
git clone <repo-url>
cd usdt-rates
make build
docker-compose up -d
```

The `app` service starts automatically via docker-compose. To run manually:

```bash
docker-compose run --rm app
```

## Makefile Commands

| Command              | Description                                  |
|----------------------|----------------------------------------------|
| `make build`         | Build the application binary                 |
| `make test`          | Run unit tests with race detector + coverage |
| `make docker-build`  | Build Docker image                           |
| `make run`           | Build and run the application                |
| `make lint`          | Run golangci-lint                            |
| `make proto`         | Regenerate protobuf Go code                  |
| `make clean`         | Remove build artifacts                       |
| `make fmt`           | Format Go source code                        |

## Configuration

All parameters can be set via **command-line flags** or **environment variables**. Environment variables take precedence over flags.

| Flag               | Env Variable                   | Default                    | Description                    |
|--------------------|--------------------------------|----------------------------|--------------------------------|
| `--grpc-port`      | `GRPC_PORT`                    | `50051`                    | gRPC server port               |
| `--metrics-port`   | `METRICS_PORT`                 | `9090`                     | Prometheus metrics port        |
| `--db-host`        | `DB_HOST`                      | `localhost`                | PostgreSQL host                |
| `--db-port`        | `DB_PORT`                      | `5432`                     | PostgreSQL port                |
| `--db-user`        | `DB_USER`                      | `postgres`                 | PostgreSQL user                |
| `--db-password`    | `DB_PASSWORD`                  | `postgres`                 | PostgreSQL password            |
| `--db-name`        | `DB_NAME`                      | `usdt_rates`               | PostgreSQL database name       |
| `--db-sslmode`     | `DB_SSLMODE`                   | `disable`                  | PostgreSQL SSL mode            |
| `--grinex-api-url` | `GRINEX_API_URL`               | `https://api.grinex.io`   | Grinex API base URL            |
| `--grinex-market`  | `GRINEX_MARKET`                | `usdtrub`                  | Market pair                    |
| `--otel-endpoint`  | `OTEL_EXPORTER_OTLP_ENDPOINT`  | `localhost:4317`           | OpenTelemetry collector        |

### Example

```bash
# Using flags
./app --db-host=mydb.example.com --db-port=5433 --grpc-port=8080

# Using environment variables
DB_HOST=mydb.example.com DB_PORT=5433 GRPC_PORT=8080 ./app
```

## gRPC API

### GetRates

Fetches USDT rates from Grinex using the specified calculation method.

**Request:**
```protobuf
message GetRatesRequest {
  string method = 1;  // "topN" or "avgNM"
  int32 n = 2;        // Position index (0-based)
  int32 m = 3;        // End index for avgNM
}
```

**Response:**
```protobuf
message GetRatesResponse {
  string ask = 1;
  string bid = 2;
  google.protobuf.Timestamp timestamp = 3;
}
```

**Methods:**
- `topN` — returns the price at position N from the order book
- `avgNM` — returns the average price from position N to M (inclusive)

### HealthCheck

Returns `"SERVING"` if the service and database are operational, `"NOT_SERVING"` otherwise.

### Example with grpcurl

```bash
# GetRates with topN method (get price at position 0)
grpcurl -plaintext -d '{"method": "topN", "n": 0}' localhost:50051 rates.v1.RatesService/GetRates

# GetRates with avgNM method (average of positions 0-4)
grpcurl -plaintext -d '{"method": "avgNM", "n": 0, "m": 4}' localhost:50051 rates.v1.RatesService/GetRates

# HealthCheck
grpcurl -plaintext localhost:50051 rates.v1.RatesService/HealthCheck
```

## Monitoring

- **Prometheus metrics**: `http://localhost:9090/metrics`
- **Jaeger UI** (when using docker-compose): `http://localhost:16686`

## Project Structure

```
usdt-rates/
├── api/proto/rates/       # Protobuf definitions and generated code
├── cmd/app/               # Application entrypoint
├── internal/
│   ├── calculator/        # topN and avgNM calculation methods
│   ├── client/            # Grinex HTTP client (resty)
│   ├── config/            # Configuration (flags + env vars)
│   ├── repository/        # PostgreSQL repository
│   ├── server/            # gRPC server and metrics
│   └── service/           # Business logic
├── migrations/            # SQL migration files
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── .golangci.yml
```

## Testing

```bash
make test
```

Unit tests cover:
- Calculator methods (topN, avgNM) with edge cases — 100% coverage
- Grinex client (HTTP mock server, parsing, retries) — 87% coverage
- Service layer (mocked dependencies, empty order book, context cancellation) — 90% coverage
- Configuration loading and validation (flags, env overrides, port validation) — 86% coverage
