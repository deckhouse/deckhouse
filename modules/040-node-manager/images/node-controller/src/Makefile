# Build variables
IMG ?= registry.deckhouse.io/deckhouse/node-controller:latest
BINARY_NAME ?= node-controller
GOBIN ?= $(shell go env GOPATH)/bin

# Location for locally-installed tooling (envtest assets, setup-envtest).
LOCALBIN ?= $(shell pwd)/bin
ENVTEST ?= $(LOCALBIN)/setup-envtest
# Kubebuilder assets version for the envtest integration suite.
ENVTEST_K8S_VERSION ?= 1.34.1
# gotestsum renders go test -json into readable output. Pinned to match the repo-root Makefile.
GOTESTSUM ?= $(LOCALBIN)/gotestsum
GOTESTSUM_VERSION ?= v1.13.0
# Output format: pkgname = one line per package (clean); override e.g. GOTESTSUM_FORMAT=standard-verbose to debug.
GOTESTSUM_FORMAT ?= pkgname

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
test: envtest gotestsum ## Run the whole unit + envtest suite with coverage (CI: GOTESTSUM_FORMAT=github-actions).
	@echo "==> [test] running gotestsum --format $(GOTESTSUM_FORMAT) (unit + envtest)..."
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		$(GOTESTSUM) --format $(GOTESTSUM_FORMAT) --format-hide-empty-pkg -- ./... -coverprofile cover.out
	@echo "==> [test] total coverage:"
	@go tool cover -func=cover.out | tail -1

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

.PHONY: gotestsum
gotestsum: $(GOTESTSUM) ## Install gotestsum locally if necessary.

$(GOTESTSUM): $(LOCALBIN)
	@test -s $(GOTESTSUM) || { \
		echo "==> [gotestsum] installing gotestsum $(GOTESTSUM_VERSION)..."; \
		GOBIN=$(LOCALBIN) go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); \
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
