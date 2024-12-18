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

# This script generates full CVE scan report for a Deckhouse release in `out/` directory in HTML format.
#
# Usage: OPTION=<value> release.sh
#
# $IMAGE - Deckhouse image (by default: registry.deckhouse.io/deckhouse/ce)
# $TAG - Deckhouse image tag (by default: the latest tag in git)
# $SEVERITY - output only entries with specified severity levels (UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)

if [ -z "$IMAGE" ]; then
  IMAGE="registry.deckhouse.io/deckhouse/ce"
fi

if [ -z "$TAG" ]; then
  TAG="$(git tag --list "v*" | tail -1)"
fi

if [ -z "$SEVERITY" ]; then
  SEVERITY="CRITICAL,HIGH"
fi

function __main__() {
  echo "Deckhouse image to check: $IMAGE:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$IMAGE:$TAG"
  digests=$(docker run --rm "$IMAGE:$TAG" cat /deckhouse/modules/images_digests.json)

  HTML_TEMP=$(mktemp -d)
  IMAGE_REPORT_NAME="deckhouse::$(echo "$IMAGE:$TAG" | sed 's/^.*\/\(.*\)/\1/')"
  mkdir -p out/
  htmlReportHeader > out/d8-images.html
  trivyGetHTMLReportPartForImage -l "$IMAGE_REPORT_NAME" -i "$IMAGE" -t "$TAG" -s "$SEVERITY" --ignore out/.trivyignore >> out/d8-images.html

  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    echo "=============================================="
    echo "🛰 Module: $MODULE_NAME"

    for module_image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
      IMAGE_NAME=$(jq -rc '.key' <<< "$module_image")
      if [[ "$IMAGE_NAME" == "trivy" ]]; then
        continue
      fi
      echo "----------------------------------------------"
      echo "👾 Image: $IMAGE_NAME"
      echo ""

      IMAGE_HASH="$(jq -rc '.value' <<< "$module_image")"
      IMAGE_REPORT_NAME="$MODULE_NAME::$IMAGE_NAME"
      trivyGetHTMLReportPartForImage  -l "$IMAGE_REPORT_NAME" -i "$IMAGE@$IMAGE_HASH" -s "$SEVERITY" --ignore out/.trivyignore >> out/d8-images.html
    done
  done

  rm -r "$HTML_TEMP"
  htmlReportFooter >> out/d8-images.html
}

__main__
