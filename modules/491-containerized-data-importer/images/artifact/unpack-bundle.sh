#!/bin/sh

# Copyright 2023 Flant JSC
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

DIR=$1

cat "$DIR/manifest.json" | jq -r '.[].RepoTags[0]' | \
  while read image; do
    (set -x; mkdir -p "$image")
    cat "$DIR/manifest.json" | jq -r --arg tag "$image" '.[]| select(.RepoTags[0] == $tag).Layers[]' | \
      while read layer; do
        (set -x; tar -C "$image" --overwrite --exclude='./var/run/*' -xf "$DIR/$layer" .) || true
      done
done
