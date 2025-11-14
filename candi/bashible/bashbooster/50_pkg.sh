# Copyright 2024 Flant JSC
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

bb-pkg() {
    local action="${1:-}"

    if [[ -z "${action}" ]]; then
        bb-log-error "bb-pkg: action is required (e.g. install, remove, package?)"
        return 1
    fi

    shift || true

    local manager
    manager="$(bb-pkg-mgr)" || return 1

    local func="bb-${manager}-${action}"

    if ! declare -F "${func}" >/dev/null; then
        bb-log-error "bb-pkg: action '${action}' is not supported for package manager '${manager}'"
        return 1
    fi

    "${func}" "$@"
}
