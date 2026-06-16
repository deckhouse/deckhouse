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

WORDLIST_PATH="${WORDLIST_PATH:-tools/docs/spelling/wordlist}"
DICT_PATH="${DICT_PATH:-tools/docs/spelling/dictionaries/dev_OPS.dic}"
DIFF_PATH="${DIFF_PATH:-${1:-./pr.diff}}"

changed_wordlist=false
changed_dic=false

if grep '^diff --git ' "${DIFF_PATH}" | grep -Fq "${WORDLIST_PATH}"; then
  changed_wordlist=true
fi
if grep '^diff --git ' "${DIFF_PATH}" | grep -Fq "${DICT_PATH}"; then
  changed_dic=true
fi

if [[ "${changed_wordlist}" == "false" && "${changed_dic}" == "false" ]]; then
  echo "No changes detected in ${WORDLIST_PATH} or ${DICT_PATH}; skipping sync checks."
  exit 0
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORDLIST_PATH="${WORDLIST_PATH}" "${script_dir}/validate_wordlist.sh"

errors=0

if [[ "${changed_wordlist}" == "true" && "${changed_dic}" == "false" ]]; then
  echo "ERROR: Spelling dictionary sync error: ${WORDLIST_PATH} changed, but ${DICT_PATH} was not updated."
  echo "Description: dev_OPS dictionary must be regenerated after editing the wordlist."
  errors=1
fi

if [[ "${changed_dic}" == "true" && "${changed_wordlist}" == "false" ]]; then
  echo "ERROR: Spelling dictionary sync error: ${DICT_PATH} changed, but ${WORDLIST_PATH} was not updated."
  echo "Description: wordlist must be updated and dev_OPS dictionary regenerated."
  errors=1
fi

expected_count="$(wc -l < "${WORDLIST_PATH}" | tr -d '[:space:]')"
actual_count="$(sed -n '1p' "${DICT_PATH}" | tr -d '\r' | tr -d '[:space:]')"

if [[ "${expected_count}" != "${actual_count}" ]]; then
  echo "ERROR: Spelling dictionary count mismatch: ${DICT_PATH} first line != wordlist word count."
  echo "Description: expected '${expected_count}' (line count of ${WORDLIST_PATH}), got '${actual_count}'."
  errors=1
fi

dict_words_file="$(mktemp)"
cleanup() {
  rm -f "${dict_words_file}"
}
trap cleanup EXIT

tail -n +2 "${DICT_PATH}" > "${dict_words_file}"

if ! diff -u "${WORDLIST_PATH}" "${dict_words_file}" > "${dict_words_file}.diff"; then
  echo "ERROR: Spelling dictionary content mismatch: words in ${DICT_PATH} do not match ${WORDLIST_PATH}."
  echo "Description: dictionary words (without the header line) must exactly match wordlist."
  echo "Regenerate with 'make docs-spellcheck-generate-dictionary'."
  head -n 30 "${dict_words_file}.diff"
  errors=1
fi
rm -f "${dict_words_file}.diff"

if [[ "${errors}" -ne 0 ]]; then
  exit 1
fi
