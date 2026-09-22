#!/usr/bin/env bash

# Copyright 2026 Flant JSC
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

TAGS=$(git ls-remote --tags origin | grep -F tags/v | awk -F '/' '{print $3}' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$')
TAGS_TO_SCAN=$(echo $TAGS | tr ' ' '\n' | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | awk -F. '{
if ($2 != prev_minor) {
    if (prev_minor != "") {
    print "v" $1 "." prev_minor "." max_patch
    }
    prev_minor = $2
    max_patch = $3
} else if ($3 > max_patch) {
    max_patch = $3
    }
}
END { print "v" $1 "." prev_minor "." max_patch }' | tail -3)
TAGS=""
for edition in fe ee se se-plus be ce cse
do
    for tag in $TAGS_TO_SCAN
    do
    TAGS+=$'\n'"$edition:$tag"
    done
done

TAGS+=$'\n'"main"
SCAN_TAGS="$(first=1; printf '['; while IFS= read -r tag; do [ -z "$tag" ] && continue; tag=${tag//\\/\\\\}; tag=${tag//\"/\\\"}; [ $first -eq 0 ] && printf ','; printf '"%s"' "$tag"; first=0; done <<< "$TAGS"; printf ']')"
echo "Tags to scan: $TAGS"
echo "SCAN_TAGS to scan: $SCAN_TAGS"
# Forming compact json
printf 'SCAN_TAGS=%s\n' "$SCAN_TAGS" > tags.env