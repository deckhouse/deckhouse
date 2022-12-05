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

# This script generates the report that contains all known CVEs for base images.
# Base images are used to deploy binaries.
# Thus, every CVE found by this script will be present in the full release report multiple times.
#
# Usage: OPTION=<value> base-images.sh
#
# $SEVERITY - output only entries with specified severity levels (UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)

if [[ "x$SEVERITY" == "x" ]]; then
  SEVERITY="CRITICAL,HIGH"
fi

# Hack to get the base images list.
# We need to figure out the proper way to store this images and avoid template rendering.
function base_images_tags {
  base_images=$(grep . $(pwd)/candi/image_versions.yml) # non empty lines
  base_images=$(grep -v "#" <<< "$base_images") # remove comments

  reg_path=$(grep "REGISTRY_PATH" <<< ${base_images} | awk '{ print $2 }' | jq -r)

  base_images=$(grep -v "REGISTRY_PATH" <<< "$base_images") # Not an image
  base_images=$(grep -v "BASE_GOLANG" <<< "$base_images") # golang images are used for multistage builds
  base_images=$(grep -v "BASE_RUST" <<< "$base_images") # rust images are used for multistage builds
  base_images=$(grep -v "BASE_JEKYLL" <<< "$base_images") # images to build docs
  base_images=$(grep -v "BASE_NODE" <<< "$base_images") # js bundles compilation

  base_images=$(awk '{ print $2 }' <<< "$base_images") # pick an actual images address
  base_images=$(jq -sr --arg reg "$reg_path" 'map(. | "\($reg)\(.)") | .[]' <<< "$base_images") # "string" -> registry.deckhouse.io/base_images/string

  echo "$base_images"
}

function __main__() {
  echo "Severity: $SEVERITY"
  echo ""

  for image in $(base_images_tags) ; do
    echo "----------------------------------------------"
    echo "👾 Image: $image"
    echo ""

    trivy image --timeout 10m --severity=$SEVERITY "$image"
  done
}

__main__
