BINARY_NAME ?= fencing-controller
GOBIN ?= $(shell go env GOPATH)/bin
CONTROLLER_GEN = $(GOBIN)/controller-gen
CONTROLLER_GEN_VERSION ?= v0.19.0
LOCALBIN ?= $(shell pwd)/bin
ENVTEST ?= $(LOCALBIN)/setup-envtest
# The envtest tests use the binaries of this Kubernetes version.
ENVTEST_K8S_VERSION ?= 1.34.1

.PHONY: build
build: ## Build the controller binary.
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods for API types.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

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
test: envtest ## Run unit and envtest tests with the race detector.
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -race ./...

.PHONY: envtest
envtest: ## Install setup-envtest and the envtest binaries (etcd, kube-apiserver, kubectl).
	@test -s $(ENVTEST) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path

.PHONY: controller-gen
controller-gen: ## Install controller-gen locally if necessary.
	@test -f $(CONTROLLER_GEN) || GOBIN=$(GOBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
