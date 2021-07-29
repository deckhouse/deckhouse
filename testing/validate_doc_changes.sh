#!/bin/bash

# Copyright 2021 Flant CJSC
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

function request_gitlab_api() {
  curl --silent -f -H "PRIVATE-TOKEN: ${JOB_TOKEN}"  https://fox.flant.com/api/v4/projects/${CI_PROJECT_ID}/${1}
}

# $1 - filename we test
# $2 - filename in other language
function checks() {
    if ! [[ -f "${2}" ]]; then
      echo "warn: ${1} changed but ${2} is absent"
      return 1
    else
      if [[ -z $(grep "${2}" <<< ${DIFF_DATA}) ]]; then
        echo "ERROR: ${1} changed but ${2} is not changed"
        return 1
      fi
    fi
}

function check_doc() {
    filename=${1}

    docs_pattern="modules/[^/]+/docs/|docs/(site|documentation)/pages"
    if ! [[ "$filename" =~ $docs_pattern ]]; then
        echo "skip ${filename}"
        return 0
    fi

    docs_pattern="modules/[^/]+/docs/(CLUSTER_CONFIGURATION|CONFIGURATION|CR|ISTIO-CR|FAQ|README|USAGE)(_RU)?.md"

    if [[ "$filename" =~ /docs/[^/]+.md ]] && ! [[ "$filename" =~ $docs_pattern ]]; then
        echo "ERROR: $filename is not allowed. You can place it in the 'internal' folder for example."
        cat <<EOF
    Only the following files (and their Russian versions with _RU.md suffux) are allowed in the module '/docs/' folder:
        CONFIGURATION.md
        CR.md
        FAQ.md
        README.md
        USAGE.md
EOF
        return 1
    fi

    if [[ "$filename" == *"_RU.md" ]]; then
      otherLangFileName=$(sed 's/_RU.md/.md/' <<< $filename)
      if ! checks "${filename}" "${otherLangFileName}" ; then
        return 1
      fi
    else
      otherLangFileName=$(sed 's/.md/_RU.md/' <<< $filename)
      if ! checks "${filename}" "${otherLangFileName}" ; then
        return 1
      fi
    fi
}

function check_resource() {
    filename=${1}

    pattern_ru="doc-ru-.+.y[a]?ml$"
    if [[ "$filename" =~ $pattern_ru ]]; then
      otherLangFileName=${filename/doc-ru-/}
      if ! checks "${filename}" "${otherLangFileName}" ; then
        return 1
      fi
    else
      otherLangFileName=$(sed -E 's#([^/]+\.y[a]?ml)$#doc-ru-\1#' <<< $filename)
      if ! checks "${filename}" "${otherLangFileName}" ; then
        return 1
      fi
    fi
}

JOB_TOKEN=$1
SKIP_LABEL_NAME='Skip doc validation'
hasErrors=0
pattern_resources="openapi/config-values.y[a]?ml$|crds/.+.y[a]?ml$"
pattern_yaml=".+\.y[a]?ml$"

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

DIFF_DATA=$(git diff origin/main... --name-status -w --ignore-blank-lines --diff-filter=ACMD | awk '{print $2}' )

if [[ -z $DIFF_DATA ]]; then
  echo "Empty diff data"
  exit 0
fi

echo

for item in ${DIFF_DATA}; do
    # skip other than .md files and resource files (openapi specs and crds)
    if [[ "$item" == *".md" ]]; then
        if ! check_doc "${item}" ; then
            hasErrors=1
        fi
    elif [[ "$item" =~ $pattern_resources ]]; then
        if ! check_resource "${item}" ; then
            hasErrors=1
        fi
    else
        continue
    fi
done

if [[ $hasErrors -gt 0 ]] ; then
  echo -e "\nFix the problem or use '${SKIP_LABEL_NAME}' MR label to skip."
fi

exit $hasErrors
