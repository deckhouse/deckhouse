GO_VERSION=1.16.3
GOLANGCI_LINT_VERSION=1.39.0
GOFUMPT_VERSION=0.1.1
GOARCH = amd64
UNAME = $(shell uname -s)

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif


.PHONY : deps fmt lint build build-docker test all

all: deps fmt build test

deps:
	go mod tidy
	test -f ./bin/golangci-lint || curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | BINARY=golangci-lint bash -s -- v$(GOLANGCI_LINT_VERSION)
	test -f ./bin/gofumpt 	    || curl -sLo ./bin/gofumpt https://github.com/mvdan/gofumpt/releases/download/v$(GOFUMPT_VERSION)/gofumpt_v$(GOFUMPT_VERSION)_$(OS)_$(GOARCH)
	chmod +x ./bin/*

lint:
	./bin/golangci-lint run ./... -c .golangci.yaml

fmt:
	@# - gofumpt is not included in the .golangci.yaml because it conflicts with imports https://github.com/golangci/golangci-lint/issues/1490#issuecomment-778782810
	@# - goimports is not turned on since it is used mostly by gofumpt internally
	./bin/gofumpt -l -w -extra .
	./bin/golangci-lint run ./... -c .golangci.yaml --fix

test:
	go test -race ./...

build:
	mkdir -p bin
	GOOS="$(OS)" GOARCH="$(GOARCH)" go build -ldflags="-s -w" -o ./bin/upmeter ./cmd/upmeter/

build-docker:
	docker build \
		--build-arg BASE_ALPINE=alpine:3.12 \
		--build-arg BASE_GOLANG_ALPINE=golang:$(GO_VERSION)-alpine3.12 \
		--build-arg BASE_GOLANG_BUSTER=golang:$(GO_VERSION)-buster \
		-t upmeter \
		.

clean:
	rm -rf bin
