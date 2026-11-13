BINARY_NAME ?= fencing-agent
GOBIN ?= $(shell go env GOPATH)/bin
CONTROLLER_GEN = $(GOBIN)/controller-gen
CONTROLLER_GEN_VERSION ?= v0.19.0

.PHONY: build
build: ## Build the agent binary.
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd/main.go

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
test: ## Run unit tests with the race detector.
	go test -race ./...

.PHONY: controller-gen
controller-gen: ## Install controller-gen locally if necessary.
	@test -f $(CONTROLLER_GEN) || GOBIN=$(GOBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
