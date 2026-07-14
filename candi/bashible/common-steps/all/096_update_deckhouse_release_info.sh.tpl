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

(
set -euo pipefail

tmp_file="$(mktemp)"
target_file="/var/lib/bashible/d8.version"

deckhouse_version="$(
  bb-curl-kube "/apis/deckhouse.io/v1alpha1/deckhousereleases" |
  jq -r '.items[] | select(.status.phase=="Deployed") | .metadata.name'
)"

modules="$(
  bb-curl-kube "/apis/deckhouse.io/v1alpha1/modulereleases" |
  jq '
    .items
    | map(select(.status.phase=="Deployed"))
    | map({key: .spec.moduleName, value: ("v" + .spec.version)})
    | from_entries
  '
)"

if [[ -z "$deckhouse_version" || -z "$modules" ]]; then
  bb-log-error "Failed to get Deckhouse release info"
  exit 0
fi

jq -n \
  --arg deckhouse "$deckhouse_version" \
  --argjson modules "$modules" \
  '{deckhouse: $deckhouse, modules: $modules}' \
  > "$tmp_file"

mv "$tmp_file" "$target_file"

) || true
