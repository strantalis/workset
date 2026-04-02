SHELL := /bin/bash
PYTHON ?= python3
VENV ?= .venv
PORT ?= 8000
GOLANGCI_LINT_CACHE ?= /tmp/golangci-lint-cache
COG ?= cog

UV := $(shell command -v uv 2>/dev/null)
BASE_SHA ?= $(shell git merge-base HEAD origin/main 2>/dev/null)

.PHONY: help docs-venv docs-serve docs-build test lint lint-fmt fmt ui-lint ui-fmt ui-test guardrails deprecations check release-stable

help:
	@printf "%s\n" "Targets:" \
		"  docs-venv   Create/refresh docs venv and install requirements" \
		"  docs-serve  Run MkDocs dev server (override with PORT=8001)" \
		"  docs-build  Build MkDocs site" \
		"  test        Run Go and frontend tests" \
		"  lint        Run golangci-lint and frontend ESLint" \
		"  lint-fmt    Format Go files with golangci-lint fmt (gofumpt)" \
		"  fmt         Format Go and frontend files" \
		"  ui-lint     Run ESLint on frontend code" \
		"  ui-fmt      Format frontend code with Prettier" \
		"  ui-test     Run frontend tests" \
		"  guardrails  Run LOC guardrails (ratcheted against origin/main when available)" \
		"  deprecations Validate deprecation register deadlines and metadata" \
		"  check       fmt + test + lint + guardrails" \
		"  release-stable Prepare a signed stable bump+tag locally on a release branch"

docs-venv:
	@if [ -z "$(UV)" ]; then \
		echo "uv not found. Install uv and re-run 'make docs-venv'."; \
		exit 1; \
	fi
	uv venv $(VENV)
	uv pip install -r requirements.txt

docs-serve: docs-venv
	$(VENV)/bin/mkdocs serve --dev-addr 127.0.0.1:$(PORT) --livereload --watch docs --watch mkdocs.yml

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

guardrails:
	@if [ -n "$(BASE_SHA)" ]; then \
		echo "Running guardrails with base=$(BASE_SHA)"; \
		go run ./scripts/guardrails --config guardrails.yml --base-sha "$(BASE_SHA)" --head-sha "$$(git rev-parse HEAD)"; \
	else \
		echo "origin/main merge-base not found; running guardrails without base ratchet"; \
		go run ./scripts/guardrails --config guardrails.yml --head-sha "$$(git rev-parse HEAD)"; \
	fi

deprecations:
	go run ./scripts/deprecations --config docs-internal/architecture/deprecation-register.yaml

check: fmt test lint guardrails deprecations

release-stable:
	@set -euo pipefail; \
		branch="$$(git rev-parse --abbrev-ref HEAD)"; \
		if ! command -v "$(COG)" >/dev/null 2>&1; then \
			echo "cog is not installed. Install Cocogitto and rerun 'make release-stable'."; \
			exit 1; \
		fi; \
		if ! git diff --quiet || ! git diff --cached --quiet; then \
			echo "Working tree must be clean before preparing a stable release."; \
			exit 1; \
		fi; \
		signing_key="$$(git config --get user.signingkey || true)"; \
		if [ -z "$$signing_key" ]; then \
			echo "git user.signingkey is not configured."; \
			exit 1; \
		fi; \
		echo "Running repo checks before creating the release commit and tag..."; \
		$(MAKE) check; \
		echo "Creating signed release commit on branch '$$branch'..."; \
		GIT_CONFIG_COUNT=2 \
		GIT_CONFIG_KEY_0=commit.gpgsign \
		GIT_CONFIG_VALUE_0=true \
		GIT_CONFIG_KEY_1=tag.gpgsign \
		GIT_CONFIG_VALUE_1=true \
		"$(COG)" bump --auto; \
		tag="$$(git describe --tags --abbrev=0)"; \
		echo "Rewriting $$tag as a signed annotated tag..."; \
		git tag -d "$$tag" >/dev/null; \
		git tag -s "$$tag" -m "$$tag"; \
		if ! git cat-file -p HEAD | grep -q '^gpgsig '; then \
			echo "Release commit is missing an embedded git signature."; \
			exit 1; \
		fi; \
		if [ "$$(git cat-file -t "$$tag")" != "tag" ]; then \
			echo "Release tag '$$tag' is not an annotated tag."; \
			exit 1; \
		fi; \
		if ! git cat-file -p "$$tag" | grep -Eq 'BEGIN SSH SIGNATURE|BEGIN PGP SIGNATURE'; then \
			echo "Release tag '$$tag' is missing an embedded signature."; \
			exit 1; \
		fi; \
		echo ""; \
		echo "Prepared $$tag on branch '$$branch'."; \
		echo "Next steps:"; \
		echo "  1. git push origin HEAD:main"; \
		echo "  2. git push origin $$tag"
