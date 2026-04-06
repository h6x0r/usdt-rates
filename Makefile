.PHONY: build test docker-build run lint proto clean fmt

APP_NAME := app
BUILD_DIR := ./cmd/app

build:
	go build -o $(APP_NAME) $(BUILD_DIR)

test:
	go test -v -race -count=1 -cover ./...

docker-build:
	docker build -t usdt-rates:latest .

run: build
	./$(APP_NAME)

lint:
	golangci-lint run ./...

proto:
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/rates/rates.proto

clean:
	rm -f $(APP_NAME)
	go clean ./...

fmt:
	go fmt ./...
	goimports -w .
