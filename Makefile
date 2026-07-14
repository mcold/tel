.PHONY: build run clean lint help

BINARY_NAME=tel
BUILD_DIR=.
SRC_DIR=cmd/tel

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(SRC_DIR)

run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME) -item users

clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf logs/*.log
	@echo "Done"

lint:
	@echo "Running linter..."
	go vet ./...
	@echo "Done"

help:
	@echo "Available targets:"
	@echo "  build  - Build the binary"
	@echo "  run    - Build and run with default item (users)"
	@echo "  clean  - Remove binary and log files"
	@echo "  lint   - Run go vet"
	@echo "  help   - Show this help message"
