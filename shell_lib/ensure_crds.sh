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

  for crd_json in "${crds_json[@]}"; do
    kubernetes::replace_or_create_json <<< "$crd_json"
  done
}
