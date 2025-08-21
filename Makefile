#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Makefile for protoc-gen-go-triple project
# This makefile is for ci test and local development

VERSION ?= latest

GO = go
GO_PATH = $(shell $(GO) env GOPATH)
GO_OS = $(shell $(GO) env GOOS)
ifeq ($(GO_OS), darwin)
    GO_OS = mac
endif
GO_BUILD = $(GO) build
GO_GET = $(GO) get
GO_TEST = $(GO) test
GO_BUILD_FLAGS = -v
GO_BUILD_LDFLAGS = -X main.version=$(VERSION)

SHELL = /bin/bash
# Uncomment to enable strict mode for all recipes (verify CI tolerance first)
# .SHELLFLAGS := -eu -o pipefail -c

# GolangCI-Lint version to install locally (v1.x to match config)
GOLANGCI_LINT_VERSION ?= v1.64.4

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: prepare
prepare: ## Prepare development environment
	@echo "Preparing development environment..."
	@go mod download
	@go install golang.org/x/tools/cmd/goimports@v0.24.0

.PHONY: deps
deps: prepare ## Install dependencies
	@echo "Installing dependencies..."
	@go mod tidy -v
	@go mod download

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w -local github.com/dubbogo/protoc-gen-go-triple .

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	@$(GO_TEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: test-short
test-short: ## Run tests without race detection
	@echo "Running tests (short mode)..."
	@$(GO_TEST) -v ./...

.PHONY: build
build: ## Build the project
	@echo "Building project..."
	@$(GO) build $(GO_BUILD_FLAGS) -ldflags="$(GO_BUILD_LDFLAGS)" ./...

.PHONY: install
install: ## Install the protoc plugin
	@echo "Installing protoc-gen-go-triple..."
	@$(GO) install -v -ldflags="$(GO_BUILD_LDFLAGS)" ./...

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf coverage.txt
	@rm -rf license-header-checker*
	@go clean -cache -testcache

.PHONY: verify
verify: clean fmt test ## Verify code quality (fmt + test)

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		$(MAKE) lint-install; \
	fi
	@golangci-lint run ./...

.PHONY: lint-install
lint-install: ## Install golangci-lint
	@echo "Installing golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
	  sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

.PHONY: lint-version
lint-version: ## Show installed golangci-lint version
	@which golangci-lint >/dev/null 2>&1 && golangci-lint --version | head -n1 || echo "golangci-lint not installed"

.PHONY: lint-upgrade
lint-upgrade: ## Force install requested golangci-lint version
	@echo "Upgrading golangci-lint to $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
	  sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

.PHONY: all
all: clean prepare deps fmt lint test build ## Run all checks and build

.PHONY: ci
ci: verify lint ## Run CI checks (verify + lint)

.PHONY: coverage
coverage: test ## Generate coverage report
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: proto-test
proto-test: ## Test protoc plugin with sample proto files
	@echo "Testing protoc plugin..."
	@if [ -d "test" ]; then \
		cd test && ./test.sh; \
	else \
		echo "Test directory not found"; \
	fi
