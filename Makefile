.PHONY: all build test test-coverage test-race benchmark benchmark-memory lint vet fmt fmt-check docs clean help

# Variables
GOPATH := $(shell go env GOPATH)
GOBIN := $(GOPATH)/bin
GOLANGCI_LINT := $(GOBIN)/golangci-lint
COVERAGE_FILE := output/coverage.out
COVERAGE_HTML := output/coverage.html
BENCHMARK_RESULTS := output/benchmarks/results/benchmark_$(shell date +%Y%m%d_%H%M%S).txt

# Default target
all: fmt vet test build

# Build the library and CLI
build:
	@echo "Building library..."
	@go build -v ./...
	@echo "Building CLI..."
	@go build -v -o bin/ipaggregator ./cmd/ipaggregator

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	@echo "Coverage summary:"
	@go tool cover -func=$(COVERAGE_FILE) | tail -1

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -v -race ./...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	@mkdir -p benchmarks/results
	@go test -bench=. -benchmem -run=^$$ ./... | tee $(BENCHMARK_RESULTS)
	@echo "Benchmark results saved to: $(BENCHMARK_RESULTS)"

# Run memory benchmarks
benchmark-memory:
	@echo "Running memory benchmarks..."
	@mkdir -p benchmarks/results
	@go test -bench=. -benchmem -memprofile=benchmarks/results/mem.prof -run=^$$ ./...
	@echo "Memory profile saved to: benchmarks/results/mem.prof"

# Install golangci-lint if not present
$(GOLANGCI_LINT):
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linting
lint: $(GOLANGCI_LINT)
	@echo "Running golangci-lint..."
	@$(GOLANGCI_LINT) run ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Check if code is formatted
fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$(go fmt ./... 2>&1)" || (echo "Code needs formatting. Run 'make fmt'" && exit 1)

# Generate documentation
docs:
	@echo "Generating documentation..."
	@go doc -all > docs/godoc.txt
	@echo "Documentation generated in docs/"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@rm -rf bin/
	@rm -f output/benchmarks/results/*.prof
	@go clean -cache

# Show help
help:
	@echo "NetJugo Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make              - Format, vet, test, and build"
	@echo "  make build        - Build the library and CLI"
	@echo "  make test         - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make test-race    - Run tests with race detector"
	@echo "  make benchmark    - Run benchmarks"
	@echo "  make benchmark-memory - Run memory benchmarks"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make vet          - Run go vet"
	@echo "  make fmt          - Format code"
	@echo "  make fmt-check    - Check if code is formatted"
	@echo "  make docs         - Generate documentation"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make help         - Show this help message"