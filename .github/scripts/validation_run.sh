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

# This script has just one purpose:
# Fetch inputs for validation script:
#   - title and description of pull request
#   - diff content

set -Eeo pipefail

# Create tmp dir and set traps to clean it after execution.
TMPDIR=$(mktemp -d ./tmp.curl.XXXXXX)
cleanup() {
  echo "Cleanup TMPDIR=${TMPDIR}"
  rm -rfv $TMPDIR
}
trap '(exit 130)' INT
trap '(exit 143)' TERM
trap 'rc=$?; cleanup; exit $rc' EXIT

# Helper to download diff.
function download_diff() {
  CURL_RESPONSE=$TMPDIR/resp
  curlHeaders=$TMPDIR/headers
  curlError=$TMPDIR/error
  # Note: headers for private repo are ignored for public repo.
  CURL_STATUS=$(curl -sS -w %{http_code} \
    --header "Accept: application/vnd.github.diff" \
    --header "Authorization: Bearer ${GITHUB_TOKEN}" \
    -o $CURL_RESPONSE \
    -D $curlHeaders \
    --request GET \
    -L "${DIFF_URL}" 2>$curlError
  )
  CURL_EXIT=$?

  IS_DIFF=$(grep 'diff --git' $CURL_RESPONSE >/dev/null 2>&1 && echo 'yes' || echo 'no')
  CURL_HEADERS=$(sed 's/\r// ; /^$/d' $curlHeaders 2>/dev/null | grep -v -i 'set-cookie\|x-github-request-id\|etag\|content-security-policy')  # remove \r and empty lines, remove cookies
  CURL_ERROR=$(cat $curlError 2>/dev/null)
}

# Check prerequisites: validation script, DIFF_URL
ARG1=$1

if [[ $ARG1 == "--download-only" ]] ; then
  echo "Download only."
  if [[ -z ${DIFF_URL} ]]; then
    echo "No diff url provided: set DIFF_URL."
    exit 1
  fi
  DIFF_PATH=$2
  DOWNLOAD_ONLY=yes
else
  VALIDATION_SCRIPT=$ARG1
  if [[ ! -f $VALIDATION_SCRIPT ]]; then
    echo "Validation script '${VALIDATION_SCRIPT}' is not found."
    exit 1
  fi
  if [[ ! -x $VALIDATION_SCRIPT ]]; then
    echo "Validation script '${VALIDATION_SCRIPT}' is not executable."
    exit 1
  fi

  if [[ "${DIFF_URL}${DIFF_PATH}" == "" ]]; then
    echo "No diff download url or diff path provided: set DIFF_URL or DIFF_PATH."
    exit 1
  fi

  DOWNLOAD_ONLY=no
fi

if [[ -n $DIFF_URL ]] ; then
  # Download diff.
  echo "Fetch changes ..."

  if ! download_diff ; then
    echo "download_diff error: exit $?."
  fi

  if [[ $IS_DIFF == "no" ]] ; then
    echo "Error downloading diff from '${DIFF_URL}'."
    echo "Curl exit code: ${CURL_EXIT}"
    echo "Curl stderr: ${CURL_ERROR}"
    echo "HTTP response:"
    echo "  Last status: ${CURL_STATUS}"
    echo "  Headers: ${CURL_HEADERS}"
    echo "  Body: "
    cat ${CURL_RESPONSE} 2>/dev/null
    exit 1
  fi

  diffFile=${CURL_RESPONSE}
else
  if ! grep 'diff --git' ${DIFF_PATH} >/dev/null 2>&1 ; then
    echo "DIFF_PATH file ${DIFF_PATH} is not diff"
    exit 1
  fi
  diffFile=$DIFF_PATH
fi

if [[ $DOWNLOAD_ONLY == "yes" ]] ; then
  echo "Copy diff to $DIFF_PATH"
  cp $diffFile $DIFF_PATH
  exit
fi


affected=$(grep -c '^diff --git a' "${diffFile}" || true)
removed=$(grep -v '^--- a/' "${diffFile}" | grep -c '^-' || true)
added=$(grep -v '^+++ b/' "${diffFile}" | grep -c '^+' || true)
echo "  diff: ${affected} files affected, ${added} lines added, ${removed} lines removed."

# Run validation script using preinstalled Go.
if ! "${VALIDATION_SCRIPT}" "${diffFile}" ; then
  echo -e "\nFix the problem or use '${SKIP_LABEL_NAME}' PR label to skip."
  exit 1
fi
