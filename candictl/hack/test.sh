#!/bin/sh

is_gitlab=$([[ $1 == "--gitlab" ]] && echo "true")

function gitlab_wrap_header() {
  [[ "$is_gitlab" == "true" ]] && echo -e "section_start:`date +%s`:$1\r\e[0K$2" || echo $2
}

function gitlab_wrap_tail() {
  [[ "$is_gitlab" == "true" ]] && echo -e "section_end:`date +%s`:$1\r\e[0K"
}

function main() {
  gitlab_wrap_header "golangci-lint" "🦊 golangci-lint"
  golangci-lint run --config=./config/golangci-lint.yaml  ../... \
    && echo "Success!" \
    || exit 1
  gitlab_wrap_tail "golangci-lint"

  echo ""

  gitlab_wrap_header "unit-tests-candictl" "🦊 go test"
  tmpfile=$(mktemp /tmp/coverage-report.XXXXXX)
  go test -cover -coverprofile=${tmpfile} -vet=off ../pkg/... \
    && echo "Coverage: $(go tool cover -func  ${tmpfile} | grep total | awk '{print $3}')" \
    && echo "Success!" \
    || exit 1
  gitlab_wrap_tail "unit-tests-candictl"
}

main
