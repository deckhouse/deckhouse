#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
# Copyright 2025 Flant JSC
#
# Modifications made by Flant JSC as part of the Deckhouse project.
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
set -o pipefail

CODEGEN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

function codegen::internal::findz() {
    # We use `find` rather than `git ls-files` because sometimes external
    # projects use this across repos.  This is an imperfect wrapper of find,
    # but good enough for this script.
    find "$@" -print0
}

function codegen::internal::grep() {
    # We use `grep` rather than `git grep` because sometimes external projects
    # use this across repos.
    grep "$@" \
        --exclude-dir .git \
        --exclude-dir _output \
        --exclude-dir vendor
}

# Generate deepcopy code only
#
# USAGE: codegen::gen_deepcopy [FLAGS] <input-dir>
#
# <input-dir>
#   The root directory under which to search for Go files which request code to
#   be generated.  This must be a local path, not a Go package.
#
# FLAGS:
#
#   --boilerplate <string = path_to_codegen_boilerplate>
#     An optional override for the header file to insert into generated files.
#
function codegen::gen_deepcopy() {
    local in_dir=""
    local boilerplate="${CODEGEN_ROOT}/hack/boilerplate.go.txt"
    local v="${VERBOSE:-0}"

    while [ "$#" -gt 0 ]; do
        case "$1" in
            "--boilerplate")
                boilerplate="$2"
                shift 2
                ;;
            *)
                if [[ "$1" =~ ^-- ]]; then
                    echo "unknown argument: $1" >&2
                    return 1
                fi
                if [ -n "$in_dir" ]; then
                    echo "too many arguments: $1 (already have $in_dir)" >&2
                    return 1
                fi
                in_dir="$1"
                shift
                ;;
        esac
    done

    if [ -z "${in_dir}" ]; then
        echo "input-dir argument is required" >&2
        return 1
    fi

    # Install deepcopy-gen only
    (
        cd "${CODEGEN_ROOT}"
        GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen@latest
    )
    
    gobin="${GOBIN:-$(go env GOPATH)/bin}"

    # Find packages that need deepcopy
    local input_pkgs=()
    while read -r dir; do
        pkg="$(cd "${dir}" && GO111MODULE=on go list -find .)"
        input_pkgs+=("${pkg}")
    done < <(
        ( codegen::internal::grep -l --null \
            -e '^\s*//\s*+k8s:deepcopy-gen=' \
            -r "${in_dir}" \
            --include '*.go' \
            || true \
        ) | while read -r -d $'\0' F; do dirname "${F}"; done \
          | LC_ALL=C sort -u
    )

    if [ "${#input_pkgs[@]}" != 0 ]; then
        echo "Generating deepcopy code for ${#input_pkgs[@]} targets"

        codegen::internal::findz \
            "${in_dir}" \
            -type f \
            -name zz_generated.deepcopy.go \
            | xargs -0 rm -f

        "${gobin}/deepcopy-gen" \
            -v "${v}" \
            --output-file zz_generated.deepcopy.go \
            --go-header-file "${boilerplate}" \
            "${input_pkgs[@]}"
    fi
}
