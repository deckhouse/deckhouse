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

# Repeats a request that admission has to refuse until the refusal carries the
# expected message. Use it where the refusing policy appears with a delay, for
# example a Gatekeeper constraint rendered after a namespace label or a policy change.
#
# Usage: eventually-denied.sh <TIMEOUT_SECONDS> <EXPECTED_MESSAGE> <COMMAND> [ARGS...]
#
# The command must not read standard input: it runs several times.
set -Eeuo pipefail

timeout="${1:?timeout is required}"
expected="${2:?expected message is required}"
shift 2
deadline=$(( $(date +%s) + timeout ))

while true; do
  if out="$("$@" 2>&1)"; then
    echo "The request was admitted: ${out}"
  elif grep -qF -- "${expected}" <<<"${out}"; then
    echo "The request was refused as expected: ${out}"
    exit 0
  else
    echo "The request failed with another message: ${out}"
  fi
  if (( $(date +%s) >= deadline )); then
    >&2 echo "The request was not refused with '${expected}' within ${timeout}s"
    exit 1
  fi
  sleep 5
done
