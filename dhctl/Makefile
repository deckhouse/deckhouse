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

# Build versions
GO_VERSION=1.19.3
GOARCH=amd64

DHCTL_BIN_NAME=bin/dhctl

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

build:
	GOOS="$(OS)" GOARCH="$(GOARCH)" CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static' -X github.com/deckhouse/deckhouse/dhctl/pkg/app.AppVersion=$(D8_VERSION)"  -o $(DHCTL_BIN_NAME) ./cmd/dhctl

build-test:
	CGO_ENABLED=0 GOOS="linux" GOARCH="$(GOARCH)" go build -ldflags="-s -w" -o "bin/dhctl-linux-amd64-test" ./cmd/dhctl

test:
	go test ./...

deps: bin/golangci-lint bin/gofumpt
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
