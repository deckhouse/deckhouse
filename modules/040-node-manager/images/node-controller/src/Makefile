# Build variables
IMG ?= registry.deckhouse.io/deckhouse/node-controller:latest
BINARY_NAME ?= node-controller
GOBIN ?= $(shell go env GOPATH)/bin

# Location for locally-installed tooling (envtest assets, setup-envtest).
LOCALBIN ?= $(shell pwd)/bin
ENVTEST ?= $(LOCALBIN)/setup-envtest
# Kubebuilder assets version for the envtest integration suite.
ENVTEST_K8S_VERSION ?= 1.34.1
# gotestfmt (GoTestTools) renders go test -json into GitHub Actions collapsible groups.
GOTESTFMT ?= $(LOCALBIN)/gotestfmt
GOTESTFMT_VERSION ?= v2.5.0
# CI sets GOTESTFMT_CI=github to emit ::group:: collapsible output; empty locally for plain output.
GOTESTFMT_CI ?=
# Raw go test -json log, captured to a file so the go test exit code survives (gotestfmt is only a
# formatter; piping into it would otherwise mask a test failure behind gotestfmt's exit code).
GOTEST_JSON ?= /tmp/node-controller-gotest.json

# Go tools
CONTROLLER_GEN = $(GOBIN)/controller-gen
CONVERSION_GEN = $(GOBIN)/conversion-gen
GOFUMPT = $(GOBIN)/gofumpt

# Build flags
LDFLAGS ?= -s -w

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint against code.
	golangci-lint run ./...

.PHONY: test
test: envtest gotestfmt ## Run unit + envtest with GitHub-friendly output (CI: GOTESTFMT_CI=github).
	@echo "==> [test] running go test -json | gotestfmt (unit + envtest)..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test -json -coverprofile cover.out ./... > $(GOTEST_JSON) 2>&1; rc=$$?; \
		$(GOTESTFMT) $(if $(GOTESTFMT_CI),-ci $(GOTESTFMT_CI),) -hide successful-downloads < $(GOTEST_JSON); \
		echo "==> [test] go test result cache: $$(grep -c '(cached)' $(GOTEST_JSON) 2>/dev/null || true) package(s) reused, the rest re-ran (changing a package's code invalidates only that package + its importers)"; \
		echo "==> [test] total coverage:"; go tool cover -func=cover.out 2>/dev/null | tail -1; \
		rm -f $(GOTEST_JSON); \
		exit $$rc

.PHONY: test-envtest
test-envtest: envtest ## Run every controller's envtest suite (ENVTEST_DEBUG=1 dumps state before/after each spec; ENVTEST_LOGS=1 adds controller logs).
	@echo "==> [test-envtest] running each controller envtest suite live (ENVTEST_DEBUG=1 = state dumps, ENVTEST_LOGS=1 = controller logs)..."
	@assets="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)"; \
	for pkg in $$(grep -rl 'RunSpecs' internal/controller --include=suite_test.go | xargs -n1 dirname | sort -u | sed 's#^#./#'); do \
		echo ""; echo "==> $$pkg"; \
		KUBEBUILDER_ASSETS="$$assets" go test $$pkg -run 'ControllerEnvtest$$' -v -ginkgo.v -count=1 || exit $$?; \
	done
	@echo "==> [test-envtest] done. (run a single suite, e.g. FOCUS, or 'make test' for coverage)"

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/main.go

.PHONY: run
run: generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push $(IMG)

##@ Generate

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./api/..."

.PHONY: generate-conversion
generate-conversion: conversion-gen ## Generate conversion functions.
	$(CONVERSION_GEN) \
		--input-dirs=./api/deckhouse.io/v1alpha1 \
		--input-dirs=./api/deckhouse.io/v1alpha2 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=hack/boilerplate.go.txt

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=node-controller crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

##@ Tools

.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	@test -f $(CONTROLLER_GEN) || GOBIN=$(GOBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

.PHONY: conversion-gen
conversion-gen: ## Download conversion-gen locally if necessary.
	@test -f $(CONVERSION_GEN) || GOBIN=$(GOBIN) go install k8s.io/code-generator/cmd/conversion-gen@latest

.PHONY: gofumpt
gofumpt: ## Download gofumpt locally if necessary.
	@test -f $(GOFUMPT) || GOBIN=$(GOBIN) go install mvdan.cc/gofumpt@latest

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: envtest
envtest: $(ENVTEST) ## Install setup-envtest and download the kubebuilder assets (etcd, kube-apiserver, kubectl).
	@echo "==> [envtest] kubebuilder assets (k8s $(ENVTEST_K8S_VERSION)): $$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)"

$(ENVTEST): $(LOCALBIN)
	@test -s $(ENVTEST) || { \
		echo "==> [envtest] installing setup-envtest..."; \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest; \
	}

.PHONY: gotestfmt
gotestfmt: $(GOTESTFMT) ## Install gotestfmt locally if necessary.

$(GOTESTFMT): $(LOCALBIN)
	@test -s $(GOTESTFMT) || { \
		echo "==> [gotestfmt] installing gotestfmt $(GOTESTFMT_VERSION)..."; \
		GOBIN=$(LOCALBIN) go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@$(GOTESTFMT_VERSION); \
	}

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/bases

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/bases

.PHONY: deploy
deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/
	kubectl apply -f config/webhook/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/webhook/ || true
	kubectl delete -f config/manager/ || true
	kubectl delete -f config/rbac/ || true

##@ Clean

.PHONY: clean
clean: ## Clean build artifacts.
	rm -rf bin/
	rm -f cover.out
