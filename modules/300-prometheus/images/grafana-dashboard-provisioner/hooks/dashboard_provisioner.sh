#!/bin/bash -e

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

set -Eeuo pipefail

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat <<EOF
    configVersion: v1
    kubernetes:
    - name: dashboard_resources
      apiVersion: deckhouse.io/v1
      kind: GrafanaDashboardDefinition
      includeSnapshotsFrom:
      - dashboard_resources
      jqFilter: '{"name": .metadata.name, "folder": .spec.folder, "definition": .spec.definition}'
EOF
}

function __main__() {
  tmpDir=$(mktemp -d -t dashboard.XXXXXX)
  existingUidsFile=$(mktemp -t uids.XXXXXX)

  malformed_dashboards=""
  for i in $(context::jq -r '.snapshots.dashboard_resources | keys[]'); do
    dashboard=$(context::get snapshots.dashboard_resources.${i}.filterResult)
    title=$(jq -rc '.definition | try(fromjson | .title)' <<<"${dashboard}")
    if [[ "x${title}" == "x" ]]; then
      malformed_dashboards="${malformed_dashboards} $(jq -rc '.name' <<<"${dashboard}")"
      continue
    fi

    title=$(slugify <<<${title})

    if ! dashboardUid=$(jq -erc '.definition | fromjson | .uid' <<<"${dashboard}"); then
      >&2 echo "ERROR: definition.uid is mandatory field"
      continue
    fi

    if grep -qE "^${dashboardUid}$" ${existingUidsFile}; then
      >&2 echo "ERROR: a dashboard with the same uid is already exist: ${dashboardUid}"
      continue
    else
      echo "${dashboardUid}" >> "${existingUidsFile}"
    fi

    folder=$(jq -rc '.folder' <<<"${dashboard}")
    file="${folder}/${title}.json"

    # General folder can't be provisioned, see the link for more details
    # https://github.com/grafana/grafana/blob/3dde8585ff951d5e9a46cfd64d296fdab5acd9a2/docs/sources/http_api/folder.md#a-note-about-the-general-folder
    if [[ "$folder" == "General" ]]; then
      file="${title}.json"
    fi

    mkdir -p "${tmpDir}/${folder}"
    jq -rc '.definition' <<<"${dashboard}" > "${tmpDir}/${file}"
  done

  if [[ "x${malformed_dashboards}" != "x" ]]; then
    echo "Skipping malformed dashboards: ${malformed_dashboards}"
  fi

  rsync -rq --delete-after "${tmpDir}/" /etc/grafana/dashboards/
  rm -rf ${tmpDir}
  rm ${existingUidsFile}

  echo -n "ok" >/tmp/ready
}

hook::run "$@"
