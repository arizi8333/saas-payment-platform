.PHONY: build run test test-coverage swagger migrate-up migrate-down migrate-status seed clean

APP_NAME := server
BUILD_DIR := ./bin
CMD_DIR := ./cmd/server
MIGRATION_DIR := ./migrations

# Build-time variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

# Build the Go application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	$(BUILD_DIR)/$(APP_NAME)

# Run all unit tests with coverage
test:
	@echo "Running tests..."
	@mkdir -p ../reports/backend
	go test -v -race -coverprofile=../reports/backend/coverage.out ./...
	@echo "Tests complete."

# Generate HTML coverage report
test-coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=../reports/backend/coverage.out -o ../reports/backend/coverage.html
	go tool cover -func=../reports/backend/coverage.out > ../reports/backend/coverage-summary.txt
	@echo "Coverage report: ../reports/backend/coverage.html"
	@echo "Coverage summary: ../reports/backend/coverage-summary.txt"

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger docs..."
	swag init -g $(CMD_DIR)/main.go -o ./docs
	@echo "Swagger docs generated in ./docs"

# Run database migrations up
migrate-up:
	@echo "Running migrations up..."
	go run ./cmd/migrate up
	@echo "Migrations applied."

# Run database migrations down
migrate-down:
	@echo "Running migrations down..."
	go run ./cmd/migrate down 1
	@echo "Migration rolled back."

# Show migration status
migrate-status:
	@echo "Checking migration status..."
	go run ./cmd/migrate status

# Seed development data (idempotent — safe to run multiple times)
seed:
	@echo "Seeding development data..."
	go run ./cmd/seed
	@echo "Seed complete."

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf ../reports/backend
	@echo "Clean complete."
