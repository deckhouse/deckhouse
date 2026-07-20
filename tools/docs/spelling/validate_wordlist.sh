#!/bin/bash

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

set -euo pipefail

wordlist="${WORDLIST_PATH:-./tools/docs/spelling/wordlist}"

duplicates="$({
  uniq -d "${wordlist}"
  awk '{print $1}' "${wordlist}" | sort | uniq -d
} | sed '/^$/d' | sort -u)"
spaced="$(grep -E '[[:space:]]' "${wordlist}" || true)"

has_error=0

if [[ -n "${duplicates}" ]]; then
  echo "ERROR: duplicate words in wordlist:"
  echo "${duplicates}" | sed 's/^/  - /'
  has_error=1
fi

if [[ -n "${spaced}" ]]; then
  echo "ERROR: words with whitespace in wordlist:"
  echo "${spaced}" | sed 's/^/  - /'
  has_error=1
fi

if [[ "${has_error}" -ne 0 ]]; then
  exit 1
fi
