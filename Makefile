export PATH := $(abspath bin/):${PATH}

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
	@awk 'BEGIN {\
	    FS = ":.*##"; \
	    printf                "Usage: ${FORMATTING_BEGIN_BLUE}OPTION${FORMATTING_END}=<value> make ${FORMATTING_BEGIN_YELLOW}<target>${FORMATTING_END}\n"\
	  } \
	  /^[a-zA-Z0-9_-]+:.*?##/ { printf "  ${FORMATTING_BEGIN_BLUE}%-46s${FORMATTING_END} %s\n", $$1, $$2 } \
	  /^.?.?##~/              { printf "   %-46s${FORMATTING_BEGIN_YELLOW}%-46s${FORMATTING_END}\n", "", substr($$1, 6) } \
	  /^##@/                  { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


GOLANGCI_VERSION = 1.46.2
TRIVY_VERSION= 0.28.1
TESTS_TIMEOUT="15m"

##@ General

deps: bin/golangci-lint bin/trivy bin/regcopy ## Install dev dependencies.

##@ Tests

.PHONY: tests-modules tests-matrix tests-openapi
tests-modules: ## Run unit tests for modules hooks and templates.
	go test -timeout=${TESTS_TIMEOUT} -vet=off ./modules/... ./global-hooks/... ./ee/modules/... ./ee/fe/modules/...

tests-matrix: ## Test how helm templates are rendered with different input values generated from values examples.
  ##~ Options: FOCUS=module-name
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
lint: ## Run linter.
	golangci-lint run

lint-fix: ## Fix lint violations.
	golangci-lint run --fix

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

.PHONY: generate render-workflow
generate: ## Run all generate-* jobs in bulk.
	cd tools; go generate

render-workflow: ## Generate CI workflow instructions.
	./.github/render-workflows.sh

##@ Security

bin/regcopy: ## App to copy docker images to the Deckhouse registry
	mkdir -p bin
	cd tools/regcopy; go build -o $(PWD)/bin/regcopy

bin/regcopy: ## App to copy docker images to the Deckhouse registry
	mkdir -p bin
	cd tools/regcopy; go build -o $(PWD)/bin/regcopy

bin/trivy:
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b ./bin v${TRIVY_VERSION}

.PHONY: cve-report cve-base-images
cve-report: ## Generate CVE report for a Deckhouse release.
  ##~ Options: SEVERITY=CRITICAL,HIGH REPO=registry.deckhouse.io TAG=v1.30.0
	./tools/cve/release.sh

cve-base-images: ## Check CVE in our base images.
  ##~ Options: SEVERITY=CRITICAL,HIGH
	./tools/cve/base-images.sh
