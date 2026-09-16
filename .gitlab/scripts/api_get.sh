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

api_get() {
local url="$1"
local token_type="${2:-auto}"

case "${token_type}" in
private)
    if [[ -z "${GITLAB_TOKEN:-}" ]]; then
    echo "GITLAB_TOKEN is required for private API request" >&2
    return 1
    fi
    curl -sfS --header "PRIVATE-TOKEN: ${GITLAB_TOKEN}" "${url}"
    ;;

job)
    if [[ -z "${CI_JOB_TOKEN:-}" ]]; then
    echo "CI_JOB_TOKEN is required for job API request" >&2
    return 1
    fi
    curl -sfS --header "JOB-TOKEN: ${CI_JOB_TOKEN}" "${url}"
    ;;

auto)
    if [[ -n "${GITLAB_TOKEN:-}" ]]; then
    curl -sfS --header "PRIVATE-TOKEN: ${GITLAB_TOKEN}" "${url}"
    elif [[ -n "${CI_JOB_TOKEN:-}" ]]; then
    curl -sfS --header "JOB-TOKEN: ${CI_JOB_TOKEN}" "${url}"
    else
    echo "Neither GITLAB_TOKEN nor CI_JOB_TOKEN is set" >&2
    return 1
    fi
    ;;

*)
    echo "Unknown token type: ${token_type}. Use: private, job, auto" >&2
    return 1
    ;;
esac
}