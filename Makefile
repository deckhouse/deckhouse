export PATH := $(abspath bin/):${PATH}

export BASE_NGINX_ALPINE=nginx:1.15.12-alpine@sha256:57a226fb6ab6823027c0704a9346a890ffb0cacde06bc19bbc234c8720673555
export BASE_ALPINE=alpine:3.12.1@sha256:c0e9560cda118f9ec63ddefb4a173a2b2a0347082d7dff7dc14272e7841a5b5a
export BASE_GOLANG_16_ALPINE=golang:1.16.3-alpine3.12@sha256:371dc6bf7e0c7ce112a29341b000c40d840aef1dbb4fdcb3ae5c0597e28f3061
export BASE_JEKYLL=jekyll/jekyll:3.8@sha256:9521c8aae4739fcbc7137ead19f91841b833d671542f13e91ca40280e88d6e34

FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

TESTS_TIMEOUT="15m"
FOCUS=""

MDLINTER_IMAGE = ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476

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


GOLANGCI_VERSION = 1.46.2
TESTS_TIMEOUT="15m"

##@ Tests

.PHONY: tests-modules tests-matrix tests-openapi
tests-modules: ## Run unit tests for modules hooks and templates.
	go test -timeout=${TESTS_TIMEOUT} -vet=off ./modules/... ./global-hooks/... ./ee/modules/... ./ee/fe/modules/...

tests-matrix: ## Test how helm templates are rendered with different input values generated from values examples. Use 'FOCUS' environment variable to run tests for a particular module.
	go test ./testing/matrix/ -v

tests-openapi: ## Run tests against modules openapi values schemas.
	go test -vet=off ./testing/openapi_cases/

.PHONY: validate
validate: ## Check common patterns through all modules.
	go test -tags=validation -run Validation -timeout=${TESTS_TIMEOUT} ./testing/...

bin/golangci-lint:
	mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | BINARY=golangci-lint bash -s -- v${GOLANGCI_VERSION}

.PHONY: lint lint-fix
lint: bin/golangci-lint ## Run linter.
	bin/golangci-lint run

lint-fix: bin/golangci-lint ## Fix lint violations.
	bin/golangci-lint run --fix

.PHONY: --lint-markdown-header lint-markdown lint-markdown-fix
--lint-markdown-header:
	@docker pull -q ${MDLINTER_IMAGE}
	@echo "\n######################################################################################################################"
	@echo '###'
	@echo "###                   Markdown linter report (powered by https://github.com/DavidAnson/markdownlint/)\n"

lint-markdown: --lint-markdown-header ## Run markdown linter.
	@bash -c \
   "if docker run --rm -v ${PWD}:/workdir ${MDLINTER_IMAGE} --config testing/markdownlint.yaml -p testing/.markdownlintignore '**/*.md' ; then \
	    echo; echo 'All checks passed.'; \
	  else \
	    echo; \
	    echo 'To run linter locally and fix common problems run: make lint-markdown-fix'; \
	    echo; \
	    exit 1; \
	  fi"

lint-markdown-fix: ## Run markdown linter and fix problems automatically.
	@docker run --rm -v ${PWD}:/workdir ${MDLINTER_IMAGE} \
		--config testing/markdownlint.yaml -p testing/.markdownlintignore "**/*.md" --fix && (echo 'Fixed successfully.')

##@ Generate

.PHONY: generate
generate: ## Run all generate-* jobs in bulk.
	cd tools; go generate

##@ Site up

.PHONY: docs
docs: ## Run containers with the documentation (werf is required to build the containers).
	docker network inspect deckhouse 2>/dev/null 1>/dev/null || docker network create deckhouse; \
	cd docs/documentation/; werf compose up --docker-compose-command-options='-d'; \
  cd ../site/; werf compose up --docker-compose-command-options='-d'; \
  echo "Open http://localhost to access the documentation..."

.PHONY: docs-down
docs-down: ## Stop all the documentation containers.
	docker rm -f site_site_1 site_front_1 documentation; docker network rm deckhouse
