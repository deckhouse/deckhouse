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

# Run integration tests for waypoint-controller.
#
# This script fetches the envtest assets (etcd + kube-apiserver binaries) via
# setup-envtest into a local cache, then runs `go test` with the integration
# build tag.
#
# Usage:
#   hack/test.sh           # run integration tests (downloads envtest if needed)
#   hack/test.sh unit      # run unit tests only (no envtest)
#   hack/test.sh all       # run unit + integration tests
#
# Environment:
#   ENVTEST_K8S_VERSION    Kubernetes version of envtest assets (default: 1.31.0)
#   KUBEBUILDER_ASSETS     If set, skip download and use this assets directory.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${SRC_DIR}"

ENVTEST_K8S_VERSION="${ENVTEST_K8S_VERSION:-1.31.0}"
# setup-envtest is published from release branches. release-0.22 is the latest
# branch that still builds on Go 1.25; newer branches require Go 1.26+.
SETUP_ENVTEST_REF="${SETUP_ENVTEST_REF:-release-0.22}"

mode="${1:-integration}"

prepare_envtest() {
    if [[ -z "${KUBEBUILDER_ASSETS:-}" ]]; then
        echo ">>> Fetching envtest binaries for k8s ${ENVTEST_K8S_VERSION}"
        # `setup-envtest use -p path` prints the assets dir on stdout.
        KUBEBUILDER_ASSETS="$(GOFLAGS=-mod=mod go run "sigs.k8s.io/controller-runtime/tools/setup-envtest@${SETUP_ENVTEST_REF}" use "${ENVTEST_K8S_VERSION}" -p path)"
        export KUBEBUILDER_ASSETS
    fi
    echo ">>> KUBEBUILDER_ASSETS=${KUBEBUILDER_ASSETS}"
}

# Each run_* function uses `exec` for the final `go test` so that Ctrl-C from
# a terminal is delivered straight to `go test` (which forwards SIGINT to the
# test binary's signal handler) instead of being eaten by bash.
run_unit() {
    echo ">>> Running unit tests"
    exec env GOFLAGS=-mod=mod go test ./...
}

run_integration() {
    prepare_envtest
    echo ">>> Running integration tests"
    exec env GOFLAGS=-mod=mod go test -tags=integration -count=1 -v ./...
}

run_all() {
    echo ">>> Running unit tests"
    GOFLAGS=-mod=mod go test ./...
    prepare_envtest
    echo ">>> Running integration tests"
    exec env GOFLAGS=-mod=mod go test -tags=integration -count=1 -v ./...
}

case "${mode}" in
    unit)
        run_unit
        ;;
    integration)
        run_integration
        ;;
    all)
        run_all
        ;;
    *)
        echo "Unknown mode: ${mode}" >&2
        echo "Usage: $0 [unit|integration|all]" >&2
        exit 2
        ;;
esac
