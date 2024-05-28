# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# Dependency versions
GOLANGCI_VERSION = 1.50.1
GOFUMPT_VERSION=0.1.1
PROTOC_VERSION = 25.2
PROTOC_GEN_GO_VERSION = 1.32
PROTOC_GEN_GO_GROC_VERSION = 1.3


# Build versions
GO_VERSION=1.19.3
GOARCH=amd64

DHCTL_BIN_NAME=bin/dhctl

OS_NAME := $(shell uname)
PLATFORM_NAME := $(shell uname -m)
ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif

ifndef D8_VERSION
	D8_VERSION = local
endif

# Set platform for deps
ifndef PROTOC_PLATFORM
	ifeq ($(OS), linux)
		PROTOC_PLATFORM = linux
	else ifeq ($(OS), darwin)
		PROTOC_PLATFORM = osx
	endif
endif

# Set arch for deps
ifndef PROTOC_ARCH
	ifeq ($(PLATFORM_NAME), x86_64)
		PROTOC_ARCH = x86_64
	else ifeq ($(PLATFORM_NAME), arm64)
		PROTOC_ARCH = universal_binary
	endif
endif

export PATH := $(abspath bin/):${PATH}

INSTALLER_IMAGE_URL=dev-registry.deckhouse.io/sys/deckhouse-oss/install:main

.PHONY : deps fmt lint build test all ci devenv devstop

all: deps fmt build test

all-dev: deps fmt build-dev test

bin/golangci-lint:
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | BINARY=golangci-lint bash -s -- v${GOLANGCI_VERSION}

bin/gofumpt:
	@mkdir -p bin
	curl -sLo "bin/gofumpt" https://github.com/mvdan/gofumpt/releases/download/v$(GOFUMPT_VERSION)/gofumpt_v$(GOFUMPT_VERSION)_$(OS)_$(GOARCH)
	@chmod +x "./bin/gofumpt"

.PHONY: bin/protoc
bin/protoc:
	@mkdir -p bin
	curl -sLo "protoc.zip" https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_PLATFORM)-$(PROTOC_ARCH).zip
	unzip -o -j "protoc.zip" "bin/protoc" -d bin 1>/dev/null
	unzip -o "protoc.zip" "include/*" -d . 1>/dev/null
	rm -f "protoc.zip"
	GOBIN=$(abspath bin/) go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(abspath bin/) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GROC_VERSION)

.PHONY: protoc
protoc:
	bin/protoc -I pkg/server/api/dhctl --go_out=pkg/server --go-grpc_out=pkg/server \
	pkg/server/api/dhctl/common.proto \
	pkg/server/api/dhctl/check.proto \
	pkg/server/api/dhctl/bootstrap.proto \
	pkg/server/api/dhctl/destroy.proto \
	pkg/server/api/dhctl/abort.proto \
	pkg/server/api/dhctl/converge.proto \
	pkg/server/api/dhctl/commander_attach.proto \
	pkg/server/api/dhctl/commander_detach.proto \
	pkg/server/api/dhctl/validation.proto \
	pkg/server/api/dhctl/services.proto

generate: protoc
	go generate ./...

build:
	GOOS="$(OS)" GOARCH="$(GOARCH)" CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static' -X github.com/deckhouse/deckhouse/dhctl/pkg/app.AppVersion=$(D8_VERSION)"  -o $(DHCTL_BIN_NAME) ./cmd/dhctl

build-test:
	CGO_ENABLED=0 GOOS="linux" GOARCH="$(GOARCH)" go build -ldflags="-s -w" -o "bin/dhctl-linux-amd64-test" ./cmd/dhctl

test:
	go test ./...

deps: bin/golangci-lint bin/gofumpt bin/protoc
	go mod tidy

lint:
	golangci-lint run ./... -c .golangci.yaml

fix:
	golangci-lint run ./... -c .golangci.yaml --fix

fmt:
	@# - gofumpt is not included in the .golangci.yaml because it conflicts with imports https://github.com/golangci/golangci-lint/issues/1490#issuecomment-778782810
	@# - goimports is not turned on since it is used mostly by gofumpt internally
	gofumpt -l -w -extra .
	golangci-lint run ./... -c .golangci.yaml --fix

ci:
	./hack/coverage.sh

devenv:
	./hack/devenv/start.sh

devstop:
	./hack/devenv/stop.sh
