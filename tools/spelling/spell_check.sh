#!/bin/bash

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

echo "Spell check the documentation..."

if [ -n "${file_name}" ]; then
    if validate_file_name "${file_name}"; then
        print_message_about_typos_in_a_file "${file_name}"
        result="$(file_check_spell ${file_name})"
        if [ -n "${result}" ]; then
          echo "${result}" | sed 's/\s\+/\n/g'
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
          echo ${result} | sed 's/\s\+/\n/g'
          echo
        fi
    fi
  done
fi

exit ${exit_code}
