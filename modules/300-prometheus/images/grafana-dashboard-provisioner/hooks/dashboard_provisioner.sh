#!/bin/bash -e

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: dashboard_resources
      apiVersion: deckhouse.io/v1alpha1
      kind: GrafanaDashboardDefinition
      includeSnapshotsFrom:
      - dashboard_resources
      jqFilter: '{"name": .metadata.name, "folder": .spec.folder, "definition": .spec.definition}'
EOF
}

function __main__() {
  mkdir -p /tmp/dashboards-prepare/
  rm -rf /tmp/dashboards-prepare/*

  if ! context::has snapshots.dashboard_resources.0 ; then
    rm -rf /tmp/dashboards/*
    return 0
  fi

  for i in $(context::jq -r '.snapshots.dashboard_resources | keys[]'); do
    dashboard=$(context::get snapshots.dashboard_resources.${i}.filterResult)
    title=$(jq -rc '.definition | fromjson | .title' <<< ${dashboard} | slugify)
    folder=$(jq -rc '.folder' <<< ${dashboard})

    # General folder can't be provisioned, see the link for more details
    # https://github.com/grafana/grafana/blob/3dde8585ff951d5e9a46cfd64d296fdab5acd9a2/docs/sources/http_api/folder.md#a-note-about-the-general-folder
    if [[ "$folder" == "General" ]]; then
      # FIXME: Change folder to "" after updating grafana to version >= 7.1
      #  In grafana >= 7.1 to store dashboard in General folder you must put it into the root of the provisioned folder
      folder="General Folder"
    fi

    mkdir -p "/tmp/dashboards-prepare/${folder}"
    jq -rc '.definition' <<< ${dashboard} > "/tmp/dashboards-prepare/${folder}/${title}.json"
  done

  rm -rf /tmp/dashboards/*
  cp -TR /tmp/dashboards-prepare/ /tmp/dashboards/
}

hook::run "$@"
