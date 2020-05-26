#!/bin/bash

source /deckhouse/shell_lib.sh

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: helm_releases
      group: main
      apiVersion: v1
      kind: ConfigMap
      namespace:
        nameSelector:
          matchNames: [d8-system]
      labelSelector:
        matchLabels:
          OWNER: TILLER
      jqFilter: .data
EOF
}

function __main__() {
  unsupported_versions='
    {
      "Deployment": {
        "apps/v1beta1": "",
        "apps/v1beta2": "",
        "extensions/v1beta1": ""
      },
      "StatefulSet": {
        "apps/v1beta1": "",
        "apps/v1beta2": ""
      },
      "DaemonSet": {
        "apps/v1beta1": "",
        "apps/v1beta2": "",
        "extensions/v1beta1": ""
      }
    }
  '

  metrics=""

  release_names="$(helm list --output json | jq -r '.Releases[].Name')"
  for release_name in $release_names; do
    manifest="$(helm get manifest "$release_name" | yq read -d"*" -j -)"
    metrics="$metrics\n$(jq -rc --arg release_name "$release_name" --argjson d "$unsupported_versions" '
      .[] | select(.kind != null) |
      {
        "name": "resource_versions_compatibility",
        "set": 0,
        "labels": {
          "helm_release_name": $release_name,
          "resource_name": .metadata.name,
          "kind": .kind,
          "api_version": .apiVersion
        }
      }
      | . as $in
      | if $d[.labels.kind] != null then if $d[$in.labels.kind] | has($in.labels.api_version) then .set = 1 else . end else . end
    ' <<< "$manifest")"
  done

  echo -e "$metrics" >> $METRICS_PATH
}

hook::run "$@"
