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

# Runs a command and checks how the API server answered it.
#
# Usage: expect.sh denied MESSAGE -- COMMAND...
#        expect.sh allowed -- COMMAND...
#
# denied  - COMMAND must fail and its combined output must contain MESSAGE
#           (a fixed string, matched as is);
# allowed - COMMAND must succeed. A failure caused by an admission webhook or an
#           authorization error is reported as such.
# The standard input of the script is passed to COMMAND, so a manifest can be
# piped into `kubectl apply -f -`.
set -Eeuo pipefail

mode="${1:?mode denied|allowed is required}"
shift
message=""
if [[ "${mode}" == "denied" ]]; then
  message="${1:?expected message is required}"
  shift
fi
[[ "${1:-}" == "--" ]] && shift
(( $# > 0 )) || { >&2 echo "a command is required after --"; exit 1; }

rc=0
out="$("$@" 2>&1)" || rc=$?
echo "$ $*"
echo "exit code ${rc}"
echo "${out}"

case "${mode}" in
  denied)
    if (( rc == 0 )); then
      >&2 echo "FAIL: the command succeeded, a denial with '${message}' was expected"
      exit 1
    fi
    if ! grep -qF -- "${message}" <<<"${out}"; then
      >&2 echo "FAIL: the command failed without the expected message '${message}'"
      exit 1
    fi
    echo "OK: denied with the expected message"
    ;;
  allowed)
    if (( rc != 0 )); then
      >&2 echo "FAIL: the command was expected to succeed"
      exit 1
    fi
    echo "OK: allowed"
    ;;
  *)
    >&2 echo "unknown mode '${mode}', expected denied or allowed"
    exit 1
    ;;
esac
