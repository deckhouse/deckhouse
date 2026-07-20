-include .env

# Export all variables from .env to sub-scripts
export

.PHONY: help install test check-env

help:
	@echo "Control Plane Manager E2E Test Suite"
	@echo ""
	@echo "Configuration:"
	@echo "  Copy .env.example to .env and fill in the required variables."
	@echo ""
	@echo "Commands:"
	@echo "  make install              - Install chainsaw (and jq/yq if missing) on the remote host"
	@echo "  make test                 - Run all tests on the remote host over SSH"
	@echo "  make test TEST=<name>     - Run a single scenario (e.g. TEST=basic-audit-policy)"
	@echo ""

check-env:
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Please run: cp .env.example .env"; \
		exit 1; \
	fi
	@if [ -z "$(CLUSTER_SSH_HOST)" ]; then \
		echo "Error: CLUSTER_SSH_HOST is not set in .env"; \
		exit 1; \
	fi

install: check-env
	./scripts/install-binary.sh

test: check-env
	@echo "Running tests on $(CLUSTER_SSH_HOST)..."
	TEST="$(TEST)" ./scripts/run-tests.sh
