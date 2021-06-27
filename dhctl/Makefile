# Copyright 2021 Flant CJSC
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

GO_VERSION=1.16.3
GOLANGCI_LINT_VERSION=1.39.0
GOFUMPT_VERSION=0.1.1
GOARCH=amd64
UNAME=$(shell uname -s)
DEV_CONTAINER_NAME=dhctl-dev

GOFUMPT_BIN=/tmp/gofumpt-$(GOFUMPT_VERSION)
GOLANGCILINT_BIN=/tmp/golangci-lint-$(GOLANGCI_LINT_VERSION)

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif


.PHONY : deps fmt lint build test all ci devenv devstop

all: deps fmt build test

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

test:
	go test ./...

build:
	GOOS="$(OS)" GOARCH="$(GOARCH)" go build -ldflags="-s -w" -o /tmp/dhctl ./cmd/dhctl

ci: deps lint
	tmpfile=$(mktemp /tmp/coverage-report.XXXXXX)
	go test -cover -coverprofile=${tmpfile} -vet=off ./pkg/... \
        && echo "Coverage: $(go tool cover -func  ${tmpfile} | grep total | awk '{print $3}')" \
        && echo "Success!" \
        || exit 1

# Shell script within makefile https://stackoverflow.com/a/29085760
devenv:
	@set -e ;\
	docker pull registry.flant.com/sys/antiopa/dev/install:master ;\
	CONTAINER_ID=$$(docker ps -aqf "name=$(DEV_CONTAINER_NAME)") ;\
	echo ;\
	test -n "$$CONTAINER_ID" && echo "Container $$CONTAINER_ID exists, entering." || echo "Creating container..." ;\
	echo ;\
	test -z "$$CONTAINER_ID" && CONTAINER_ID=$$(docker run \
			--name "$(DEV_CONTAINER_NAME)" \
			--detach \
			--rm \
			-v $(HOME)/.ssh/:/root/.ssh/ \
			--mount type=tmpfs,destination=/tmp:exec \
			-v "$$(pwd):/dhctl" \
			-v "$$(pwd)/../candi:/deckhouse/candi" \
			registry.flant.com/sys/antiopa/dev/install:master \
			tail -f /dev/null) \
		&& docker exec    "$$CONTAINER_ID" apk add go curl make musl\
		&& docker exec -i "$$CONTAINER_ID" sh -c "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \"$$(go env GOPATH)\" /bin v$(GOLANGCI_LINT_VERSION)" \
		&& docker exec    "$$CONTAINER_ID" sh -c 'ln -fs /root/go/bin/golangci-lint /usr/local/bin/golangci-lint' ;\
	docker exec -it -w /dhctl "$$CONTAINER_ID" bash

devstop:
	@docker kill $$(docker ps -f name=$(DEV_CONTAINER_NAME) -q) 2&>/dev/null
