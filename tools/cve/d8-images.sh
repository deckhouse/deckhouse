#!/bin/bash

# Copyright 2022 Flant JSC
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
shopt -s failglob

source tools/cve/trivy-wrapper.sh

# This script makes full CVE scan for a Deckhouse release.
#
# Usage: OPTION=<value> release.sh
#
# $REPO - Deckhouse images repo
# $TAG - Deckhouse image tag (by default: the latest tag)
# $SEVERITY - output only entries with specified severity levels (UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)
# $HTML - prepare *.tar.gz report artifact (path to generated artifact expected)

if [ -z "$REPO" ]; then
  REPO="registry.deckhouse.io/deckhouse/ce"
fi

if [ -z "$TAG" ]; then
  TAG="$(git tag --list "v*" | tail -1)"
fi

if [ -z "$SEVERITY" ]; then
  SEVERITY="CRITICAL,HIGH"
fi

function __main__() {
  echo "Deckhouse image to check: $REPO:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$REPO:$TAG"
  digests=$(docker run --rm "$REPO:$TAG" cat /deckhouse/modules/images_digests.json)

  HTML_TEMP=$(mktemp -d)
  IMAGE_REPORT_NAME="deckhouse::$(echo "$REPO:$TAG" | sed 's/^.*\/\(.*\)/\1/')"
  mkdir -p out/
  htmlReportHeader > out/d8-images.html
  TITLE=$IMAGE_REPORT_NAME IMAGE=$REPO TAG=$TAG IGNORE=out/.trivyignore SEVERITY=$SEVERITY trivyGetHTMLReportPartForImage >> out/d8-images.html

  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    echo "=============================================="
    echo "🛰 Module: $MODULE_NAME"

    for image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
      IMAGE_NAME=$(jq -rc '.key' <<< "$image")
      echo "----------------------------------------------"
      echo "👾 Image: $IMAGE_NAME"
      echo ""

      IMAGE_HASH="$(jq -rc '.value' <<<"$image")"
      IMAGE="$REPO@$IMAGE_HASH"
      IMAGE_REPORT_NAME="$MODULE_NAME::$IMAGE_NAME"
      TITLE=$IMAGE_REPORT_NAME IMAGE=$IMAGE IGNORE=.trivyignore SEVERITY=$SEVERITY TAG='' trivyGetHTMLReportPartForImage >> out/d8-images.html
    done
  done

  rm -r "$HTML_TEMP"
  htmlReportFooter >> out/d8-images.html
}

__main__
