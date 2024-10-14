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

set -e

. $(dirname "$(realpath "$0")")/spell_lib.sh

SKIP_MISSING_FILES=0
exit_code=0

HELP_STRING=$(cat <<EOF
Usage: spell_check.sh [OPTION]

Optional arguments:
  -f PATH, --file PATH         the name of the file with a path (relative from the Deckhouse repo)
  -l PATH, --list PATH         the name of the file with a list of files to check (relative from the Deckhouse repo)
  --skipmissing                skip missing files
  -h, --help         output this message
EOF
)

while true; do
  case "$1" in
    -f | --file )
      if [ -n "$2" ]; then
        if [ -f "$2" ]; then
          file_name=$2; shift 2
        else
          echo "File $2 not found" >&2
          exit 1
        fi
      else
        shift 1
      fi
      ;;
    -l | --list )
      files_list_filename=$2; shift 2;;
    --skipmissing )
      SKIP_MISSING_FILES=1; shift 1;;
    -h | --help )
      echo "${HELP_STRING}"; exit 0 ;;
    * )
      break ;;
  esac
done

echo "Spell-checking the documentation..."

if [ -n "${file_name}" ]; then
    if validate_file_name "${file_name}"; then
        print_message_about_typos_in_a_file "${file_name}"
        result="$(file_check_spell ${file_name})"
        if [ -n "${result}" ]; then
          echo "${result}" | sed 's/\s\+/\n/g' | sort -u
          echo
          exit_code=1
        fi
    fi
else
  if [ -n "${files_list_filename}" ]; then
      files_search_path="$(cat ${files_list_filename})"
  else
      files_search_path="$(find ./ -type f)"
  fi
  for file_name in ${files_search_path};
  do
    if validate_file_name "${file_name}"; then
        result="$(file_check_spell "${file_name}")"
        if [ -n "${result}" ]; then
          exit_code=1
          print_message_about_typos_in_a_file "${file_name}"
          echo ${result} | sed 's/\s\+/\n/g' | sort -u
          echo
        fi
    fi
  done
fi

exit ${exit_code}
