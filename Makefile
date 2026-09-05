# Mozi Builder Makefile
# Model-Driven Development Platform CLI

BINARY_NAME := mozi
BIN_DIR := bin
GO := go
GOPROXY ?= https://goproxy.cn,direct

# Installation paths (configurable via environment)
PREFIX ?= /usr/local
INSTALL_DIR ?= $(PREFIX)/bin

# Build information
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
VERSION   := $(shell cat VERSION 2>/dev/null || echo "dev")

LDFLAGS := -X 'github.com/pangu-studio/mozi-builder/cmd/mozi/cmd.version=$(VERSION)' \
           -X 'main.BuildTime=$(BUILD_TIME)' \
           -X 'main.GitCommit=$(GIT_COMMIT)' \
           -X 'main.GitBranch=$(GIT_BRANCH)'

.PHONY: all build install clean test

## build: Compile the mozi binary
build:
	@echo "→ Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/mozi
	@echo "✓ Binary built: $(BIN_DIR)/$(BINARY_NAME)"

## install: Install mozi to $(INSTALL_DIR) (set PREFIX to override, e.g. PREFIX=/usr/local)
install: build
	@echo "→ Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Installed: $(INSTALL_DIR)/$(BINARY_NAME)"

## clean: Remove build artifacts
clean:
	@echo "→ Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
	@echo "✓ Clean"

## test: Run all tests
test:
	$(GO) test ./...

# v2 is an independent module; these targets do not change v1 dependencies.
COMPOSE ?= docker compose
POC_COMPOSE := platform/deploy/poc/compose.yaml
.PHONY: v2-test v2-migrate v2-migrate-verify v2-poc-build v2-poc-up v2-poc-down v2-poc-smoke v2-web-build
v2-test:
	cd platform && $(GO) test ./...
v2-migrate:
	cd platform && $(GO) run ./cmd/platform-migrate -env-file ../.env -target design
	cd platform && $(GO) run ./cmd/platform-migrate -env-file ../.env -target platform
v2-migrate-verify:
	cd platform && $(GO) run ./cmd/platform-migrate -env-file ../.env -target design -verify
	cd platform && $(GO) run ./cmd/platform-migrate -env-file ../.env -target platform -verify
v2-poc-build:
	platform/deploy/poc/build.sh
v2-poc-up: v2-poc-build
	$(COMPOSE) -f $(POC_COMPOSE) up -d --no-build
v2-poc-down:
	$(COMPOSE) -f $(POC_COMPOSE) down
v2-poc-smoke:
	python3 platform/deploy/poc/smoke.py
v2-web-build:
	cd platform/web && npm ci && npm run build
