#!/bin/bash
function common_hooks::https::apply_crds::config() {
  jo -p configVersion=v1 onStartup=10
}

function common_hooks::https::apply_crds::main() {
  # TODO: switch back to "x-description" once we've converted all CRDs to apiVersion "v1"
  custom_fields_regexp="x-description"

  crds=$(for file in "$@"; do
    echo "---";
    # Prune custom fields
    cat "$file"
  done)

  echo -n "$crds" \
    | yq r -d '*' - --tojson \
    | jq -rc --arg regex "$custom_fields_regexp" '
      .[] | select(.)
      | walk(
        if type == "object"
        then with_entries(
          select(.key | test($regex) | not)
        )
        else . end)' \
    | kubectl apply -f -
}
