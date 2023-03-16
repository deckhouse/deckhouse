# https://github.com/golangci/golangci-lint
GOLANGCI_LINT_VERSION=1.51.1
# https://github.com/mvdan/gofumpt
GOFUMPT_VERSION=0.4.0
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


.PHONY: all
all: deps fmt build test

.PHONY: deps
deps:
	go mod tidy
	@test -f $(GOFUMPT_BIN)      || curl -sLo $(GOFUMPT_BIN) https://github.com/mvdan/gofumpt/releases/download/v$(GOFUMPT_VERSION)/gofumpt_v$(GOFUMPT_VERSION)_$(OS)_$(GOARCH)
	@test -f $(GOLANGCILINT_BIN) || curl -sfL https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(GOARCH).tar.gz | tar -xzOf - golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(GOARCH)/golangci-lint > $(GOLANGCILINT_BIN)
	@chmod +x $(GOLANGCILINT_BIN) $(GOFUMPT_BIN)

.PHONY: lint
lint:
	$(GOLANGCILINT_BIN) run ./... -c .golangci.yaml

.PHONY: fmt
fmt:
	@# - gofumpt is not included in the .golangci.yaml because it conflicts with imports https://github.com/golangci/golangci-lint/issues/1490#issuecomment-778782810
	@# - goimports is not turned on since it is used mostly by gofumpt internally
	gci -w --local deckhouse.io/uibackend .
	$(GOFUMPT_BIN) -l -w -extra .
	$(GOLANGCILINT_BIN) run ./... -c .golangci.yaml --fix

.PHONY: test
test:
	go test ./...

.PHONY: build
build:
	GOOS="$(OS)" GOARCH="$(GOARCH)" go build -ldflags="-s -w" -o backend .

.PHONY: ci
ci: deps lint
	tmpfile=$(mktemp /tmp/coverage-report.XXXXXX)
	go test -cover -coverprofile=${tmpfile} -vet=off ./pkg/... \
        && echo "Coverage: $(go tool cover -func  ${tmpfile} | grep total | awk '{print $3}')" \
        && echo "Success!" \
        || exit 1
