.PHONY: all build clean collect store detect server help

# Binary names (in current directory)
COLLECT_BIN=collect
STORE_BIN=store
DETECT_BIN=detect
SERVER_BIN=server

# Install location
INSTALL_DIR?=/usr/local/bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

all: build

## build: Build all executables
build: collect store detect server

## collect: Build the collect service
collect:
	@echo "Building collect..."
	$(GOBUILD) -o $(COLLECT_BIN) ./cmd/collect

## store: Build the store service
store:
	@echo "Building store..."
	$(GOBUILD) -o $(STORE_BIN) ./cmd/store

## detect: Build the detect service
detect:
	@echo "Building detect..."
	$(GOBUILD) -o $(DETECT_BIN) ./cmd/detect

## server: Build the server service
server:
	@echo "Building server..."
	$(GOBUILD) -o $(SERVER_BIN) ./cmd/server

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(COLLECT_BIN) $(STORE_BIN) $(DETECT_BIN) $(SERVER_BIN)
	rm -f metrics.csv

## test: Run tests
test:
	$(GOTEST) -v ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
