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

# This helper script is looking for *_GITREPO *_VERSION and *_COMMIT_REF variables
# on Dockerfiles and updating them to the latest versions from GitHub
#
# Usage: hack/update.sh [./images]

sed_regex=""
targets="$(grep -rl '^ARG [A-Z_]*_\(VERSION\|COMMIT_REF\)=' $@)"
versions=$(grep -r '^ARG [A-Z_]*_\(VERSION\|COMMIT_REF\)=' $targets | awk '{print $NF}' | sort -u)
gitrepos=$(grep -r '^ARG [A-Z_]*_GITREPO=' $targets | awk '{print $NF}' | sort -u)

while read name repo; do
  shortrepo=$(echo "$repo" | awk -F/ '{print $(NF-1) "/" $NF}')
  if echo "$versions" | grep -q "^${name}_VERSION="; then
    echo -n "Checking $shortrepo tag: "
    if [ "$name" = DRBD ]; then
      # Convert drbd-9.X.X to v9.X.X and select the latest 9 version
      current_tag=$(curl -fLsS "https://api.github.com/repos/${shortrepo}/tags" | jq -r '.[] | .name' | sed -n 's|drbd-9|v9|p' | sort -V | tail -n1)
    else
      current_tag=$(curl -fLsS "https://api.github.com/repos/${shortrepo}/tags" | jq -r '.[0].name')
    fi
    echo "$current_tag"
    sed_regex="$(printf "%s\n" "$sed_regex" "s|\(${name}_VERSION=\).*|\1${current_tag#*v}|")"
  fi
  if echo "$versions" | grep -q "^${name}_COMMIT_REF="; then
    echo -n "Checking $shortrepo commit: "
    current_sha=$(curl -fLsS "https://api.github.com/repos/${shortrepo}/commits" | jq -r '.[0].sha')
    echo "$current_sha"
    sed_regex="$(printf "%s\n" "$sed_regex" "s|\(${name}_COMMIT_REF=\).*|\1${current_sha}|")"
  fi
done < <(echo "$gitrepos" | sed -n 's|_GITREPO=| |p')

echo "Applying changes:"
set -x
sed -e "$sed_regex" -i $targets
