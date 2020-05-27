#!/bin/bash

source /deckhouse/shell_lib.sh

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: helm_releases
      group: main
      queue: /modules/$(module::name::kebab_case)/helm_releases
      waitForSynchronization: false
      apiVersion: v1
      kind: ConfigMap
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
      },
      "ReplicaSet": {
        "apps/v1beta1": "",
        "apps/v1beta2": "",
        "extensions/v1beta1": ""
      },
      "NetworkPolicy": {
        "extensions/v1beta1": ""
      },
      "PodSecurityPolicy": {
        "extensions/v1beta1": ""
      }
    }
  '


  D8_HELM_HOST="$HELM_HOST"
  metrics=""

  namespaces="$(context::jq -r '.snapshots.helm_releases[].object.metadata.namespace' | sort | uniq)"
  for namespace in $namespaces; do
    if [ "$namespace" != "d8-system" ]; then
      HELM_HOST=""
    else
      HELM_HOST="$D8_HELM_HOST"
    fi
    if release_names="$(helm --tiller-namespace "$namespace" list --output json 2>/dev/null | jq -r '.Releases[].Name')"; then
      for release_name in $release_names; do
        manifest="$(helm --tiller-namespace "$namespace" get manifest "$release_name" | yq read -d"*" -j -)"
        metrics="$metrics\n$(jq -rc --arg release_name "$release_name" --argjson d "$unsupported_versions" '
          .[] | select(.kind != null) |
          {
            "name": "resource_versions_compatibility",
            "set": 0,
            "labels": {
              "helm_release_name": $release_name,
              "resource_name": .metadata.name,
              "kind": .kind,
              "api_version": .apiVersion,
              "namespace": .metadata.namespace
            }
          }
          | . as $in
          | if $d[.labels.kind] != null then if $d[$in.labels.kind] | has($in.labels.api_version) then .set = 1 else . end else . end
        ' <<< "$manifest")"
      done
    fi
  done

  echo -e "$metrics" >> $METRICS_PATH
}

hook::run "$@"
