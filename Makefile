export PATH := $(abspath bin/):${PATH}

FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

FOCUS =

MDLINTER_IMAGE = ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476
SPELLCHECKER_IMAGE = registry.deckhouse.io/base_images/hunspell:1.7.0-r1-alpine@sha256:f419f1dc5b55cd9c0038ece60612549e64333bb0a0e7d4764d45ed94336dec9c

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
	GH_PLATFORM = linux
else ifeq ($(OS_NAME), Darwin)
	JQ_PLATFORM = osx-amd64
	YQ_PLATFORM = darwin
	TRDL_PLATFORM = darwin
	GH_PLATFORM = macOS
endif
JQ_VERSION = 1.6

# Set arch for deps
ifeq ($(PLATFORM_NAME), x86_64)
	YQ_ARCH = amd64
	CRANE_ARCH = x86_64
	TRDL_ARCH = amd64
	CRANE_ARCH = x86_64
	GH_ARCH = amd64
else ifeq ($(PLATFORM_NAME), aarch64)
	YQ_ARCH = amd64
	CRANE_ARCH = x86_64
	TRDL_ARCH = amd64
	CRANE_ARCH = x86_64
	GH_ARCH = amd64
else ifeq ($(PLATFORM_NAME), arm64)
	YQ_ARCH = arm64
	CRANE_ARCH = arm64
	TRDL_ARCH = arm64
	CRANE_ARCH = arm64
	GH_ARCH = arm64
endif

# Set testing path for tests-modules
ifeq ($(FOCUS),)
	TESTS_PATH = ./modules/... ./global-hooks/... ./ee/modules/... ./ee/fe/modules/... ./ee/be/modules/... ./ee/se/modules/... ./ee/se-plus/modules/...
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
	SE_PLUS_MODULES = $(shell find ./ee/se-plus/modules -maxdepth 1 -regex ".*[0-9]-${FOCUS}")
	ifneq ($(FE_MODULES),)
		SE_PLUS_MODULES_RECURSE = ${SE_PLUS_MODULES}/...
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


GOLANGCI_VERSION = 2.1.2
TRIVY_VERSION= 0.63.0
PROMTOOL_VERSION = 2.37.0
GATOR_VERSION = 3.9.0
GH_VERSION = 2.52.0
TESTS_TIMEOUT="15m"

##@ General

deps: bin/golangci-lint bin/trivy bin/regcopy bin/jq bin/yq bin/crane bin/promtool bin/gator bin/werf bin/gh ## Install dev dependencies.

##@ Security
bin:
	mkdir -p bin

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

.PHONY: bin/yq
bin/yq: bin ## Install yq deps for update-patchversion script.
	curl -sSfL https://github.com/mikefarah/yq/releases/download/v4.25.3/yq_$(YQ_PLATFORM)_$(YQ_ARCH) -o bin/yq && chmod +x bin/yq

.PHONY: tests-modules dmt-lint tests-openapi tests-controller tests-webhooks
tests-modules: ## Run unit tests for modules hooks and templates.
  ##~ Options: FOCUS=module-name
	go test -cover -race -timeout=${TESTS_TIMEOUT} -vet=off ${TESTS_PATH}

dmt-lint:
	export DMT_METRICS_URL="${DMT_METRICS_URL}"
	export DMT_METRICS_TOKEN="${DMT_METRICS_TOKEN}"
	docker run --rm -v ${PWD}:/deckhouse-src -e DMT_METRICS_URL="${DMT_METRICS_URL}" -e DMT_METRICS_TOKEN="${DMT_METRICS_TOKEN}" --user $(id -u):$(id -g) ubuntu /deckhouse-src/tools/dmt-lint.sh


tests-openapi: ## Run tests against modules openapi values schemas.
	go test -timeout=${TESTS_TIMEOUT} -vet=off ./testing/openapi_cases/

tests-controller: ## Run deckhouse-controller unit tests.
	go test -timeout=${TESTS_TIMEOUT} -cover -race ./deckhouse-controller/... -v

tests-webhooks: bin/yq ## Run python webhooks unit tests.
	./testing/webhooks/run.sh

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

lint-src-artifact: set-build-envs ## Run src-artifact stapel linter
	@bin/werf config render | awk 'NR!=1 {print}' | go run ./tools/lint-src-artifact/lint-src-artifact.go

##@ Generate

## Run all generate-* jobs in bulk.
.PHONY: generate render-workflow
generate: generate-kubernetes generate-tools

.PHONY: generate-tools
generate-tools:
	cd tools && go generate -v && cd ..

render-workflow: ## Generate CI workflow instructions.
	./.github/render-workflows.sh

bin/regcopy: bin ## App to copy docker images to the Deckhouse registry
	cd tools/regcopy; go build -o bin/regcopy

bin/trivy-${TRIVY_VERSION}/trivy:
	mkdir -p bin/trivy-${TRIVY_VERSION}
	curl ${DECKHOUSE_PRIVATE_REPO}/api/v4/projects/${TRIVY_PROJECT_ID}/packages/generic/trivy-v${TRIVY_VERSION}/v${TRIVY_VERSION}/trivy -o bin/trivy-${TRIVY_VERSION}/trivy

.PHONY: trivy
bin/trivy: bin bin/trivy-${TRIVY_VERSION}/trivy
	rm -rf bin/trivy
	chmod u+x bin/trivy-${TRIVY_VERSION}/trivy
	ln -s ${PWD}/bin/trivy-${TRIVY_VERSION}/trivy bin/trivy

.PHONY: cve-report
cve-report: bin/trivy bin/jq ## Generate CVE report for a Deckhouse release.
  ##~ Options: SEVERITY=CRITICAL,HIGH REPO=registry.deckhouse.io TAG=v1.30.0
	./tools/cve/d8_images_cve_scan.sh

cve-base-images-check-default-user: bin/jq ## Check CVE in our base images.
  ##~ Options: SEVERITY=CRITICAL,HIGH
	./tools/cve/check-non-root.sh

##@ Documentation

.PHONY: docs
docs: bin/werf ## Run containers with the documentation.
	cd docs/site/; ../../bin/werf compose up --docker-compose-command-options='-d' --env local --repo ":local" --skip-image-spec-stage=true
	echo "Open http://localhost to access the documentation..."

.PHONY: docs-dev
docs-dev: bin/werf ## Run containers with the documentation in the dev mode (allow uncommited files).
	export DOC_API_URL=dev
	export DOC_API_KEY=dev
	cd docs/site/; ../../bin/werf compose up --docker-compose-command-options='-d' --dev --env development --repo ":local" --skip-image-spec-stage=true
	echo "Open http://localhost to access the documentation..."

.PHONY: docs-down
docs-down: ## Stop all the documentation containers (e.g. site_site_1 - for Linux, and site-site-1 for MacOs)
	docker rm -f site-site-1 site_site_1 site-router-1  site_router_1  site-front-1 site_front_1 site-frontend-1 site_frontend_1 2>/dev/null || true ; docker network rm deckhouse 2>/dev/null || true

.PHONY: tests-doc-links
docs-linkscheck: ## Build documentation and run checker of html links.
	@bash tools/docs/link-checker/run.sh

.PHONY: docs-spellcheck
docs-spellcheck: ## Check the spelling in the documentation.
  ##~ Options: file="path/to/file" (Specify a path to a specific file)
	@docker run --rm -v ${PWD}:/spelling -v ${PWD}/tools/docs/spelling:/app ${SPELLCHECKER_IMAGE} /app/spell_check.sh -f $(file)

lint-doc-spellcheck-pr:
	@docker run --rm -v ${PWD}:/spelling -v ${PWD}/tools/docs/spelling:/app ${SPELLCHECKER_IMAGE} /app/check_diff.sh

.PHONY: docs-spellcheck-generate-dictionary
docs-spellcheck-generate-dictionary: ## Generate a dictionary (run it after adding new words to the tools/docs/spelling/wordlist file).
	@echo "Sorting wordlist..."
	@sort ./tools/docs/spelling/wordlist -o ./tools/docs/spelling/wordlist
	@echo "Generating dictionary..."
	@test -f ./tools/docs/spelling/dictionaries/dev_OPS.dic && rm ./tools/docs/spelling/dictionaries/dev_OPS.dic
	@touch ./tools/docs/spelling/dictionaries/dev_OPS.dic
	@cat ./tools/docs/spelling/wordlist | wc -l | sed 's/^[ \t]*//g' > ./tools/docs/spelling/dictionaries/dev_OPS.dic
	@sort ./tools/docs/spelling/wordlist >> ./tools/docs/spelling/dictionaries/dev_OPS.dic
	@echo "Don't forget to commit changes and push it!"
	@git diff --stat

.PHONY: docs-spellcheck-get-typos-list
docs-spellcheck-get-typos-list: ## Print out a list of all the terms in all pages that were considered as a typo.
	@echo "Please wait a bit. It may take about 20 minutes and there may be no output in the terminal..." && \
	docker run --rm -v ${PWD}:/spelling --entrypoint /bin/bash -v ${PWD}/tools/docs/spelling:/app ${SPELLCHECKER_IMAGE} -c "/app/spell_check.sh 2>/dev/null | sed '/Spell-checking the documentation/ d; /^Possible typos/d' | sort -u"

##@ Update kubernetes control-plane patchversions

bin/jq-$(JQ_VERSION)/jq:
	mkdir -p bin/jq-$(JQ_VERSION)
	curl -sSfL https://github.com/stedolan/jq/releases/download/jq-$(JQ_VERSION)/jq-$(JQ_PLATFORM) -o $(PWD)/bin/jq-$(JQ_VERSION)/jq && chmod +x $(PWD)/bin/jq-$(JQ_VERSION)/jq

.PHONY: bin/jq
bin/jq: bin bin/jq-$(JQ_VERSION)/jq ## Install jq deps for update-patchversion script.
	rm -f bin/jq
	ln -s jq-$(JQ_VERSION)/jq bin/jq

bin/crane: bin ## Install crane deps for update-patchversion script.
	curl -sSfL https://github.com/google/go-containerregistry/releases/download/v0.10.0/go-containerregistry_$(OS_NAME)_$(CRANE_ARCH).tar.gz | tar -xzf - crane && mv crane bin/crane && chmod +x bin/crane

bin/trdl: bin
	@if ! command -v bin/trdl >/dev/null 2>&1; then \
		curl -sSfL https://tuf.trdl.dev/targets/releases/0.7.0/$(TRDL_PLATFORM)-$(TRDL_ARCH)/bin/trdl -o bin/trdl; \
		chmod +x bin/trdl; \
	fi

bin/werf: bin bin/trdl ## Install werf for images-digests generator.
		@bash -c 'bin/trdl --home-dir bin/.trdl add werf https://tuf.werf.io 1 b7ff6bcbe598e072a86d595a3621924c8612c7e6dc6a82e919abe89707d7e3f468e616b5635630680dd1e98fc362ae5051728406700e6274c5ed1ad92bea52a2';
		@if command -v bin/werf >/dev/null 2>&1; then \
			bin/trdl --home-dir bin/.trdl --no-self-update=true update --in-background werf 2 alpha; \
		else \
			bin/trdl --home-dir bin/.trdl --no-self-update=true update werf 2 alpha; \
			ln -sf $$(bin/trdl --home-dir bin/.trdl bin-path werf 2 alpha | sed 's|^.*/bin/\(.trdl.*\)|\1/werf|') bin/werf; \
		fi;

bin/gh: bin ## Install gh cli.
	curl -sSfL https://github.com/cli/cli/releases/download/v$(GH_VERSION)/gh_$(GH_VERSION)_$(GH_PLATFORM)_$(GH_ARCH).tar.gz -o bin/gh.tar.gz
	tar zxf bin/gh.tar.gz -C bin/ && ln -s bin/gh_$(GH_VERSION)_$(GH_PLATFORM)_$(GH_ARCH)/bin/gh bin/gh
	rm bin/gh.tar.gz

.PHONY: update-k8s-patch-versions
update-k8s-patch-versions: ## Run update-patchversion script to generate new version_map.yml.
	cd candi/tools; bash update_kubernetes_patchversions.sh

##@ Lib helm
.PHONY: update-lib-helm
update-lib-helm: yq ## Update lib-helm.
	##~ Options: version=MAJOR.MINOR.PATCH
	cd helm_lib/ && yq -i '.dependencies[0].version = "$(version)"' Chart.yaml && helm dependency update && tar -xf charts/deckhouse_lib_helm-*.tgz -C charts/ && rm charts/deckhouse_lib_helm-*.tgz && git add Chart.yaml Chart.lock charts/*

.PHONY: update-base-images-versions
update-base-images-versions:
	##~ Options: version=vMAJOR.MINOR.PATCH
	cd candi && curl --fail -sSLO https://fox.flant.com/api/v4/projects/deckhouse%2Fbase-images/packages/generic/base_images/$(version)/base_images.yml

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
  ifeq ($(CLOUD_PROVIDERS_SOURCE_REPO),)
 		export CLOUD_PROVIDERS_SOURCE_REPO=https://github.com
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
 		export CI_COMMIT_REF_SLUG=$(shell bin/gh pr view $$CI_COMMIT_BRANCH --json number -q .number 2>/dev/null)
 	endif
  ifeq ($(DECKHOUSE_REGISTRY_HOST),)
 		export DECKHOUSE_REGISTRY_HOST=registry.deckhouse.io
  endif
  ifeq ($(OBSERVABILITY_SOURCE_REPO),)
  	export OBSERVABILITY_SOURCE_REPO=https://example.com
  endif
  ifeq ($(DECKHOUSE_PRIVATE_REPO),)
  	export DECKHOUSE_PRIVATE_REPO=https://github.com
  endif

	export WERF_REPO=$(DEV_REGISTRY_PATH)
	export REGISTRY_SUFFIX=$(shell echo $(WERF_ENV) | tr '[:upper:]' '[:lower:]')
	export SECONDARY_REPO=--secondary-repo $(DECKHOUSE_REGISTRY_HOST)/deckhouse/$(REGISTRY_SUFFIX)

build: bin/werf set-build-envs ## Build Deckhouse images.
	##~ Options: FOCUS=image-name
	bin/werf build --parallel=true --parallel-tasks-limit=5 --platform linux/amd64 --save-build-report=true --build-report-path images_tags_werf.json $(SECONDARY_REPO) $(FOCUS)
  ifeq ($(FOCUS),)
    ifneq ($(CI_COMMIT_REF_SLUG),)
				@# By default in the Github CI_COMMIT_REF_SLUG is a 'prNUM' for dev branches.
				SRC=$(shell jq -r '.Images."dev".DockerImageName' images_tags_werf.json) && \
				DST=$(DEV_REGISTRY_PATH):pr$(CI_COMMIT_REF_SLUG) && \
				echo "⚓️ 💫 [$(date -u)] Publish images to dev-registry for branch '$(CI_COMMIT_BRANCH)' and edition '$(WERF_ENV)' using tag '$(CI_COMMIT_REF_SLUG)' ..." && \
				echo "⚓️ 💫 [$(date -u)] Publish 'dev' image to dev-registry using tag 'pr$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true

				SRC=$(shell jq -r '.Images."dev/install".DockerImageName' images_tags_werf.json) && \
  			DST=$(DEV_REGISTRY_PATH)/install:pr$(CI_COMMIT_REF_SLUG) && \
  			echo "⚓️ 💫 [$(date -u)] Publish 'dev/install' image to dev-registry using tag 'pr$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true

				SRC=$(shell jq -r '.Images."dev/install-standalone".DockerImageName' images_tags_werf.json) && \
				DST=$(DEV_REGISTRY_PATH)/install-standalone:pr$(CI_COMMIT_REF_SLUG) && \
				echo "⚓️ 💫 [$(date -u)] Publish 'dev/install-standalone' image to dev-registry using tag 'pr$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true

				SRC="$(shell jq -r '.Images."e2e-opentofu-eks".DockerImageName' images_tags_werf.json)" && \
				DST="$(DEV_REGISTRY_PATH)/e2e-opentofu-eks:pr$(CI_COMMIT_REF_SLUG)" && \
				echo "⚓️ 💫 [$(date -u)] Publish 'e2e-opentofu-eks' image to dev-registry using tag 'pr$(CI_COMMIT_REF_SLUG)'" && \
				docker pull $$SRC && \
				docker image tag $$SRC $$DST && \
				docker image push $$DST && \
				docker image rmi $$DST || true
    endif
  endif

build-render: set-build-envs ## render werf.yaml for build Deckhouse images.
	bin/werf config render --dev

GO=$(shell which go)
GIT=$(shell which git)
GOLANGCI_LINT=$(shell which golangci-lint)

.PHONY: go-check
go-check:
	$(call error-if-empty,$(GO),go)

.PHONY: go-module-version
go-module-version: go-check
	@echo "go get $(shell go list ./deckhouse-controller/cmd/deckhouse-controller)@$(shell git rev-parse HEAD)"

.PHONY: all-mod
all-mod: go-check
	@for dir in $$(find . -mindepth 2 -name go.mod | sed -r 's/(.*)(go.mod)/\1/g'); do \
		echo "Running go mod tidy in $${dir}"; \
		cd $(CURDIR)/$${dir} && go mod tidy && cd $(CURDIR); \
	done

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
DECKHOUSE_CLI ?= $(LOCALBIN)/d8
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CLIENT_GEN ?= $(LOCALBIN)/client-gen
INFORMER_GEN ?= $(LOCALBIN)/informer-gen
LISTER_GEN ?= $(LOCALBIN)/lister-gen
YQ = $(LOCALBIN)/yq

## Tool Versions
GO_TOOLCHAIN_AUTOINSTALL_VERSION ?= go1.24.9
DECKHOUSE_CLI_VERSION ?= v0.24.2
CONTROLLER_TOOLS_VERSION ?= v0.18.0
CODE_GENERATOR_VERSION ?= v0.32.10
YQ_VERSION ?= v4.47.2

## Generate tools documentation
.PHONY: generate-docs
generate-docs: deckhouse-cli ## Generate documentation for deckhouse-cli.
	@$(DECKHOUSE_CLI) help-json > ./docs/documentation/_data/reference/d8-cli.json && echo "d8 help-json content is updated"

## Generate codebase for deckhouse-controllers kubernetes entities
.PHONY: generate-kubernetes
generate-kubernetes: controller-gen-generate client-gen-generate lister-gen-generate informer-gen-generate

## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
.PHONY: controller-gen-generate
controller-gen-generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="./deckhouse-controller/hack/boilerplate.go.txt" paths="./deckhouse-controller/pkg/apis/..."

.PHONY: manifests 
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@echo "Generating CRDs..."
	@$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./deckhouse-controller/pkg/apis/deckhouse.io/..." output:crd:artifacts:config=bin/crd/bases

## Generate clientset
.PHONY: client-gen-generate
client-gen-generate: client-gen
	$(CLIENT_GEN) \
		--clientset-name "versioned" \
		--input-base "" \
		--input "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1,github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2" \
		--output-pkg "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset" \
		--output-dir "./deckhouse-controller/pkg/client/clientset" \
		--go-header-file "./deckhouse-controller/hack/boilerplate.go.txt"

## Generate listers (required for informers)
.PHONY: lister-gen-generate
lister-gen-generate: lister-gen
	$(LISTER_GEN) \
		--output-pkg "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/listers" \
		--output-dir "./deckhouse-controller/pkg/client/listers" \
		--go-header-file "./deckhouse-controller/hack/boilerplate.go.txt" \
		github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1 \
		github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2

## Generate informers
.PHONY: informer-gen-generate
informer-gen-generate: informer-gen lister-gen-generate client-gen-generate
	$(INFORMER_GEN) \
		--versioned-clientset-package "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned" \
		--listers-package "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/listers" \
		--output-pkg "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/informers" \
		--output-dir "./deckhouse-controller/pkg/client/informers" \
		--go-header-file "./deckhouse-controller/hack/boilerplate.go.txt" \
		github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1 \
		github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2

## Tool installations

## Download deckhouse-cli locally if necessary.
.PHONY: deckhouse-cli
deckhouse-cli:
	@if [ -f "$(DECKHOUSE_CLI)" ]; then \
		CURRENT_VERSION=$$($(DECKHOUSE_CLI) --version 2>/dev/null | head -n1 | awk '{print $$3}' || echo "unknown"); \
		if [ "$$CURRENT_VERSION" != "$(DECKHOUSE_CLI_VERSION)" ]; then \
			echo "Current d8 version ($$CURRENT_VERSION) does not match required version ($(DECKHOUSE_CLI_VERSION)), downloading new binary..."; \
			INSTALL_DIR=$(LOCALBIN) VERSION=$(DECKHOUSE_CLI_VERSION) FORCE=yes sh -c "$$(curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/tools/install.sh)" >/dev/null 2>&1; \
		else \
			echo "d8 version $(DECKHOUSE_CLI_VERSION) is already installed."; \
		fi; \
	else \
		echo "d8 not found, downloading..."; \
		INSTALL_DIR=$(LOCALBIN) VERSION=$(DECKHOUSE_CLI_VERSION) FORCE=yes sh -c "$$(curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/tools/install.sh)" >/dev/null 2>&1; \
	fi

## Download client-gen locally if necessary.
.PHONY: client-gen
client-gen: $(CLIENT_GEN)
$(CLIENT_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen,$(CODE_GENERATOR_VERSION))

## Download lister-gen locally if necessary.
.PHONY: lister-gen
lister-gen: $(LISTER_GEN)
$(LISTER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(LISTER_GEN),k8s.io/code-generator/cmd/lister-gen,$(CODE_GENERATOR_VERSION))

## Download informer-gen locally if necessary.
.PHONY: informer-gen
informer-gen: $(INFORMER_GEN)
$(INFORMER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(INFORMER_GEN),k8s.io/code-generator/cmd/informer-gen,$(CODE_GENERATOR_VERSION))

## Download controller-gen locally if necessary.
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4,$(YQ_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) GOTOOLCHAIN=$(GO_TOOLCHAIN_AUTOINSTALL_VERSION) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

define error-if-empty
@if [[ -z $(1) ]]; then echo "$(2) not installed"; false; fi
endef
