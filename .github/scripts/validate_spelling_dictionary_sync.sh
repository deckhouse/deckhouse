#!/bin/bash

# Copyright 2021 Flant JSC
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

set -Eeo pipefail

WORDLIST_PATH="tools/docs/spelling/wordlist"
DICTIONARY_PATH="tools/docs/spelling/dictionaries/dev_OPS.dic"

function diff_touches_file() {
  local diffFile=$1
  local filePath=$2
  grep -qE "^diff --git a/${filePath}[[:space:]]" "${diffFile}"
}

diffFile=$1
if [[ -z "${diffFile}" || ! -f "${diffFile}" ]]; then
  echo "Error: diff file is not provided or not found."
  exit 1
fi

echo "Run spelling dictionary sync validation ..."

validationFailed=0

wordlistChanged=false
dictionaryChanged=false

if diff_touches_file "${diffFile}" "${WORDLIST_PATH}"; then
  wordlistChanged=true
fi
if diff_touches_file "${diffFile}" "${DICTIONARY_PATH}"; then
  dictionaryChanged=true
fi

if [[ "${wordlistChanged}" == true && "${dictionaryChanged}" == false ]]; then
  echo "Error: spelling dictionary sync validation failed."
  echo "  When '${WORDLIST_PATH}' is changed, '${DICTIONARY_PATH}' must also be updated."
  echo "  Run 'make docs-spellcheck-generate-dictionary' to regenerate the dictionary."
  validationFailed=1
fi

if [[ "${dictionaryChanged}" == true && "${wordlistChanged}" == false ]]; then
  echo "Error: spelling dictionary sync validation failed."
  echo "  When '${DICTIONARY_PATH}' is changed, '${WORDLIST_PATH}' must also be updated."
  echo "  The dictionary must be generated from the wordlist, not edited directly."
  validationFailed=1
fi

wordlistFile="./${WORDLIST_PATH}"
dictionaryFile="./${DICTIONARY_PATH}"

if [[ ! -f "${wordlistFile}" ]]; then
  echo "Error: spelling dictionary header validation failed."
  echo "  File '${WORDLIST_PATH}' is not found."
  validationFailed=1
elif [[ ! -f "${dictionaryFile}" ]]; then
  echo "Error: spelling dictionary header validation failed."
  echo "  File '${DICTIONARY_PATH}' is not found."
  validationFailed=1
else
  expectedCount=$(wc -l < "${wordlistFile}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
  actualHeader=$(head -n1 "${dictionaryFile}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

  if [[ "${actualHeader}" != "${expectedCount}" ]]; then
    echo "Error: spelling dictionary header validation failed."
    echo "  The first line of '${DICTIONARY_PATH}' must contain the number of words in '${WORDLIST_PATH}'."
    echo "  Expected: ${expectedCount}, actual: ${actualHeader}."
    validationFailed=1
  fi
fi

exit "${validationFailed}"
