# Makefile for DNI RADIUS Server Project

# Variables
SERVER_MAIN=./cmd/api
CONSUMER_MAIN=./cmd/consumer

# Default target
.DEFAULT_GOAL := help

# Run targets
.PHONY: run-server run-consumer

run-server: ## Run the RADIUS server
	@echo "Starting RADIUS server..."
	@go run $(SERVER_MAIN)

run-consumer: ## Run the Redis consumer (requires USERNAME env var)
	@echo "Starting Redis consumer..."
	@if [ -z "$(USERNAME)" ]; then \
		echo "Error: USERNAME environment variable is required"; \
		echo "Usage: make run-consumer USERNAME=testuser-1"; \
		exit 1; \
	fi
	@USERNAME=$(USERNAME) go run $(CONSUMER_MAIN)

# Help target
.PHONY: help

help: ## Show available commands
	@echo "DNI RADIUS Server - Available commands:"
	@echo ""
	@echo "  make run-server                    # Run the RADIUS server"
	@echo "  make run-consumer USERNAME=user1   # Run consumer for specified user"
	@echo ""
	@echo "Examples:"
	@echo "  make run-server"
	@echo "  make run-consumer USERNAME=testuser-1"