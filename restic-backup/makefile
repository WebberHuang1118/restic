# Makefile for restic-backup project

# Name of the binary to build
APP_NAME := restic-backup

# Directory where the final binary will be placed
BIN_DIR := bin

# Directory containing the main package source (main.go)
MAIN_DIR := cmd

# Default target
all: build

# Build the Go binary by explicitly specifying the main source file.
build:
	@echo "=> Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP_NAME) $(MAIN_DIR)/main.go

# Run go mod tidy to ensure dependencies are in sync.
mod-tidy:
	@echo "=> Tidying go.mod..."
	@go mod tidy

# Clean up generated files.
clean:
	@echo "=> Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)

# Run tests (if you have tests to run).
test:
	@echo "=> Running tests..."
	@go test ./...

# Run the binary from the bin directory.
# Example usage: `make run ARGS="--mode=backup --pvc=foo"`
run:
	@echo "=> Running $(APP_NAME) with args: $(ARGS)"
	@$(BIN_DIR)/$(APP_NAME) $(ARGS)

# Show help message for common targets.
help:
	@echo "Common targets:"
	@echo "  make build      - Build the binary"
	@echo "  make run        - Run the binary (pass ARGS=\"...\" for flags)"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make mod-tidy   - Tidy go.mod/go.sum"
	@echo "  make test       - Run tests"

.PHONY: all build mod-tidy clean test run help
