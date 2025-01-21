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

  IMAGE_REPORT_NAME="deckhouse::$(echo "$IMAGE:$TAG" | sed 's/^.*\/\(.*\)/\1/')"
  mkdir -p out/json

  date_iso=$(date -I)

  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    touch out/${MODULE_NAME}_report
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

      # Output reports per images
      trivyGetJSONReportPartForImage -l "$IMAGE_REPORT_NAME" -i "$IMAGE@$IMAGE_HASH" -s "$SEVERITY" --ignore "out/.trivyignore" --output "out/json/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json"
      echo ""
      echo " Uploading trivy CVE report for image ${IMAGE_NAME} of ${MODULE_NAME} module"
      echo ""
      curl -s -X POST \
        http://${DEFECTDOJO_HOST}/api/v2/reimport-scan/ \
        -H "accept: application/json" \
        -H "Content-Type: multipart/form-data"  \
        -H "Authorization: Token ${DEFECTDOJO_API_TOKEN}" \
        -F "auto_create_context=True" \
        -F "minimum_severity=Info" \
        -F "active=true" \
        -F "verified=true" \
        -F "scan_type=Trivy Scan" \
        -F "close_old_findings=false" \
        -F "push_to_jira=false" \
        -F "file=@out/json/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" \
        -F "product_type_name=Deckhouse images" \
        -F "product_name=Deckhouse" \
        -F "scan_date=${date_iso}" \
        -F "engagement_name=CVE Test: Deckhouse Images" \
        -F "service=${MODULE_NAME}" \
        -F "group_by=component_name+component_version" \
        -F "lead=1" \
        -F "deduplication_on_engagement=false" \
        -f "tags=[${MODULE_NAME}, ${IMAGE_NAME}]" \
        -F "test_title=${MODULE_NAME}: ${IMAGE_NAME}" \
      > /dev/null

    done
#    # Create an issue with found vulnerabilities
#    CODEOWNERS_MODULE_NAME=$(echo $MODULE_NAME|sed -s 's/[A-Z]/-&/g')
#    owners="[\"Nikolay1224\"]" # Default assignee in case if not found in CODEOWNERS file
#
#    while IFS="\n" read -r line; do
#      if echo $line| grep -i -q "$CODEOWNERS_MODULE_NAME"; then
#        owners=$(echo $line | cut -d "@" -f 2-|jq --raw-input 'split(" @")')
#        owner_found=true
#        break
#      fi
#    done < .github/CODEOWNERS


#    for line in $(cat ./.github/CODEOWNERS); do
#      echo " DEBUG"
#      echo "CODEOWNERS_MODULE_NAME: $CODEOWNERS_MODULE_NAME"
#      echo "line: $line"
#      if echo $line| grep -i -q "$CODEOWNERS_MODULE_NAME"; then
#        owners=$(echo $line | cut -d "@" -f 2-|jq --raw-input 'split(" @")')
#        owner_found=true
#        break
#      fi
#    done
    

  done

}

__main__
