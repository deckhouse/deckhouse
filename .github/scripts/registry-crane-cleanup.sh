#!/usr/bin/env bash

# Copyright 2025 Flant JSC
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

export DOCKER_CONFIG=/tmp/

cd /tmp
# cleanup custom tags
if [ ! -f /tmp/crane ]; then
  VERSION=$(curl -s "https://api.github.com/repos/google/go-containerregistry/releases/latest" | jq -r '.tag_name')
  OS=Linux
  ARCH=x86_64
  curl -sL "https://github.com/google/go-containerregistry/releases/download/${VERSION}/go-containerregistry_${OS}_${ARCH}.tar.gz" > /tmp/go-containerregistry.tar.gz
  tar -zxvf /tmp/go-containerregistry.tar.gz -C /tmp/ crane
fi

function crain_rm {
  local REPO=$1

  local THRESHOLD_EPOCH=$(( $(date +%s) - 14*24*60*60 ))
  local LIST=$(/tmp/crane ls $REPO | grep -E "^(pr|release|main)" | grep -v ^main$)
  if [ -n "$LIST" ]; then
    echo "$LIST" | while read -r line; do
      local CREATED_EPOCH=$(/tmp/crane config $REPO:$line | jq -r '.created | split(".")[0] + "Z" | fromdateiso8601')
      if [ "$CREATED_EPOCH" -lt "$THRESHOLD_EPOCH" ]; then
        echo "old crane delete $REPO:$line"
        /tmp/crane delete $REPO:$line
      fi
    done
  fi
}

crain_rm "${REGISTRY_URL}/install"
crain_rm "${REGISTRY_URL}/install-standalone"
crain_rm "${REGISTRY_URL}/e2e-terraform"
crain_rm "${REGISTRY_URL}/e2e-opentofu-eks"
crain_rm "${REGISTRY_URL}"
