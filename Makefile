.PHONY: help setup up down restart logs migrate test test-api test-web bench lint clean

SHELL := /bin/bash

help: ## Show this help message
	@echo "Observatory APM Makefile Commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

setup: ## Copy environment file and verify prerequisites
	@test -f .env || cp .env.example .env
	@echo "Setup complete. Configured .env from .env.example"

up: setup ## Start all standard infrastructure and application containers
	docker compose up -d timescaledb redis api web demo

down: ## Stop all running containers
	docker compose down

restart: down up ## Restart all services

logs: ## Tail all service logs
	docker compose logs -f

migrate: ## Run schema migrations against local TimescaleDB
	@echo "Running schema migrations..."
	@psql -h localhost -U observatory -d observatory -f db/migrations/000001_init_schema.up.sql

test: test-api test-web ## Run all test suites across backend and frontend

test-api: ## Run Go unit and integration tests with race detector
	@echo "Running API tests..."
	cd api && go test -v -race ./...

test-web: ## Run Web tests and TypeScript type checking
	@echo "Running Web type-check and tests..."
	cd web && npm run type-check

bench: ## Execute high-throughput OTLP ingest benchmark
	@echo "Running Ingest Benchmark (Target: 10,000 points/sec)..."
	docker compose --profile benchmark run --rm loadgen

lint: ## Run linters across Go and TypeScript
	cd api && go vet ./...
	cd web && npm run lint

clean: ## Remove temporary build and test artifacts
	rm -rf api/bin/ web/.next/ coverage/ test-results/
