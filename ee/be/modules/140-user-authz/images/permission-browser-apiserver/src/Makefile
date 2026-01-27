# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

BINARY_NAME=permission-browser-apiserver
GO=go

.PHONY: all build clean test generate fmt vet

all: generate build

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BINARY_NAME) .

build-local:
	$(GO) build -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)

test:
	$(GO) test -v ./...

generate:
	./hack/update-codegen.sh

verify:
	./hack/verify-codegen.sh

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

mod-download:
	$(GO) mod download

mod-tidy:
	$(GO) mod tidy

lint:
	golangci-lint run --config=.golangci.yaml ./...
