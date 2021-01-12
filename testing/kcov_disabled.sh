#!/bin/bash

function request_gitlab_api() {
  curl --silent -f -H "PRIVATE-TOKEN: ${FOX_ACCESS_TOKEN}"  https://fox.flant.com/api/v4/projects/${CI_PROJECT_ID}/${1}
}

function main() {
  export KCOV_DISABLED=yes

  if [[ "${CI_COMMIT_REF_NAME}" == "master" ]]; then
    export KCOV_DISABLED=no
    echo '"Current branch name is master. Enabling kcov.'
    return 0
  fi

  MERGE_REQUEST_ID=$(request_gitlab_api "merge_requests?state=opened" | jq -r --arg c ${CI_COMMIT_SHA} '.[]|select(.sha == $c) | .iid')
  if [[ "$MERGE_REQUEST_ID" == "" ]]; then
    echo "No merge request found for commit sha: ${CI_COMMIT_SHA}"
  else
    echo "Merge Request ID = ${MERGE_REQUEST_ID}"

    MERGE_REQUEST_DATA=$(request_gitlab_api "merge_requests/${MERGE_REQUEST_ID}" | jq -r '.')
    if [[ $(jq -rc '.labels[] | select ( . == "Testing: Kcov Enabled")' <<< ${MERGE_REQUEST_DATA}) != "" ]]; then
      echo '"Testing: Kcov Enabled" label is present. Enabling kcov.'
      export KCOV_DISABLED=no
    fi
  fi
}

main
