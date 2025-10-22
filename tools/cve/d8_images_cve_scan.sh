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

# This script will perform CVE scan for a Deckhouse in DEV registry for provided tag and show reports to output.
#
# Usage: OPTION=<value> make cve-report
#
# $IMAGE - Deckhouse image (by default: dev-registry.deckhouse.io/sys/deckhouse-oss)
# $TAG - Deckhouse image tag (by default: main)
# $SEVERITY - output only entries with specified severity levels (by default: UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)
# TRIVYIGNORE - path to trivy .trivyignore (by default: empty file is used)

TRIVY_PROJECT_ID="2181"
TRIVY_DB_URL="dev-registry.deckhouse.io/sys/deckhouse-oss/security/trivy-db:2"
TRIVY_JAVA_DB_URL="dev-registry.deckhouse.io/sys/deckhouse-oss/security/trivy-java-db:1"
TRIVY_POLICY_URL="dev-registry.deckhouse.io/sys/deckhouse-oss/security/trivy-bdu:1"

if [ -z "$IMAGE" ]; then
  IMAGE="dev-registry.deckhouse.io/sys/deckhouse-oss"
fi

if [ -z "$TAG" ]; then
  echo "WARNING: env variable TAG is not set! Will use 'main'"
  TAG="main"
fi

if [ -z "$SEVERITY" ]; then
  SEVERITY="UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"
fi

if [ -z "$TRIVYIGNORE" ]; then
  TRIVYIGNORE="$(mktemp)"
fi

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

for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
  MODULE_NAME=$(jq -rc '.key' <<< "$module")
  echo "=============================================="
  echo "🛰 Module: $MODULE_NAME"
  for module_image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
    IMAGE_NAME="$(jq -rc '.key' <<< "$module_image")"
    IMAGE_HASH="$(jq -rc '.value' <<< ${module_image})"
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
    if [ "${additional_image_detected}" == true ]; then
      trivy i --policy "${TRIVY_POLICY_URL}" --java-db-repository "${TRIVY_JAVA_DB_URL}" --db-repository "${TRIVY_DB_URL}" --exit-code 0 --severity "${SEVERITY}" --ignorefile "${TRIVYIGNORE}" --format table --scanners vuln --quiet "${IMAGE}:${TAG}"
    else
      trivy i --policy "${TRIVY_POLICY_URL}" --java-db-repository "${TRIVY_JAVA_DB_URL}" --db-repository "${TRIVY_DB_URL}" --exit-code 0 --severity "${SEVERITY}" --ignorefile "${TRIVYIGNORE}" --format table --scanners vuln --quiet "${IMAGE}@${IMAGE_HASH}"
    fi
  done
done
