#!/bin/bash

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

. $(dirname "$(realpath "$0")")/spell_lib.sh

diff_start_re='^diff --git a/(.*) b/(.*)$'
new_file_name_re='^\+\+\+ (/dev/null|b/(.*))$'
end_metadata_re='^@@[- 0-9,+]+@@(.*)$'

first_line=true
metadata_block=false
skip_file=false
pr_diff=./pr.diff

file_name=""
file_changes=""

exit_code=0

function check() {
  local file_name=$1
  local file_changes=$2

  if [[ -n "${file_name}" ]] && [[ -n "${file_changes}" ]]; then
    local result=$(file_diff_check_spell ${file_name} "${file_changes}")
    if [ -n "${result}" ]; then
      print_message_about_typos_in_a_file "${file_name}"
      echo "${result}" | sed 's/\s\+/\n/g'
      echo
      exit_code=1
    fi
  fi
}

while IFS= read -r line
do
  if [[ "${line}" =~ ${diff_start_re} ]]; then
    check "${file_name}" "${file_changes}"
    metadata_block=true
    skip_file=false
    file_changes=""
    continue
  elif [[ "${line}" =~ $new_file_name_re ]]; then
    file_name=${BASH_REMATCH[2]}
    skip_file=false
    if ! validate_file_name "${file_name}"; then
      skip_file=true
      continue
    fi
    continue
  elif [[ "${line}" =~ $end_metadata_re ]]; then
    metadata_block=false
    continue
  fi
  if [ ${metadata_block} = true ] || [ ${skip_file} = true ]; then
    continue
  fi
  file_changes+=$(echo "${line}" | sed 's/^[+-]//')
done < ${pr_diff}

check "${file_name}" "${file_changes}"

exit ${exit_code}
