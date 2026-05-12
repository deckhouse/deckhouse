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

# Shared logging for tools/sync_versions/*.sh — one line per message: "LEVEL: text".
# Set SYNC_VERSIONS_DEBUG=1 to enable DEBUG lines.

log_msg() {
  local level="$1"
  shift
  printf '%s: %s\n' "$level" "$*" >&2
}

log_debug() {
  [[ "${SYNC_VERSIONS_DEBUG:-0}" == 1 ]] || return 0
  log_msg DEBUG "$@"
}

log_info() {
  log_msg INFO "$@"
}

log_warn() {
  log_msg WARN "$@"
}

log_error() {
  log_msg ERROR "$@"
}
