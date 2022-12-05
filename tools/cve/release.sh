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

# This script makes full CVE scan for a Deckhouse release.
#
# Usage: OPTION=<value> release.sh
#
# $REPO - Deckhouse images repo
# $TAG - Deckhouse image tag (by default: the latest tag)
# $SEVERITY - output only entries with specified severity levels (UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)


if [[ "x$REPO" == "x" ]]; then
  REPO="registry.deckhouse.io/deckhouse/ce"
fi

if [[ "x$TAG" == "x" ]]; then
  TAG="$(git tag --list "v*" | tail -1)"
fi

if [[ "x$SEVERITY" == "x" ]]; then
  SEVERITY="CRITICAL,HIGH"
fi

function __main__() {
  echo "Deckhouse image to check: $REPO:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$REPO:$TAG"
  tags=$(docker run --rm "$REPO:$TAG" cat /deckhouse/modules/images_tags.json)

  trivy image --timeout 10m --severity=$SEVERITY "$REPO:$TAG"

  for module in $(jq -rc 'to_entries[]' <<< "$tags"); do
    echo "=============================================="
    echo "🛰 Module: $(jq -rc '.key' <<< "$module")"

    for image in $(jq -rc '.value | to_entries[]' <<< "$module"); do
      echo "----------------------------------------------"
      echo "👾 Image: $(jq -rc '.key' <<< "$image")"
      echo ""

      trivy image --timeout 10m --severity=$SEVERITY "$REPO:$(jq -rc '.value' <<< "$image")"
    done
  done
}

__main__
