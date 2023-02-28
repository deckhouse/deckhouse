GO_VERSION=1.16.3
GOLANGCI_LINT_VERSION=1.39.0
GOFUMPT_VERSION=0.1.1
GOARCH=amd64
UNAME=$(shell uname -s)

GOFUMPT_BIN=/tmp/gofumpt-$(GOFUMPT_VERSION)
GOLANGCILINT_BIN=/tmp/golangci-lint-$(GOLANGCI_LINT_VERSION)

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif


.PHONY : deps fmt lint build build-docker all

all: deps fmt build lint

deps:
	go mod tidy
	@test -f $(GOFUMPT_BIN)      || curl -sLo $(GOFUMPT_BIN) https://github.com/mvdan/gofumpt/releases/download/v$(GOFUMPT_VERSION)/gofumpt_v$(GOFUMPT_VERSION)_$(OS)_$(GOARCH)
	@test -f $(GOLANGCILINT_BIN) || curl -sfL https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(GOARCH).tar.gz | tar -xzOf - golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(GOARCH)/golangci-lint > $(GOLANGCILINT_BIN)
	@chmod +x $(GOLANGCILINT_BIN) $(GOFUMPT_BIN)

lint:
	$(GOLANGCILINT_BIN) run ./... -c .golangci.yaml

fmt:
	@# - gofumpt is not included in the .golangci.yaml because it conflicts with imports https://github.com/golangci/golangci-lint/issues/1490#issuecomment-778782810
	@# - goimports is not turned on since it is used mostly by gofumpt internally
	$(GOFUMPT_BIN) -l -w -extra .
	$(GOLANGCILINT_BIN) run ./... -c .golangci.yaml --fix

build:
	GOOS="$(OS)" GOARCH="$(GOARCH)" go build -ldflags="-s -w" -o /tmp/smoke-mini smoke-mini.go

build-docker:
	docker build \
		--build-arg BASE_ALPINE=alpine:3.12 \
		--build-arg BASE_GOLANG_ALPINE=golang:$(GO_VERSION)-alpine3.12 \
		--build-arg BASE_GOLANG_BUSTER=golang:$(GO_VERSION)-buster \
		-t smoke-mini \
		.
