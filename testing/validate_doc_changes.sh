#!/bin/bash

set -e

function request_gitlab_api() {
  curl --silent -f -H "PRIVATE-TOKEN: ${JOB_TOKEN}"  https://fox.flant.com/api/v4/projects/${CI_PROJECT_ID}/${1}
}

# $1 - filename we test
# $2 - filename in other language
function checks() {
    if ! [[ -f "${2}" ]]; then
      echo "${1} changed but ${2} is absent"
      return 1
    else
      if [[ -z $(grep "${2}" <<< ${DIFF_DATA}) ]]; then
        echo "${1} changed but ${2} is not changed"
        return 1
      fi
    fi
}

JOB_TOKEN=$1
SKIP_LABEL_NAME='Skip doc validation'
hasErrors=0

if [[ -z $CI_OPEN_MERGE_REQUESTS ]]; then
  echo "There are no merge requests found"
  exit 0
fi

IFS=',' read -r -a MERGE_REQUESTS_ARRRAY <<< "$CI_OPEN_MERGE_REQUESTS"
for MERGE_REQUEST_ID in ${MERGE_REQUESTS_ARRRAY[*]}; do
  MERGE_REQUEST_ID=$( cut -d \! -f 2 <<<${MERGE_REQUEST_ID})
  MERGE_REQUEST_DATA=$(request_gitlab_api "merge_requests/${MERGE_REQUEST_ID}/" | jq -r '.')

  if [[ $(jq -rc --arg label_name "${SKIP_LABEL_NAME}" '.labels[] | select ( . == $label_name)' <<< ${MERGE_REQUEST_DATA}) != "" ]]; then
    echo "Validation skipped...";
    exit 0
  fi
done

DIFF_DATA=$(git diff origin/master... --name-status -w --ignore-blank-lines --diff-filter=ACMD | awk '{print $2}' )

if [[ -z $DIFF_DATA ]]; then
  echo "Empty diff data"
  exit 0
fi

echo

for item in ${DIFF_DATA}; do
    # skip other than .md files
    if ! [[ "$item" == *".md" ]]; then
      continue
    fi
    if [[ "$item" == *"_RU.md" ]]; then
      otherLangFileName=$(sed 's/_RU.md/.md/' <<< $item)
      if ! checks "${item}" "${otherLangFileName}" ; then
        hasErrors=1
      fi
    else
      otherLangFileName=$(sed 's/.md/_RU.md/' <<< $item)
      if ! checks "${item}" "${otherLangFileName}" ; then
        hasErrors=1
      fi
    fi

done

if [[ $hasErrors -gt 0 ]] ; then
  echo -e "\nFix the problem or use '${SKIP_LABEL_NAME}' MR label to skip."
fi

exit $hasErrors