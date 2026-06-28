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
