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
  SEVERITY="CRITICAL,HIGH"
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
  mkdir -p out/
  htmlReportHeader > out/base-images.html

  for image in $BASE_IMAGES; do
    # Some of our base images contain no layers.
    # Trivy cannot scan such images because docker never implemented exporting them.
    # We should not attempt to scan images that cannot be exported.
    # Fixes https://github.com/deckhouse/deckhouse/issues/5020
    docker manifest inspect $image | jq -e '.layers | length > 0' > /dev/null || continue

    echo "----------------------------------------------"
    echo "👾 Image: $image"
    echo ""

    trivyGetCVEListForImage -r "$REGISTRY" -i "$image" > "$WORKDIR/$(echo "$image" | tr "/" "_").cve"
    trivyGetHTMLReportPartForImage -r "$REGISTRY" -i "$image" -l "$(echo "$image" | cut -d@ -f1)" >> out/base-images.html
  done

  find "$WORKDIR" -type f -exec cat {} + | uniq | sort > out/.trivyignore
  rm -r "$WORKDIR"
  htmlReportFooter >> out/base-images.html
}

__main__
