# Build variables
IMG ?= registry.deckhouse.io/deckhouse/node-controller:latest
BINARY_NAME ?= node-controller
GOBIN ?= $(shell go env GOPATH)/bin

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
test: ## Run tests.
	go test ./... -coverprofile cover.out

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
