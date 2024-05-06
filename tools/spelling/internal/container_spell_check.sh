#!/bin/sh

set -e

SKIP_MISSING_FILES=0
DICTIONARIES="/app/dictionaries/ru_RU,/app/dictionaries/en_US,/app/dictionaries/dev_OPS"
ex_result=0

function file_check_spell() {
  if [[ ! -f ${1} ]]; then
    if [ "$SKIP_MISSING_FILES" -eq 1 ]; then
      echo "Skip missing file ${1}..." >&2
      return 0
    fi
    echo "Error: file ${1} not found..." >&2
    return 1
  else
    python3 /app/internal/clean-files.py ${1} | sed '/^\s*$/d' | hunspell -d ${DICTIONARIES} -l
  fi
}


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
          FILENAME=$2; shift 2
        else
          echo "File $2 not found" >&2
          exit 1
        fi
      else
        shift 1
      fi
      ;;
    -l | --list )
      FILELIST=$2; shift 2;;
    --skipmissing )
      SKIP_MISSING_FILES=1; shift 1;;
    -h | --help )
      echo "$HELP_STRING"; exit 0 ;;
    * )
      break ;;
  esac
done

echo "Checking docs..."

if [ -n "${FILENAME}" ]; then
    check=1
    if test -f "/app/internal/filesignore"; then
      while read file_name_to_ignore; do
        if [[ "${FILENAME}" =~ ${file_name_to_ignore} ]]; then
          unset check
          check=0
        fi
      done <<-__EOF__
  $(cat /app/internal/filesignore | grep -vE '^#\s*|^\s*$')
__EOF__
      if [ "$check" -eq 1 ]; then
        echo "Possible typos in $(echo ${FILENAME} | sed '#^\./#d'):"
        result="$(file_check_spell ${FILENAME})"
        if [ -n "$result" ]; then
          echo $result | sed 's/\s\+/\n/g'
        fi
      else
        echo "Ignoring ${FILENAME}..."
      fi
    fi
else
  if [ -n "${FILELIST}" ]; then
      LIST="$(cat ${FILELIST})"
  else
      LIST="$(find ./ -type f)"
  fi
  for file in $LIST;
  do
    check=1
    if test -f "/app/internal/filesignore"; then
      while read file_name_to_ignore; do
        if [[ "$file" =~ ${file_name_to_ignore} ]]; then
          unset check
          check=0
        fi
      done <<-__EOF__
  $(cat /app/internal/filesignore | grep -vE '^#\s*|^\s*$')
__EOF__
      if [ "$check" -eq 1 ]; then
        result="$(file_check_spell ${file})"
        if [ -n "$result" ]; then
          unset ex_result
          ex_result=1
          echo "Possible typos in $(echo ${file} | sed '#^\./#d'):"
          echo $result | sed 's/\s\+/\n/g'
          echo
        fi
      else
        echo "Ignoring $file..."
      fi
    fi
  done
  if [ "$ex_result" -eq 1 ]; then
    exit 1
  fi
fi
