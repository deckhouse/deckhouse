export PATH := $(abspath bin/):${PATH}

FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

TESTS_TIMEOUT="15m"
FOCUS=""

MDLINTER_IMAGE = ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476

# Explicitly set architecture on arm, since werf currently does not support building of images for any other platform
# besides linux/amd64 (e.g. relevant for mac m1).
PLATFORM_NAME := $(shell uname -p)
OS_NAME := $(shell uname)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

# Set platform for jq
ifeq ($(OS_NAME), Linux)
	JQ_PLATFORM = linux64
else ifeq ($(OS_NAME), Darwin)
	JQ_PLATFORM = osx-amd64
endif

# Set platform for yq
ifeq ($(OS_NAME), Linux)
	YQ_PLATFORM = linux
else ifeq ($(OS_NAME), Darwin)
	YQ_PLATFORM = darwin
endif
# Set arch for yq
ifeq ($(PLATFORM_NAME), x86_64)
	YQ_ARCH = amd64
else ifeq ($(PLATFORM_NAME), arm)
	YQ_ARCH = arm64
endif

# Set arch for crane
ifeq ($(PLATFORM_NAME), x86_64)
	CRANE_ARCH = x86_64
else ifeq ($(PLATFORM_NAME), arm)
	CRANE_ARCH = arm64
endif

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

deps: bin/golangci-lint bin/trivy bin/regcopy bin/jq bin/yq bin/crane ## Install dev dependencies.

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

bin/trivy:
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b ./bin v${TRIVY_VERSION}

.PHONY: cve-report cve-base-images
cve-report: ## Generate CVE report for a Deckhouse release.
  ##~ Options: SEVERITY=CRITICAL,HIGH REPO=registry.deckhouse.io TAG=v1.30.0
	./tools/cve/release.sh

cve-base-images: ## Check CVE in our base images.
  ##~ Options: SEVERITY=CRITICAL,HIGH
	./tools/cve/base-images.sh

##@ Documentation

.PHONY: docs
docs: ## Run containers with the documentation (werf is required to build the containers).
	docker network inspect deckhouse 2>/dev/null 1>/dev/null || docker network create deckhouse
	cd docs/documentation/; werf compose up --docker-compose-command-options='-d' --env local
	cd docs/site/; werf compose up --docker-compose-command-options='-d' --env local
	echo "Open http://localhost to access the documentation..."

.PHONY: docs-dev
docs-dev: ## Run containers with the documentation in the dev mode (allow uncommited files).
	docker network inspect deckhouse 2>/dev/null 1>/dev/null || docker network create deckhouse
	cd docs/documentation/; werf compose up --docker-compose-command-options='-d' --dev --env development
	cd docs/site/; werf compose up --docker-compose-command-options='-d' --dev --env development
	echo "Open http://localhost to access the documentation..."

.PHONY: docs-down
docs-down: ## Stop all the documentation containers.
	docker rm -f site_site_1 site_front_1 documentation; docker network rm deckhouse

##@ Update kubernetes control-plane patchversions

bin/jq: ## Install jq deps for update-patchversion script.
	curl -sSfL https://github.com/stedolan/jq/releases/download/jq-1.6/jq-$(JQ_PLATFORM) -o $(PWD)/bin/jq && chmod +x $(PWD)/bin/jq

bin/yq: ## Install yq deps for update-patchversion script.
	curl -sSfL https://github.com/mikefarah/yq/releases/download/v4.25.3/yq_$(YQ_PLATFORM)_$(YQ_ARCH) -o $(PWD)/bin/yq && chmod +x $(PWD)/bin/yq

bin/crane: ## Install crane deps for update-patchversion script.
	curl -sSfL https://github.com/google/go-containerregistry/releases/download/v0.10.0/go-containerregistry_$(OS_NAME)_$(CRANE_ARCH).tar.gz | tar -xzf - crane && mv crane $(PWD)/bin/crane && chmod +x $(PWD)/bin/crane

.PHONY: update-k8s-patch-versions
update-k8s-patch-versions: ## Run update-patchversion script to generate new version_map.yml.
	cd candi/tools; bash update_kubernetes_patchversions.sh
