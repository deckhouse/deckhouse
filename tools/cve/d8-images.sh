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
  SEVERITY="UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"
fi

function __main__() {
  echo "Deckhouse image to check: $IMAGE:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$IMAGE:$TAG"
  digests=$(docker run --rm "$IMAGE:$TAG" cat /deckhouse/modules/images_digests.json)

  # Additional images to scan
  declare -a additional_images=("dev-registry.deckhouse.io/sys/deckhouse-oss" 
                "dev-registry.deckhouse.io/sys/deckhouse-oss/install"
                "dev-registry.deckhouse.io/sys/deckhouse-oss/install-standalone"
                )
  for additional_image in "${additional_images[@]}"; do
    additional_image_name=$(echo "$additional_image" | grep -o '[^/]*$')
    digests=$(echo "$digests"|jq --arg i "$additional_image_name" --arg s "$TAG" '.deckhouse += { ($i): ($s) }')
  done

  IMAGE_REPORT_NAME="deckhouse::$(echo "$IMAGE:$TAG" | sed 's/^.*\/\(.*\)/\1/')"
  mkdir -p out/json

  date_iso=$(date -I)

  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    touch out/${MODULE_NAME}_report
    echo "=============================================="
    echo "🛰 Module: $MODULE_NAME"

    # Get codeowners to fill defectDojo tags
    CODEOWNERS_MODULE_NAME="$(echo $MODULE_NAME|sed -s 's/[A-Z]/-&/g')"
    codeowner_tags=""
    # Search module number if any
    if ls -1 modules/ |grep -i "^[0-9]*-${CODEOWNERS_MODULE_NAME}$"; then
      # As we know module number - lets search with it
      CODEOWNERS_MODULE_NAME=$(ls -1 modules/ |grep -i "^[0-9]*-${CODEOWNERS_MODULE_NAME}$")
      while IFS="\n" read -r line; do
        search_pattern=$(echo "$line"| sed 's/^\///'|cut -d '/' -f 1)
        if echo ${CODEOWNERS_MODULE_NAME} | grep -i -q "$search_pattern"; then
          for owner_name in $(echo "${line#*@}"); do
            codeowner_tags="${codeowner_tags},codeowner:${owner_name#*@}"
          done
          break
        fi
      done < .github/CODEOWNERS
    else
      # As we dont have module number - also cut it from search pattern
      while IFS="\n" read -r line; do
        # 'sed' will cut "/" before folder name if exist, 'cut' will get dirname that will be used as regexp for current module_name, then cut digits from module name
        search_pattern=$(echo "$line"| sed 's/^\///'|cut -d '/' -f 1|sed 's/^[0-9]*-//')
        if echo ${CODEOWNERS_MODULE_NAME} | grep -i -q "$search_pattern"; then
          for owner_name in $(echo "${line#*@}"); do
            codeowner_tags="${codeowner_tags},codeowner:${owner_name#*@}"
          done
          break
        fi
      done < .github/CODEOWNERS
    fi

    # Set default codeowner in case if not found in CODEOWNERS file
    if [ -z "${codeowner_tags}" ]; then
      codeowner_tags=",codeowner:RomanenkoDenys"
    fi

    for module_image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
      IMAGE_NAME=$(jq -rc '.key' <<< "$module_image")
      if [[ "$IMAGE_NAME" == "trivy" ]]; then
        continue
      fi
      # Set flag if additional image to use tag instead of hash
      additional_image_detected=false
      for image_item in "${additional_images[@]}"; do
        if [ "$IMAGE_NAME" == $(echo "$image_item"| grep -o '[^/]*$') ]; then
          additional_image_detected=true
          break
        fi
      done

      echo "----------------------------------------------"
      echo "👾 Image: $IMAGE_NAME"
      echo ""

      IMAGE_HASH="$(jq -rc '.value' <<< "$module_image")"
      IMAGE_REPORT_NAME="$MODULE_NAME::$IMAGE_NAME"

      # Output reports per images
      if [ "$additional_image_detected" == true ]; then
        trivyGetJSONReportPartForImage -l "$IMAGE_REPORT_NAME" -i "$IMAGE" -t "$TAG" -s "$SEVERITY" --ignore "out/.trivyignore" --output "out/json/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json"
      else
        trivyGetJSONReportPartForImage -l "$IMAGE_REPORT_NAME" -i "$IMAGE@$IMAGE_HASH" -s "$SEVERITY" --ignore "out/.trivyignore" --output "out/json/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json"
      fi
      echo ""
      echo " Uploading trivy CVE report for image ${IMAGE_NAME} of ${MODULE_NAME} module"
      echo ""
      curl -s -X POST \
        https://${DEFECTDOJO_HOST}/api/v2/reimport-scan/ \
        -H "accept: application/json" \
        -H "Content-Type: multipart/form-data"  \
        -H "Authorization: Token ${DEFECTDOJO_API_TOKEN}" \
        -F "auto_create_context=True" \
        -F "minimum_severity=Info" \
        -F "active=true" \
        -F "verified=true" \
        -F "scan_type=Trivy Scan" \
        -F "close_old_findings=true" \
        -F "do_not_reactivate=false" \
        -F "push_to_jira=false" \
        -F "file=@out/json/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" \
        -F "product_type_name=Deckhouse images" \
        -F "product_name=Deckhouse" \
        -F "scan_date=${date_iso}" \
        -F "engagement_name=CVE Test: Deckhouse Images" \
        -F "service=${MODULE_NAME} / ${IMAGE_NAME}" \
        -F "group_by=component_name+component_version" \
        -F "deduplication_on_engagement=false" \
        -F "tags=deckhouse_image,module:${MODULE_NAME},image:${IMAGE_NAME},branch:${TAG}${codeowner_tags}" \
        -F "test_title=[${MODULE_NAME}]: ${IMAGE_NAME}:${TAG}" \
        -F "version=${TAG}" \
        -F "build_id=${IMAGE_HASH}" \
        -F "commit_hash=${GITHUB_SHA}" \
        -F "branch_tag=${TAG}" \
        -F "apply_tags_to_findings=true" \
      > /dev/null
    done
  done
}

__main__

