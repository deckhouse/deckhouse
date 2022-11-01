export DOCKERIZED := 1

FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

TESTS_TIMEOUT="15m"
FOCUS=""

MDLINTER_IMAGE = ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476

# Set testing path for tests-modules
ifeq ($(FOCUS),"")
       TESTS_PATH = ./modules/... ./global-hooks/... ./ee/modules/... ./ee/fe/modules/...
else
       TESTS_PATH = $(wildcard ./modules/*-${FOCUS} ./ee/modules/*-${FOCUS} ./ee/fe/modules/*-${FOCUS})/...
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


TESTS_TIMEOUT="15m"

##@ Tests

.PHONY: tests-modules tests-matrix tests-openapi tests-prometheus
tests-modules: ## Run unit tests for modules hooks and templates.
  ##~ Options: FOCUS=module-name
	@./tools/dockerized.sh \
		"go test -timeout=${TESTS_TIMEOUT} -vet=off ${TESTS_PATH}"

tests-matrix: ## Test how helm templates are rendered with different input values generated from values examples.
  ##~ Options: FOCUS=module-name
	@./tools/dockerized.sh \
		"go test ./testing/matrix/ -v"

tests-openapi: ## Run tests against modules openapi values schemas.
	@./tools/dockerized.sh \
		"go test -vet=off ./testing/openapi_cases/"

.PHONY: validate
validate: ## Check common patterns through all modules.
	@./tools/dockerized.sh \
		"go test -tags=validation -run Validation -timeout=${TESTS_TIMEOUT} ./testing/..."

.PHONY: lint lint-fix
lint: ## Run linter.
	@./tools/dockerized.sh \
		"golangci-lint run"

lint-fix: ## Fix lint violations.
	@./tools/dockerized.sh \
		"golangci-lint run --fix"

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
	@./tools/dockerized.sh \
		"cd tools; go generate" \
    "cd /deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot; go generate ."

render-workflow: ## Generate CI workflow instructions.
	./.github/render-workflows.sh

##@ Security
bin/regcopy: ## App to copy docker images to the Deckhouse registry
	mkdir -p bin
	cd tools/regcopy; go build -o bin/regcopy

.PHONY: cve-report cve-base-images
cve-report: ## Generate CVE report for a Deckhouse release.
  ##~ Options: SEVERITY=CRITICAL,HIGH REPO=registry.deckhouse.io TAG=v1.30.0
	./tools/cve/release.sh

cve-base-images: ## Check CVE in our base images.
  ##~ Options: SEVERITY=CRITICAL,HIGH
	@./tools/dockerized.sh \
		"./tools/cve/base-images.sh"

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

.PHONY: update-k8s-patch-versions
update-k8s-patch-versions: ## Run update-patchversion script to generate new version_map.yml.
	cd candi/tools; bash update_kubernetes_patchversions.sh
