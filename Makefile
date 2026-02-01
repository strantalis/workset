SHELL := /bin/bash
PYTHON ?= python3
VENV ?= .venv
GOLANGCI_LINT_CACHE ?= /tmp/golangci-lint-cache

UV := $(shell command -v uv 2>/dev/null)

.PHONY: help docs-venv docs-serve docs-build test lint lint-fmt fmt ui-lint ui-fmt ui-test check

help:
	@printf "%s\n" "Targets:" \
		"  docs-venv   Create/refresh docs venv and install requirements" \
		"  docs-serve  Run MkDocs dev server" \
		"  docs-build  Build MkDocs site" \
		"  test        Run Go and frontend tests" \
		"  lint        Run golangci-lint and frontend ESLint" \
		"  lint-fmt    Format Go files with golangci-lint fmt (gofumpt)" \
		"  fmt         Format Go and frontend files" \
		"  ui-lint     Run ESLint on frontend code" \
		"  ui-fmt      Format frontend code with Prettier" \
		"  ui-test     Run frontend tests" \
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

test: ui-test
	go test ./...

ui-test:
	cd wails-ui/workset/frontend && npm run test

lint: ui-lint
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run

lint-fmt:
	golangci-lint fmt

fmt: ui-fmt
	gofmt -w ./cmd ./internal

ui-lint:
	cd wails-ui/workset/frontend && npm run lint

ui-fmt:
	cd wails-ui/workset/frontend && npm run format

check: fmt test lint
