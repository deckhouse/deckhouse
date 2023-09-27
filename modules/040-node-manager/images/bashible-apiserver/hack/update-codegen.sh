#!/usr/bin/env bash
# Copyright 2017 The Kubernetes Authors.
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

set -o errexit
set -o nounset

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
export GOPATH=${GOPATH:-$(go env | grep GOPATH | cut -d= -f2 | tr -d '"')}
CODEGEN_PKG_ABS=${GOPATH}/pkg/mod/$(go mod graph | grep code-generator | head -n 1 | cut -d" " -f2)

# Note (Eugene Shevchenko):
#   We need relative path for the code-generation script to work properly
#   and this is the best thing I found. Supports python versions 2 and 3.
PY=$(which python || which python3)
CODEGEN_PKG=$($PY -c "import os.path; print (os.path.relpath('${CODEGEN_PKG_ABS}', '$(pwd)'))")

# Note (Eugene Shevchenko):
#   Fixing failing code generation when OPENAPI_EXTRA_PACKAGES array is not defined
export OPENAPI_EXTRA_PACKAGES=${OPENAPI_EXTRA_PACKAGES:-(())}

# Note (Yuriy Losev)
# kube_codegen.sh is a new script intended to run kubernetes code generators (deepcopy, defaulter, etc)
# this script exists for generators >= 0.28 version, so, if we use kube-client < 0.28
# we are running this script from our repo, otherwise - from upstream
if [ -z "${CODEGEN_PKG}/kube_codegen.sh" ]; then
  source "${CODEGEN_PKG}/kube_codegen.sh"
else
  source "${SCRIPT_ROOT}/hack/kube_codegen.sh"
fi

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.

kube::codegen::gen_helpers \
    --input-pkg-root bashible-apiserver/pkg/apis \
    --output-base "${SCRIPT_ROOT}/.." \
    --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

kube::codegen::gen_openapi \
    --input-pkg-root bashible-apiserver/pkg/apis \
    --output-pkg-root bashible-apiserver/pkg/generated \
    --output-base "${SCRIPT_ROOT}/.." \
    --report-filename "${SCRIPT_ROOT}/hack/openapi_violation_exceptions.list" \
    --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

kube::codegen::gen_client \
    --with-applyconfig \
    --input-pkg-root bashible-apiserver/pkg/apis \
    --output-pkg-root bashible-apiserver/pkg/generated \
    --output-base "${SCRIPT_ROOT}/.." \
    --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt
