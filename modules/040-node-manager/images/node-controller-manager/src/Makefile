# Image URL to use all building/pushing image targets
IMG ?= node-controller:latest
REGISTRY ?= registry.deckhouse.io/deckhouse/ce

# Go parameters
GOOS ?= linux
GOARCH ?= amd64

# Get the currently used golang install path
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: test
test: fmt vet ## Run tests
	go test ./... -coverprofile cover.out

.PHONY: test-verbose
test-verbose: fmt vet ## Run tests with verbose output
	go test -v ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build controller binary
	go build -o bin/controller cmd/controller/main.go

.PHONY: run
run: fmt vet ## Run controller from host
	go run cmd/controller/main.go

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image
	docker push ${IMG}

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push docker image

##@ Code Generation

.PHONY: generate
generate: controller-gen ## Generate code (DeepCopy, etc.)
	$(CONTROLLER_GEN) object paths="./api/..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) crd webhook paths="./..." output:crd:artifacts:config=config/crd

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the cluster
	kubectl apply -f config/crd/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the cluster
	kubectl delete -f config/crd/

.PHONY: deploy
deploy: manifests ## Deploy controller to the cluster
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/
	kubectl apply -f config/webhook/

.PHONY: undeploy
undeploy: ## Undeploy controller from the cluster
	kubectl delete -f config/webhook/
	kubectl delete -f config/manager/
	kubectl delete -f config/rbac/

##@ Tools

CONTROLLER_GEN = $(GOBIN)/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen if necessary
	@test -f $(CONTROLLER_GEN) || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

GOLANGCI_LINT = $(GOBIN)/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint if necessary
	@test -f $(GOLANGCI_LINT) || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

##@ Helpers

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: vendor
vendor: ## Run go mod vendor
	go mod vendor

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f cover.out
