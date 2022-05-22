export PATH := $(abspath bin/):${PATH}

FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

TESTS_TIMEOUT="15m"
FOCUS=""

help:
	@printf -- "${FORMATTING_BEGIN_BLUE}%s${FORMATTING_END}\n" \
	"" \
	"     ██████╗░███████╗░█████╗░██╗░░██╗██╗░░██╗░█████╗░██╗░░░██╗░██████╗███████╗" \
	"     ██╔══██╗██╔════╝██╔══██╗██║░██╔╝██║░░██║██╔══██╗██║░░░██║██╔════╝██╔════╝" \
	"     ██║░░██║█████╗░░██║░░╚═╝█████═╝░███████║██║░░██║██║░░░██║╚█████╗░█████╗░░" \
	"     ██║░░██║██╔══╝░░██║░░██╗██╔═██╗░██╔══██║██║░░██║██║░░░██║░╚═══██╗██╔══╝░░" \
	"     ██████╔╝███████╗╚█████╔╝██║░╚██╗██║░░██║╚█████╔╝╚██████╔╝██████╔╝███████╗" \
	"     ╚═════╝░╚══════╝░╚════╝░╚═╝░░╚═╝╚═╝░░╚═╝░╚════╝░░╚═════╝░╚═════╝░╚══════╝" \
	"" \
	"-----------------------------------------------------------------------------------" \
	""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make ${FORMATTING_BEGIN_YELLOW}<target>${FORMATTING_END}\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  ${FORMATTING_BEGIN_BLUE}%-46s${FORMATTING_END} %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


GOLANGCI_VERSION = 1.42.0
TESTS_TIMEOUT="15m"

##@ Tests

.PHONY: tests-modules tests-matrix tests-openapi
tests-modules: ## Run unit tests for modules hooks and templates.
	@go test -timeout=${TESTS_TIMEOUT} -vet=off ./modules/... ./global-hooks/... ./ee/modules/... ./ee/fe/modules/...

tests-matrix: ## Test how helm templates are rendered with different input values generated from values examples. Use 'FOCUS' environment variable to run tests for a particular module.
	@go test ./testing/matrix/ -v

tests-openapi: ## Run tests against modules openapi values schemas.
	@go test -vet=off ./testing/openapi_cases/

.PHONY: validate
validate: ## Check common patterns through all modules.
	@go test -tags=validation -run Validation -timeout=${TESTS_TIMEOUT} ./testing/...

bin/golangci-lint:
	@mkdir -p bin
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | BINARY=golangci-lint bash -s -- v${GOLANGCI_VERSION}

.PHONY: lint lint-fix
lint: bin/golangci-lint ## Run linter.
	@bin/golangci-lint run

lint-fix: bin/golangci-lint ## Fix lint violations.
	@bin/golangci-lint run --fix

##@ Generate

.PHONY: generate
generate: ## Run all generate-* jobs in bulk.
	@cd tools; go generate
