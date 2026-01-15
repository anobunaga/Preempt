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

# Database Migration Commands
DB_USER?=myapp
DB_PASSWORD?=mypassword123
DB_HOST?=localhost
DB_PORT?=3306
DB_NAME?=preempt
MIGRATION_PATH=./migrations
DB_URL=mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_HOST):$(DB_PORT))/$(DB_NAME)

## migrate-up: Apply all pending migrations
migrate-up:
	@echo "Running migrations up..."
	docker run --rm -v $(PWD)/migrations:/migrations --network preempt_preempt-network \
		migrate/migrate -path=/migrations -database "$(DB_URL)" up

## migrate-down: Rollback last migration
migrate-down:
	@echo "Rolling back migration..."
	docker run --rm -v $(PWD)/migrations:/migrations --network preempt_preempt-network \
		migrate/migrate -path=/migrations -database "$(DB_URL)" down 1

## migrate-down-all: Rollback all migrations
migrate-down-all:
	@echo "Rolling back all migrations..."
	docker run --rm -v $(PWD)/migrations:/migrations --network preempt_preempt-network \
		migrate/migrate -path=/migrations -database "$(DB_URL)" down -all

## migrate-create: Create a new migration (usage: make migrate-create NAME=add_users_table)
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	@echo "Creating migration: $(NAME)"
	docker run --rm -v $(PWD)/migrations:/migrations \
		migrate/migrate create -ext sql -dir /migrations -seq $(NAME)

## migrate-force: Force set migration version (usage: make migrate-force VERSION=1)
migrate-force:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make migrate-force VERSION=version_number"; exit 1; fi
	@echo "Forcing migration version to $(VERSION)..."
	docker run --rm -v $(PWD)/migrations:/migrations --network preempt_preempt-network \
		migrate/migrate -path=/migrations -database "$(DB_URL)" force $(VERSION)

## migrate-version: Show current migration version
migrate-version:
	@echo "Current migration version:"
	docker run --rm -v $(PWD)/migrations:/migrations --network preempt_preempt-network \
		migrate/migrate -path=/migrations -database "$(DB_URL)" version
