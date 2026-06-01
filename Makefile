.PHONY: test lint check

CACHE_DIR := $(CURDIR)/.rtk-cache
GOCACHE := $(CACHE_DIR)/go-build
GOLANGCI_LINT_CACHE := $(CACHE_DIR)/golangci-lint

test:
	rtk go test ./...

lint:
	rtk proxy env GOCACHE=$(GOCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run ./...

check: test lint
