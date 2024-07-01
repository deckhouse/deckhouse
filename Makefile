export PATH := $(abspath bin/):${PATH}

FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

FOCUS =

MDLINTER_IMAGE = ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476

# Explicitly set architecture on arm, since werf currently does not support building of images for any other platform
# besides linux/amd64 (e.g. relevant for mac m1).
PLATFORM_NAME := $(shell uname -m)
OS_NAME := $(shell uname)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

# Set platform for deps
ifeq ($(OS_NAME), Linux)
	JQ_PLATFORM = linux64
	YQ_PLATFORM = linux
	TRDL_PLATFORM = linux
else ifeq ($(OS_NAME), Darwin)
	JQ_PLATFORM = osx-amd64
	YQ_PLATFORM = darwin
	TRDL_PLATFORM = darwin
endif
JQ_VERSION = 1.6

# Set arch for deps
ifeq ($(PLATFORM_NAME), x86_64)
	YQ_ARCH = amd64
	CRANE_ARCH = x86_64
	TRDL_ARCH = amd64
else ifeq ($(PLATFORM_NAME), arm64)
	YQ_ARCH = arm64
	CRANE_ARCH = arm64
	TRDL_ARCH = arm64
endif


# Set arch for crane
ifeq ($(PLATFORM_NAME), x86_64)
	CRANE_ARCH = x86_64
else ifeq ($(PLATFORM_NAME), arm64)
	CRANE_ARCH = arm64
endif

# Set testing path for tests-modules
ifeq ($(FOCUS),)
	TESTS_PATH = ./modules/... ./global-hooks/... ./ee/modules/... ./ee/fe/modules/... ./ee/be/modules/... ./ee/se/modules/...
else
	CE_MODULES = $(shell find ./modules -maxdepth 1 -regex ".*[0-9]-${FOCUS}")
	ifneq ($(CE_MODULES),)
		CE_MODULES_RECURSE = ${CE_MODULES}/...
	endif
	EE_MODULES = $(shell find ./ee/modules -maxdepth 1 -regex ".*[0-9]-${FOCUS}")
	ifneq ($(EE_MODULES),)
		EE_MODULES_RECURSE = ${EE_MODULES}/...
	endif
	FE_MODULES = $(shell find ./ee/fe/modules -maxdepth 1 -regex ".*[0-9]-${FOCUS}")
	ifneq ($(FE_MODULES),)
		FE_MODULES_RECURSE = ${FE_MODULES}/...
	endif
	BE_MODULES = $(shell find ./ee/be/modules -maxdepth 1 -regex ".*[0-9]-${FOCUS}")
	ifneq ($(FE_MODULES),)
		BE_MODULES_RECURSE = ${BE_MODULES}/...
	endif
	SE_MODULES = $(shell find ./ee/se/modules -maxdepth 1 -regex ".*[0-9]-${FOCUS}")
	ifneq ($(FE_MODULES),)
		SE_MODULES_RECURSE = ${SE_MODULES}/...
	endif
	TESTS_PATH = ${CE_MODULES_RECURSE} ${EE_MODULES_RECURSE} ${FE_MODULES_RECURSE} ${BE_MODULES_RECURSE} ${SE_MODULES_RECURSE}
endif

# Set host arch & OS for golang-based programs, e.g. Prometheus
ifneq (, $(shell which go))
	GOHOSTARCH := $(shell go env GOHOSTARCH)
	GOHOSTOS := $(shell go env GOHOSTOS)
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


GOLANGCI_VERSION = 1.54.2
TRIVY_VERSION= 0.38.3
PROMTOOL_VERSION = 2.37.0
GATOR_VERSION = 3.9.0
TESTS_TIMEOUT="15m"

##@ General

deps: bin/golangci-lint bin/trivy bin/regcopy bin/jq bin/yq bin/crane bin/promtool bin/gator bin/werf ## Install dev dependencies.

##@ Tests

bin/promtool-${PROMTOOL_VERSION}/promtool:
	mkdir -p bin/promtool-${PROMTOOL_VERSION}
	curl -sSfL https://github.com/prometheus/prometheus/releases/download/v${PROMTOOL_VERSION}/prometheus-${PROMTOOL_VERSION}.${GOHOSTOS}-${GOHOSTARCH}.tar.gz -o - | tar zxf - -C bin/promtool-${PROMTOOL_VERSION} --strip=1 prometheus-${PROMTOOL_VERSION}.${GOHOSTOS}-${GOHOSTARCH}/promtool

.PHONY: bin/promtool
bin/promtool: bin/promtool-${PROMTOOL_VERSION}/promtool
	rm -f bin/promtool
	ln -s promtool-${PROMTOOL_VERSION}/promtool bin/promtool

bin/gator-${GATOR_VERSION}/gator:
	mkdir -p bin/gator-${GATOR_VERSION}
	curl -sSfL https://github.com/open-policy-agent/gatekeeper/releases/download/v${GATOR_VERSION}/gator-v${GATOR_VERSION}-${GOHOSTOS}-${GOHOSTARCH}.tar.gz -o - | tar zxf - -C bin/gator-${GATOR_VERSION} gator

.PHONY: bin/gator
bin/gator: bin/gator-${GATOR_VERSION}/gator
	rm -f bin/gator
	ln -s /deckhouse/bin/gator-${GATOR_VERSION}/gator bin/gator

.PHONY: tests-modules tests-matrix tests-openapi tests-controller
tests-modules: ## Run unit tests for modules hooks and templates.
  ##~ Options: FOCUS=module-name
	go test -timeout=${TESTS_TIMEOUT} -vet=off ${TESTS_PATH}

tests-matrix: bin/promtool bin/gator ## Test how helm templates are rendered with different input values generated from values examples.
  ##~ Options: FOCUS=module-name
	go test -timeout=${TESTS_TIMEOUT} ./testing/matrix/ -v

tests-openapi: ## Run tests against modules openapi values schemas.
	go test -vet=off ./testing/openapi_cases/

tests-controller: ## Run deckhouse-controller unit tests.
	go test ./deckhouse-controller/... -v

.PHONY: tests-doc-links
tests-doc-links: ## Build documentation and run checker of html links.
	bash tools/doc_check_links.sh


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
generate: bin/werf ## Run all generate-* jobs in bulk.
	cd tools; go generate

render-workflow: ## Generate CI workflow instructions.
	./.github/render-workflows.sh

##@ Security
bin:
	mkdir -p bin

bin/regcopy: bin ## App to copy docker images to the Deckhouse registry
	cd tools/regcopy; go build -o bin/regcopy

bin/trivy-${TRIVY_VERSION}/trivy:
	mkdir -p bin/trivy-${TRIVY_VERSION}
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b ./bin/trivy-${TRIVY_VERSION} v${TRIVY_VERSION}

.PHONY: trivy
bin/trivy: bin bin/trivy-${TRIVY_VERSION}/trivy
	rm -f bin/trivy
	ln -s trivy-${TRIVY_VERSION}/trivy bin/trivy

.PHONY: cve-report cve-base-images
cve-report: bin/trivy bin/jq ## Generate CVE report for a Deckhouse release.
  ##~ Options: SEVERITY=CRITICAL,HIGH REPO=registry.deckhouse.io TAG=v1.30.0
	./tools/cve/d8-images.sh

cve-base-images: bin/trivy bin/jq ## Check CVE in our base images.
  ##~ Options: SEVERITY=CRITICAL,HIGH
	./tools/cve/base-images.sh

##@ Documentation

.PHONY: docs
docs: ## Run containers with the documentation.
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
docs-down: ## Stop all the documentation containers (e.g. site_site_1 - for Linux, and site-site-1 for MacOs)
	docker rm -f site-site-1 site-front-1 site_site_1 site_front_1 documentation 2>/dev/null; docker network rm deckhouse

.PHONY: docs-spellcheck
docs-spellcheck: ## Check the spelling in the documentation.
  ##~ Options: file="path/to/file" (Specify a path to a specific file)
	cd tools/spelling && werf run docs-spell-checker --dev --docker-options="--entrypoint=sh" -- /app/spell_check.sh -f $(file)

lint-doc-spellcheck-pr:
	@cd tools/spelling && werf run docs-spell-checker --dev --docker-options="--entrypoint=bash" -- /app/check_diff.sh

.PHONY: docs-spellcheck-generate-dictionary
docs-spellcheck-generate-dictionary: ## Generate a dictionary (run it after adding new words to the tools/spelling/wordlist file).
	@echo "Sorting wordlist..."
	@sort ./tools/spelling/wordlist -o ./tools/spelling/wordlist
	@echo "Generating dictionary..."
	@test -f ./tools/spelling/dictionaries/dev_OPS.dic && rm ./tools/spelling/dictionaries/dev_OPS.dic
	@touch ./tools/spelling/dictionaries/dev_OPS.dic
	@cat ./tools/spelling/wordlist | wc -l | sed 's/^[ \t]*//g' > ./tools/spelling/dictionaries/dev_OPS.dic
	@sort ./tools/spelling/wordlist >> ./tools/spelling/dictionaries/dev_OPS.dic
	@echo "Don't forget to commit changes and push it!"
	@git diff --stat

.PHONY: docs-spellcheck-get-typos-list
docs-spellcheck-get-typos-list: ## Print out a list of all the terms in all pages that were considered as a typo.
	@cd tools/spelling && werf run docs-spell-checker --dev --docker-options="--entrypoint=sh" -- "/app/spell_check.sh" 2>/dev/null | sed "1,/Spell-checking the documentation/ d; /^Possible typos/d" | sort -u

##@ Update kubernetes control-plane patchversions

bin/jq-$(JQ_VERSION)/jq:
	mkdir -p bin/jq-$(JQ_VERSION)
	curl -sSfL https://github.com/stedolan/jq/releases/download/jq-$(JQ_VERSION)/jq-$(JQ_PLATFORM) -o $(PWD)/bin/jq-$(JQ_VERSION)/jq && chmod +x $(PWD)/bin/jq-$(JQ_VERSION)/jq

.PHONY: bin/jq
bin/jq: bin bin/jq-$(JQ_VERSION)/jq ## Install jq deps for update-patchversion script.
	rm -f bin/jq
	ln -s jq-$(JQ_VERSION)/jq bin/jq

bin/yq: bin ## Install yq deps for update-patchversion script.
	curl -sSfL https://github.com/mikefarah/yq/releases/download/v4.25.3/yq_$(YQ_PLATFORM)_$(YQ_ARCH) -o bin/yq && chmod +x bin/yq

bin/crane: bin ## Install crane deps for update-patchversion script.
	curl -sSfL https://github.com/google/go-containerregistry/releases/download/v0.10.0/go-containerregistry_$(OS_NAME)_$(CRANE_ARCH).tar.gz | tar -xzf - crane && mv crane bin/crane && chmod +x bin/crane

bin/trdl: bin
	curl -sSfL https://tuf.trdl.dev/targets/releases/0.6.3/$(TRDL_PLATFORM)-$(TRDL_ARCH)/bin/trdl -o bin/trdl
	chmod +x bin/trdl

bin/werf: bin bin/trdl ## Install werf for images-digests generator.
	trdl --home-dir bin/.trdl add werf https://tuf.werf.io 1 b7ff6bcbe598e072a86d595a3621924c8612c7e6dc6a82e919abe89707d7e3f468e616b5635630680dd1e98fc362ae5051728406700e6274c5ed1ad92bea52a2 && \
	trdl --home-dir bin/.trdl --no-self-update=true update werf 1.2 stable
	ln -sf $$(bin/trdl --home-dir bin/.trdl bin-path werf 1.2 stable | sed 's|^.*/bin/\(.trdl.*\)|\1/werf|') bin/werf

.PHONY: update-k8s-patch-versions
update-k8s-patch-versions: ## Run update-patchversion script to generate new version_map.yml.
	cd candi/tools; bash update_kubernetes_patchversions.sh

##@ Lib helm
.PHONY: update-lib-helm
update-lib-helm: ## Update lib-helm.
	##~ Options: version=MAJOR.MINOR.PATCH
	cd helm_lib/ && yq -i '.dependencies[0].version = "$(version)"' Chart.yaml && helm dependency update && tar -xf charts/deckhouse_lib_helm-*.tgz -C charts/ && rm charts/deckhouse_lib_helm-*.tgz && git add Chart.yaml Chart.lock charts/*

##@ Build
.PHONY: build
set-build-envs:
  ifeq ($(WERF_ENV),)
  	export WERF_ENV=FE
  endif
  ifeq ($(WERF_CHANNEL),)
 		export WERF_CHANNEL=ea
  endif
  ifeq ($(DEV_REGISTRY_PATH),)
 		export DEV_REGISTRY_PATH=dev-registry.deckhouse.io/sys/deckhouse-oss
  endif
  ifeq ($(SOURCE_REPO),)
 		export SOURCE_REPO=https://github.com
  endif
  ifeq ($(GOPROXY),)
 		export GOPROXY=https://proxy.golang.org/
  endif
  ifeq ($(CI_COMMIT_TAG),)
 		export CI_COMMIT_TAG=$(shell git describe --abbrev=0 2>/dev/null)
  endif
  ifeq ($(CI_COMMIT_BRANCH),)
 		export CI_COMMIT_BRANCH=$(shell git branch --show-current)
  endif
  ifeq ($(CI_COMMIT_REF_NAME),)
 		export CI_COMMIT_REF_NAME=$(shell git rev-parse --abbrev-ref HEAD)
 	else
		ifeq ($(CI_COMMIT_TAG),)
			export CI_COMMIT_REF_NAME=$(CI_COMMIT_BRANCH)
		else
			export CI_COMMIT_REF_NAME=$(CI_COMMIT_TAG)
		endif
 	endif
  ifeq ($(CI_COMMIT_REF_SLUG),)
 		export CI_COMMIT_REF_SLUG=$(shell echo $(CI_COMMIT_BRANCH) | cut -c -63 | sed -E 's/[^a-z0-9-]+/-/g' | sed -E 's/^-*([a-z0-9-]+[a-z0-9])-*$$/\1/g')
 	endif
  ifeq ($(DECKHOUSE_REGISTRY_HOST),)
 		export DECKHOUSE_REGISTRY_HOST=registry.deckhouse.io
  endif
	export WERF_REPO=$(DEV_REGISTRY_PATH)
	export REGISTRY_SUFFIX=$(shell echo $(WERF_ENV) | tr '[:upper:]' '[:lower:]')
	export SECONDARY_REPO=--secondary-repo $(DECKHOUSE_REGISTRY_HOST)/deckhouse/$(REGISTRY_SUFFIX)

build: set-build-envs ## Build Deckhouse images.
	##~ Options: FOCUS=image-name
	werf build --parallel=true --parallel-tasks-limit=5 --platform linux/amd64 --report-path images_tags_werf.json $(SECONDARY_REPO) $(FOCUS)
  ifeq ($(FOCUS),)
    ifneq ($(CI_COMMIT_BRANCH),)
				@# CI_COMMIT_REF_SLUG is a 'prNUM' for dev branches or 'main' for default branch.
				@# Use it as image tag. Add suffix to not overlap with PRs in main repo.
				SRC=$(shell jq -r '.Images."dev".DockerImageName' images_tags_werf.json) && \
				DST=$(DEV_REGISTRY_PATH):$(CI_COMMIT_REF_SLUG) && \
				echo "⚓️ 💫 [$(date -u)] Publish images to dev-registry for branch '$(CI_COMMIT_BRANCH)' and edition '$(WERF_ENV)' using tag '$(CI_COMMIT_REF_SLUG)' ..." && \
				echo "⚓️ 💫 [$(date -u)] Publish 'dev' image to dev-registry using tag '$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true

				SRC=$(shell jq -r '.Images."dev/install".DockerImageName' images_tags_werf.json) && \
  			DST=$(DEV_REGISTRY_PATH)/install:$(CI_COMMIT_REF_SLUG) && \
  			echo "⚓️ 💫 [$(date -u)] Publish 'dev/install' image to dev-registry using tag '$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true

				SRC=$(shell jq -r '.Images."dev/install-standalone".DockerImageName' images_tags_werf.json) && \
				DST=$(DEV_REGISTRY_PATH)/install-standalone:$(CI_COMMIT_REF_SLUG) && \
				echo "⚓️ 💫 [$(date -u)] Publish 'dev/install-standalone' image to dev-registry using tag '$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true

				SRC="$(shell jq -r '.Images."e2e-terraform".DockerImageName' images_tags_werf.json)" && \
				DST="$(DEV_REGISTRY_PATH)/e2e-terraform:$(CI_COMMIT_REF_SLUG)" && \
				echo "⚓️ 💫 [$(date -u)] Publish 'e2e-terraform' image to dev-registry using tag '$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true
    endif
  endif
