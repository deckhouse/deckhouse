#!/usr/bin/env bash
# Copyright 2026 Flant JSC
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

# Run `go test` in nested Go modules touched by the diff between $DIFF_BASE and
# HEAD. The modules are found by changed-go-modules.sh.
#
# `make tests-modules` (TESTS_PATH = ./modules/... and the ee ones) stops at a
# nested module boundary, so tests of images/*/src, go_lib/* and pkg/* modules
# with their own go.mod never run in CI otherwise. Modules that already have a
# dedicated CI job are skipped, see SKIP_MODULES.
#
# Packages under e2e/ are left out, as the modules' own `make test` does: they
# need a kind cluster and docker. Suites built on controller-runtime envtest get
# kube-apiserver and etcd through KUBEBUILDER_ASSETS, downloaded by setup-envtest
# only when an affected module has such a suite. Without the assets these suites
# either fail or skip themselves, and a skipped suite tests nothing.
#
# Inputs (env vars):
#   DIFF_BASE             — git ref/SHA to diff against. Default: HEAD~1.
#   GO_TEST_ARGS          — args for `go test` before the package list.
#                           Default: -timeout ${TEST_TIMEOUT:-15m}.
#   KUBEBUILDER_ASSETS    — envtest binaries; when set, nothing is downloaded.
#   ENVTEST_K8S_VERSION   — Kubernetes version of the envtest binaries. Default: 1.34.1.
#   SETUP_ENVTEST_VERSION — setup-envtest version to install. Default: release-0.22.
#   ENVTEST_DIR           — where setup-envtest and the binaries go.
#                           Default: ${XDG_CACHE_HOME:-$HOME/.cache}/deckhouse-envtest.

set -Eeuo pipefail

DIFF_BASE="${DIFF_BASE:-HEAD~1}"
GO_TEST_ARGS="${GO_TEST_ARGS:--timeout ${TEST_TIMEOUT:-15m}}"
ENVTEST_K8S_VERSION="${ENVTEST_K8S_VERSION:-1.34.1}"
SETUP_ENVTEST_VERSION="${SETUP_ENVTEST_VERSION:-release-0.22}"
ENVTEST_DIR="${ENVTEST_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/deckhouse-envtest}"

# Modules not tested here, as module directories: covered by their own jobs.
SKIP_MODULES=(
  '.'                                                   # tests: make tests-controller tests-modules
  'dhctl'                                               # dhctl_tests
  'modules/040-node-manager/images/node-controller/src' # tests_node_controller
  'modules/038-registry/images/registry-agent/src'      # tests_registry_images
  'modules/038-registry/images/registry-controller/src' # tests_registry_images
  'modules/038-registry/images/registry-syncer/src'     # tests_registry_images
  'modules/038-registry/images/docker-distribution/wrapper' # tests_registry_images
)

# Download the envtest binaries once and export KUBEBUILDER_ASSETS. envtest reads
# KUBEBUILDER_ASSETS before the BinaryAssetsDirectory a suite sets itself.
ensure_envtest() {
  if [[ -n "${KUBEBUILDER_ASSETS:-}" ]]; then
    return 0
  fi
  echo "Installing envtest binaries for Kubernetes ${ENVTEST_K8S_VERSION} into ${ENVTEST_DIR}"
  # Called from a condition, where set -e does not apply: every step returns explicitly.
  mkdir -p "$ENVTEST_DIR" || return 1
  GOBIN="$ENVTEST_DIR" GOFLAGS="-buildvcs=false" \
    go install "sigs.k8s.io/controller-runtime/tools/setup-envtest@${SETUP_ENVTEST_VERSION}" || return 1
  local assets
  assets="$("$ENVTEST_DIR/setup-envtest" use "$ENVTEST_K8S_VERSION" --bin-dir "$ENVTEST_DIR" -p path)" || return 1
  [[ -n "$assets" ]] || return 1
  export KUBEBUILDER_ASSETS="$assets"
}

uses_envtest() {
  grep -rq --include='*_test.go' 'sigs.k8s.io/controller-runtime/pkg/envtest' "$1"
}

# Files we care about: Go sources, module manifests and test fixtures.
# Command substitution, not `< <(...)`: if git diff fails, the job must fail
# rather than pass with nothing to test.
changed="$(
  DIFF_BASE="$DIFF_BASE" bash "$(dirname "${BASH_SOURCE[0]}")/changed-go-modules.sh" \
    '*.go' \
    'go.mod' 'go.sum' \
    '**/go.mod' '**/go.sum' \
    '**/testdata/**'
)"
CHANGED_MODULES=()
if [[ -n "$changed" ]]; then
  mapfile -t CHANGED_MODULES <<< "$changed"
fi

AFFECTED=()
for mod in "${CHANGED_MODULES[@]}"; do
  skip=""
  for skipped in "${SKIP_MODULES[@]}"; do
    if [[ "$mod" == "$skipped" ]]; then
      skip=1
      break
    fi
  done
  if [[ -n "$skip" ]]; then
    echo "Skipping ./${mod}: listed in SKIP_MODULES."
    continue
  fi
  AFFECTED+=("$mod")
done

if (( ${#AFFECTED[@]} == 0 )); then
  echo "No nested Go modules to test."
  exit 0
fi

echo "Affected Go modules (diff base: $DIFF_BASE):"
for mod in "${AFFECTED[@]}"; do echo "  - $mod"; done

FAILED=()
for mod in "${AFFECTED[@]}"; do
  echo ""
  echo "============================================================"
  echo "Running go test in ./${mod}"
  echo "============================================================"
  if uses_envtest "$mod" && ! ensure_envtest; then
    echo "Could not install the envtest binaries." >&2
    FAILED+=("$mod")
    continue
  fi
  (
    cd "$mod"
    pkgs="$(GOFLAGS="-buildvcs=false" go list ./...)"
    pkgs="$(grep -vE '/e2e(/|$)' <<< "$pkgs" || true)"
    if [[ -z "$pkgs" ]]; then
      echo "No packages to test outside e2e/."
      exit 0
    fi
    # shellcheck disable=SC2086 # GO_TEST_ARGS holds several flags, pkgs several packages.
    GOFLAGS="-buildvcs=false" go test $GO_TEST_ARGS $pkgs
  ) || FAILED+=("$mod")
done

if (( ${#FAILED[@]} > 0 )); then
  echo ""
  echo "go test failed in:"
  for mod in "${FAILED[@]}"; do echo "  - $mod"; done
  exit 1
fi
