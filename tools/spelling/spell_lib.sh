#!/bin/bash

DICTIONARIES="/app/dictionaries/ru_RU,/app/dictionaries/en_US,/app/dictionaries/dev_OPS"
FILES_TO_IGNORE="$(dirname "$(realpath "$0")")/filesignore"
FILES_MATCH_PATTERN='.+(.md|.html|.htm|.liquid|.yaml|.yml)$'

MSG_PREFIX_FILE_TYPOS="Possible typos in"

# Returns 0 if file should be processed, 1 if it should be skipped
# Function to validate a given file name against a list of files to ignore
# Parameters:
# $1: file_name - the name of the file to validate
validate_file_name() {
  local file_name=$1

  if ! [[ "$file_name" =~ ${FILES_MATCH_PATTERN} ]]; then
    return 1
  fi

  # Check if the list of files to ignore exists
  if test -f "${FILES_TO_IGNORE}"; then
    ignore_patterns=$(grep -vE '^#\s*|^\s*$' "${FILES_TO_IGNORE}" | tr '\n' '|' | sed 's/|$//')
    if [[ "$file_name" =~ $ignore_patterns ]]; then
      echo "Ignoring ${file_name}..." >&2
      return 1
    fi
  else
    # Warn if the list of files to ignore is missing
    echo "Warning! No patterns with filenames to ignore (the filesignore file)" >&2
  fi

  return 0  # File name is not in the ignore list
}

# Function to check the spelling in a file
# Arguments:
#   $1: The file to check for spelling
file_check_spell() {
  # Check if the file exists
  if [[ ! -f ${1} ]]; then
    # Handle missing file based on SKIP_MISSING_FILES flag
    if [ "$SKIP_MISSING_FILES" -eq 1 ]; then
      echo "Skip missing file ${1}..." >&2
      return 0
    fi
    # Return error if file not found
    echo "Error: file ${1} not found..." >&2
    return 1
  else
    # Run spell check using hunspell after cleaning the file
    python3 /app/clean-files.py ${1} | sed '/^\s*$/d' | hunspell -d ${DICTIONARIES} -l
  fi
}

# Function to check spelling in a file based on the provided changes
# Arguments:
#   $1: file_name - name of the file to check
#   $2: file_changes - changes made to the file
file_diff_check_spell() {
  local file_name=$1
  local file_changes="$2"

  # Clean the file changes, remove empty lines, check spelling, and format the output
  echo "${file_changes}" | python3 /app/clean-files.py - | sed '/^\s*$/d' | hunspell -d ${DICTIONARIES} -l | sed 's/\s\+/\n/g'
}

# Function to print a message about typos in a file
print_message_about_typos_in_a_file() {
  # Print the message prefix and the file name without './'
  printf "%s %s:\n" "${MSG_PREFIX_FILE_TYPOS}" "$(echo "${1}" | sed 's#^\./##')"
}
