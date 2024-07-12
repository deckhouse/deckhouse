# https://github.com/golangci/golangci-lint
GOLANGCI_LINT_VERSION=1.58.0
# https://github.com/mvdan/gofumpt
GOFUMPT_VERSION=0.6.0

ARCH_NAME := $(shell uname -m)
OS_NAME := $(shell uname)
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


.PHONY : deps fmt lint build build-docker test ci all

all: deps fmt build test

deps:
	go mod tidy
	@test -f $(GOFUMPT_BIN)      || curl -sLo $(GOFUMPT_BIN) https://github.com/mvdan/gofumpt/releases/download/v$(GOFUMPT_VERSION)/gofumpt_v$(GOFUMPT_VERSION)_$(OS)_$(ARCH_NAME)
	@test -f $(GOLANGCILINT_BIN) || curl -sfL https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH_NAME).tar.gz | tar -xzOf - golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH_NAME)/golangci-lint > $(GOLANGCILINT_BIN)
	@chmod +x $(GOLANGCILINT_BIN) $(GOFUMPT_BIN)

lint:
	$(GOLANGCILINT_BIN) run ./... -c .golangci.yaml

fmt:
	@# - gofumpt is not included in the .golangci.yaml because it conflicts with imports https://github.com/golangci/golangci-lint/issues/1490#issuecomment-778782810
	@# - goimports is not turned on since it is used mostly by gofumpt internally
	$(GOFUMPT_BIN) -l -w -extra .
	goimports -w -local d8.io/upmeter .
	$(GOLANGCILINT_BIN) run ./... -c .golangci.yaml --fix

test:
	go test -race ./...

ci: deps lint
	tmpfile=$(mktemp /tmp/coverage-report.XXXXXX)
	go test -cover -coverprofile=${tmpfile} -vet=off ./pkg/... \
        && echo "Coverage: $(go tool cover -func  ${tmpfile} | grep total | awk '{print $3}')" \
        && echo "Success!" \
        || exit 1

build:
	GOOS="$(OS)" GOARCH="$(ARCH_NAME)" go build -ldflags="-s -w" -o /tmp/upmeter ./cmd/upmeter/

build-docker:
	docker build \
		--build-arg BASE_ALPINE=alpine:3.12 \
		--build-arg BASE_GOLANG_ALPINE=golang:$(GO_VERSION)-alpine3.12 \
		--build-arg BASE_GOLANG_BUSTER=golang:$(GO_VERSION)-buster \
		-t upmeter \
		.
