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
function __base_images_tags__ {
  docker run -i --rm \
    -e TARGET_UID=$(id -u) \
    -e TARGET_GID=$(id -g) \
    -e TARGET_UMASK=$(umask) \
    -e TARGET_OSTYPE=${OSTYPE} \
    -v $(pwd)/.github/ci_includes:/in/ci_includes \
    hairyhenderson/gomplate:v3.10.0-alpine \
      --left-delim '{!{' \
      --right-delim '}!}' \
      --plugin echo=/bin/echo \
      --template /in/ci_includes \
      -i '{!{ tmpl.Exec "image_versions_envs" . }!}'
}

function __main__() {
  echo "Severity: $SEVERITY"
  echo ""

  base_images=$(__base_images_tags__)
  base_images=$(grep -v "#" <<< "$base_images") # remove comments
  base_images=$(grep -v "BASE_GOLANG" <<< "$base_images") # golang images are used for multistage builds
  base_images=$(grep -v "BASE_RUST" <<< "$base_images") # rust images are used for multistage builds
  base_images=$(grep -v "BASE_JEKYLL" <<< "$base_images") # images to build docs
  base_images=$(awk '{ print $2 }' <<< "$base_images") # pick an actual images address
  base_images=$(jq -sr '.[]' <<< "$base_images") # unwrap quotes "string" -> string

  for image in $base_images ; do
    echo "----------------------------------------------"
    echo "👾 Image: $image"
    echo ""

    trivy image --timeout 10m --severity=$SEVERITY "$image"
  done
}

__main__
