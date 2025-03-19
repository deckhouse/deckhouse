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

# This script generates full base image CVE scan report
# for a checked image_versions.yml in `out/` directory in HTML format.
# Also, this script generates `out/.trivyignore` file to exclude found CVEs from `d8-images.sh` scan.
#
# Usage: OPTION=<value> base-images.sh
#
# $SEVERITY - output only entries with specified severity levels (UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)

if [[ "x$SEVERITY" == "x" ]]; then
  SEVERITY="UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"
fi

# Hack to get the base images list.
# We need to figure out the proper way to store this images and avoid template rendering.

function registryPath() {
    grep "REGISTRY_PATH" <<<"$base_images" | awk '{ print $2 }' | tr -d '"'
}

function base_images_tags() {
  base_images=$(grep . "$(pwd)/candi/image_versions.yml") # non empty lines
  base_images=$(grep -v "#" <<<"$base_images")            # remove comments

  reg_path=$(grep "REGISTRY_PATH" <<<"$base_images" | awk '{ print $2 }' | tr -d '"')

  base_images=$(grep -v "REGISTRY_PATH" <<<"$base_images") # Not an image
  base_images=$(grep -v "BASE_GOLANG" <<<"$base_images")   # golang images are used for multistage builds
  base_images=$(grep -v "BASE_JEKYLL" <<<"$base_images")   # images to build docs
  base_images=$(grep -v "BASE_NODE" <<<"$base_images")     # js bundles compilation

  base_images=$(awk '{ print $2 }' <<<"$base_images")                                          # pick an actual images address
  base_images=$(tr -d '"' <<<"$base_images") # "string" -> registry.deckhouse.io/base_images/string

  echo "$reg_path"
  echo "$base_images"
}

function __main__() {
  echo "Severity: $SEVERITY"
  echo ""

  base_images_tags

  WORKDIR=$(mktemp -d)
  BASE_IMAGES_RAW=$(base_images_tags)
  REGISTRY=$(echo "$BASE_IMAGES_RAW" | head -n 1)
  BASE_IMAGES=$(echo "$BASE_IMAGES_RAW" | tail -n +2)
  mkdir -p out/json

  date_iso=$(date -I)

  for image in $BASE_IMAGES; do
    # Some of our base images contain no layers.
    # Trivy cannot scan such images because docker never implemented exporting them.
    # We should not attempt to scan images that cannot be exported.
    # Fixes https://github.com/deckhouse/deckhouse/issues/5020
    MANIFEST=$(echo ${REGISTRY}${image} | sed 's/:[^:@]*@/@/')
    docker manifest inspect $MANIFEST | jq -e '.layers | length > 0' > /dev/null || continue

    echo "----------------------------------------------"
    echo "👾 Image: $image"
    echo ""
    trivyGetCVEListForImage -r "$REGISTRY" -i "$image" > "$WORKDIR/$(echo "$image" | tr "/" "_").cve"

    # Output reports per images
    IMAGE_NAME="$(echo ${image}|cut -d ':' -f 1)"
    IMAGE_TAG="$(echo ${image}|cut -d ':' -f 2|cut -d '@' -f 1)"
    IMAGE_HASH="$(echo ${image}|cut -d '@' -f 2)"
    trivyGetJSONReportPartForImage -r "$REGISTRY" -i "$image" -l "${IMAGE_NAME}:${IMAGE_TAG}" --output "out/json/base_image_${IMAGE_NAME}_report.json"
    echo ""
    echo " Uploading trivy CVE report for base image ${IMAGE_NAME}"
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
      -F "file=@out/json/base_image_${IMAGE_NAME}_report.json" \
      -F "product_type_name=Deckhouse images" \
      -F "product_name=Deckhouse" \
      -F "scan_date=${date_iso}" \
      -F "engagement_name=CVE Test: Base Images" \
      -F "service=Base Image / ${IMAGE_NAME}" \
      -F "group_by=component_name+component_version" \
      -F "deduplication_on_engagement=false" \
      -F "tags=base_image,image:${IMAGE_NAME},codeowner:RomanenkoDenys" \
      -F "test_title=[Base Image]: ${IMAGE_NAME}:${IMAGE_TAG}" \
      -F "version=${IMAGE_TAG}" \
      -F "build_id=${IMAGE_HASH}" \
      -F "commit_hash=${GITHUB_SHA}" \
      -F "branch_tag=${IMAGE_TAG}" \
      -F "apply_tags_to_findings=true" \
    > /dev/null
  done

  find "$WORKDIR" -type f -exec cat {} + | uniq | sort > out/.trivyignore
  rm -r "$WORKDIR"
}

__main__
