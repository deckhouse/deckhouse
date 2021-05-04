#!/bin/bash

function common_hooks::https::ensure_crds::config() {
  cat << EOF
    configVersion: v1
    onStartup: 10
EOF
}

function common_hooks::https::ensure_crds::main() {
  custom_fields_regexp="(x-description|x-doc-default)"

  crds=$(for file in "$@"; do
    name=$(basename -- "$file")
    if [[ $name == doc-* ]]; then
      continue
    fi
    
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

  cluster_crds="$(kubectl get crds -o json | jq '.items[].metadata.name')"

  for crd in "${crds_json[@]}"; do
    crd_name="$(jq -er '.metadata.name' <<< "$crd")"

    echo "$crd_name"
    jq --arg name "$crd_name" 'select(contains($name))' <<< "$cluster_crds"

    if jq -e --arg name "$crd_name" 'select(contains($name))' <<< "$cluster_crds" >/dev/null; then
      apiVersion="$(jq -er '.apiVersion' <<< "$crd")"
      kubernetes::merge_patch "" "$apiVersion" "customresourcedefinitions" "$crd_name" <<< "$(jq -er '{"spec": .spec}' <<< "$crd")"
    else
      kubernetes::create_json <<< "$crd"
    fi
  done
}
