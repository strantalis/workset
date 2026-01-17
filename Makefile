SHELL := /bin/bash
PYTHON ?= python3
VENV ?= .venv
GOLANGCI_LINT_CACHE ?= /tmp/golangci-lint-cache

UV := $(shell command -v uv 2>/dev/null)

.PHONY: help docs-venv docs-serve docs-build test lint fmt check

help:
	@printf "%s\n" "Targets:" \
		"  docs-venv   Create/refresh docs venv and install requirements" \
		"  docs-serve  Run MkDocs dev server" \
		"  docs-build  Build MkDocs site" \
		"  test        Run Go tests" \
		"  lint        Run golangci-lint" \
		"  fmt         Format Go files" \
		"  check       fmt + test + lint"

docs-venv:
	@if [ -z "$(UV)" ]; then \
		echo "uv not found. Install uv and re-run 'make docs-venv'."; \
		exit 1; \
	fi
	uv venv $(VENV)
	uv pip install -r requirements.txt

docs-serve: docs-venv
	$(VENV)/bin/mkdocs serve --livereload --watch docs --watch mkdocs.yml

docs-build: docs-venv
	$(VENV)/bin/mkdocs build

test:
	go test ./...

lint:
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run

fmt:
	gofmt -w ./cmd ./internal

check: fmt test lint
