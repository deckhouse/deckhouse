#!/bin/bash

function common_hooks::https::ensure_crds::config() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: crds
      group: main
      apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      keepFullObjectsInMemory: false
      executeHookOnEvent: []
      jqFilter: '.metadata.name'

EOF
}

function common_hooks::https::ensure_crds::main() {
  custom_fields_regexp="(x-description|x-doc-default)"

  crds=$(for file in "$@"; do
    echo "---";
    # Prune custom fields
    cat "$file"
  done)

  readarray -t -d $'\n' crds_json < <(yq r -d '*' - --tojson <<<"$crds" \
    | jq -rc --arg regex "$custom_fields_regexp" '
      .[] | select(.)
      | walk(
        if type == "object"
        then with_entries(
          select(.key | test($regex) | not)
        )
        else . end)')

  for crd in "${crds_json[@]}"; do
    crd_name="$(jq -er '.metadata.name' <<< "$crd")"

    echo "$crd_name"
    context::jq --arg name "$crd_name" '.snapshots.crds[].filterResult | select(contains($name))'
    if context::jq -e --arg name "$crd_name" '.snapshots.crds[].filterResult | select(contains($name))' >/dev/null; then
      apiVersion="$(jq -er '.apiVersion' <<< "$crd")"
      kubernetes::merge_patch "" "$apiVersion" "customresourcedefinitions" "$crd_name" <<< "$(jq -er '{"spec": .spec}' <<< "$crd")"
    else
      kubernetes::create_json <<< "$crd"
    fi
  done
}
