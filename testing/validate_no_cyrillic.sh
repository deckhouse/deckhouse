#!/bin/bash

set -e

function request_gitlab_api() {
  curl --silent -f -H "PRIVATE-TOKEN: ${JOB_TOKEN}"  https://fox.flant.com/api/v4/projects/${PROJECT_ID}/${1}
}

function check_russian_letters() {
  letters=$(grep -Eo "[А-Яа-яЁё]+" <<< ${1} || true)
  if [[ "$letters" != "" ]]; then
    >&2 echo "  ERROR: Cyrillic letters found!"
    >&2 echo $(echo "${letters}" | jq -R '.' | jq -sc '.')
    exit 1
  fi
  echo "  OK!"
}

function main() {
  MERGE_REQUEST_DATA=$(request_gitlab_api "merge_requests/${MERGE_REQUEST_ID}/changes" | jq -r '.')
  if [[ $(jq -rc '.labels[] | select ( . == "Content: Cyrillic")' <<< ${MERGE_REQUEST_DATA}) != "" ]]; then
    echo '"Content: Cyrillic" label is present. End.'
    exit 0
  fi

  echo -n "Check title:"
  check_russian_letters "$(jq -r '.title' <<< ${MERGE_REQUEST_DATA})"

  echo -n "Check description:"
  check_russian_letters "$(jq -r '.description' <<< ${MERGE_REQUEST_DATA})"

  echo "Check diff:"
  CHANGES=$(jq -rc '.changes' <<< ${MERGE_REQUEST_DATA})

  for key in $(jq -rc 'keys[]' <<< "${CHANGES}"); do
    FILE_DIFF=$(jq -rc --arg key "$key" '.[$key | tonumber]' <<< "${CHANGES}")

    new_path=$(jq -rc '.new_path' <<< ${FILE_DIFF})

    # skip documentation
    if [[ "$new_path" == *".md" ]]; then
      echo "  * skip: $new_path"
      continue
    fi

    echo -n "  * diff: $new_path"
    check_russian_letters "$(jq -rc '.diff' <<< ${FILE_DIFF} | grep -Ev '-')"
  done
}

PROJECT_ID=$1
MERGE_REQUEST_ID=$2
JOB_TOKEN=$3

main
