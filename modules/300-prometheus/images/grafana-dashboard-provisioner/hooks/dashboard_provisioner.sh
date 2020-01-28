#!/bin/bash -e

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
{
  "configVersion": "v1",
  "kubernetes": [
    {
      "name": "dashboard_resources",
      "apiVersion": "deckhouse.io/v1alpha1",
      "kind": "GrafanaDashboardDefinition"
    }
  ]
}
EOF
}

function _grafana_api_get () {
  uri="$1"
  while ! resp="$(curl -s -XGET "http://admin:admin@localhost:3000$uri")"; do
    sleep 1
  done

  echo "$resp"
}

function _grafana_api_delete () {
  uri="$1"
  while ! resp="$(curl -s -XDELETE "http://admin:admin@localhost:3000$uri")"; do
    sleep 1
  done

  echo "$resp"
}

function _grafana_api_post () {
  uri="$1"
  data="$2"
  while ! resp="$(curl -s -H "Content-Type: application/json" -XPOST "http://admin:admin@localhost:3000$uri" -d @- <<< "${data}")"; do
    sleep 1
  done

  echo "$resp"
}

function _grafana_api_get_folder_id() {
  title="$1"
  folders="$(_grafana_api_get /api/folders)"

  if folderId="$(jq -er '.[] | select(.title == "'"${title}"'") | .id' <<< "$folders")"; then
    echo "$folderId"
  else
    newFolder="$(_grafana_api_post /api/folders '{"title": "'"${title}"'"}')"
    jq -r '.id' <<< "${newFolder}"
  fi
}

function _handle_dashboard_resource_add() {
  res_manifest="$1"
  folder="$(jq -r '.spec.folder' <<< "${res_manifest}")"
  definition="$(jq -r '.spec.definition' <<< "${res_manifest}")"

  folderId="$(_grafana_api_get_folder_id "${folder}")"
  _grafana_api_post /api/dashboards/db "$(jq '{"dashboard": ., "folderId": '"${folderId}"', "overwrite": true}' <<< "${definition}")"
}

function _handle_dashboard_resource_del() {
  res_manifest="$1"
  uid="$(jq -r '.spec.definition | fromjson | .uid' <<< "${res_manifest}")"
  _grafana_api_delete "/api/dashboards/uid/${uid}"
}

function __on_kubernetes::dashboard_resources::synchronization() {
  objects_num="$(context::jq -r '.objects | length')"
  for (( i = 0; i < $objects_num; i++ )); do
    _handle_dashboard_resource_add "$(context::get objects.${i}.object)"
  done
}

function __on_kubernetes::dashboard_resources::added_or_modified() {
  _handle_dashboard_resource_add "$(context::get object)"
}

function __on_kubernetes::dashboard_resources::deleted() {
  _handle_dashboard_resource_del "$(context::get object)"
}

hook::run "$@"
