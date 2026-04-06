# Build stage
FROM golang:1.26.1-alpine3.21 AS builder

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/app ./cmd/app

# Final stage
FROM alpine:3.21.3

RUN apk add --no-cache ca-certificates && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/app .
COPY --from=builder /build/migrations ./migrations

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 50051 9090

CMD ["./app"]
