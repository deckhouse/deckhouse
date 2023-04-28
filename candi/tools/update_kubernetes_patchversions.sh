#!/usr/bin/env bash

# Copyright 2021 Flant JSC
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

. functions.sh

function check_requirements() {
    check_jq
    check_crane
    check_yq
}

# Updates version map with new patchversion
# update_version_map VERSION PATCH
function update_version_map() {
  yq -i e ".k8s.\"${1}\".patch = ${2}" ../version_map.yml
}

check_requirements

for VERSION in $(yq e ../version_map.yml -o json | jq -r '.k8s | keys[]'); do
  for PATCH in $(yq e ../version_map.yml -o json | jq -r --arg version "${VERSION}" '.k8s."\($version)".patch'); do
    # Get last patch version from github CHANGELOG.md
    NEW_FULL_VERSION="$(curl -s "https://raw.githubusercontent.com/kubernetes/kubernetes/master/CHANGELOG/CHANGELOG-${VERSION}.md" | grep '## Downloads for v' | head -n 1 | grep -Eo "${VERSION}.[0-9]+")"
    NEW_PATCH="$(awk -F "." '{print $3}' <<< "${NEW_FULL_VERSION}")"
    if [[ "${NEW_PATCH}" -ne "${PATCH}" ]]; then
      PR_DESCRIPTION="${PR_DESCRIPTION}$(echo -e "\n* New kubernetes patch version ${VERSION}.${NEW_PATCH}.")"
      CREATE_PR=true
      update_version_map "${VERSION}" "${NEW_PATCH}"
    fi
  done
done
