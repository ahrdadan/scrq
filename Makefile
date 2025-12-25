# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=server
BINARY_UNIX=$(BINARY_NAME)_linux

# Build info
VERSION ?= 1
LDFLAGS=-ldflags "-X github.com/ahrdadan/scrq/internal/config.Version=$(VERSION)"

# Main package path
MAIN_PATH=./cmd/server

.PHONY: all build clean test coverage deps lint run docker-build docker-run help

all: test build

## build: Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v $(MAIN_PATH)

## build-linux: Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) -v $(MAIN_PATH)

## build-all: Build for all platforms
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/scrq-linux-amd64 -v $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/scrq-linux-arm64 -v $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/scrq-darwin-amd64 -v $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/scrq-darwin-arm64 -v $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/scrq-windows-amd64.exe -v $(MAIN_PATH)

## clean: Remove build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f coverage.out
	rm -f coverage.html
	rm -rf dist/

## test: Run tests
test:
	$(GOTEST) -v ./...

## test-short: Run short tests
test-short:
	$(GOTEST) -short -v ./...

## coverage: Run tests with coverage
coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## lint: Run linter
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## fmt: Format code
fmt:
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	$(GOCMD) vet ./...

## run: Run the application
run: build
	./$(BINARY_NAME)

## run-dev: Run with go run
run-dev:
	$(GOCMD) run $(MAIN_PATH)

## docker-build: Build Docker image
docker-build:
	docker build -t scrq:$(VERSION) .

## docker-run: Run Docker container
docker-run:
	docker run -p 8000:8000 scrq:$(VERSION)

## docker-compose-up: Start with Docker Compose
docker-compose-up:
	docker-compose up -d

## docker-compose-down: Stop Docker Compose
docker-compose-down:
	docker-compose down

## docker-compose-chrome: Start with Chrome support
docker-compose-chrome:
	docker-compose --profile chrome up -d

## help: Show this help message
help:
	@echo "Scrq Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'

