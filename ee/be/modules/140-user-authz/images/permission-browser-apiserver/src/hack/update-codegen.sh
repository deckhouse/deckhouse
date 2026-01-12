#!/usr/bin/env bash
# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
# See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${SCRIPT_ROOT}"

# Source the kube_codegen.sh functions
source "${SCRIPT_ROOT}/hack/kube_codegen.sh"

# Module and output configuration
MODULE_NAME="permission-browser-apiserver"
API_PKG="${MODULE_NAME}/pkg/apis"
OUTPUT_PKG="${MODULE_NAME}/pkg/generated"

echo "=== Generating deepcopy, defaulter, conversion ==="
kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

echo "=== Generating OpenAPI ==="
kube::codegen::gen_openapi \
    --output-dir "${SCRIPT_ROOT}/pkg/generated/openapi" \
    --output-pkg "${OUTPUT_PKG}/openapi" \
    --report-filename "${SCRIPT_ROOT}/hack/openapi_violation_exceptions.list" \
    --update-report \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

echo "=== Generating clientset, listers, informers ==="
kube::codegen::gen_client \
    --with-applyconfig \
    --output-dir "${SCRIPT_ROOT}/pkg/generated" \
    --output-pkg "${OUTPUT_PKG}" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

echo "=== Code generation complete ==="
