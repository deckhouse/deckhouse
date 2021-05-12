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

  cluster_crds="$(kubectl get crds -o json | jq '.items[]')"

  for crd in "${crds_json[@]}"; do
    crd_name="$(jq -er '.metadata.name' <<< "$crd")"

    if cluster_crd="$(jq -re --arg name "$crd_name" 'select(.metadata.name | contains($name)) | select(.spec.conversion)' <<< "$cluster_crds")"; then
      crd="$(jq -re --slurpfile cluster_crd <(printf "%s" "$cluster_crd") '.spec.conversion = $cluster_crd[0].spec.conversion' <<<"$crd")"
    fi

    kubernetes::replace_or_create_json <<< "$crd"
  done
}
