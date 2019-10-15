#!/bin/bash

set -o pipefail
set -e

declare -A bundles_map; bundles_map=( ["Default"]="default" ["Minimal"]="minimal" )

bundle=${DECKHOUSE_BUNDLE:-Default}
if [[ ! ${bundles_map[$bundle]+_} ]]; then
    cat <<EOF
-- Deckhouse bundle "$bundle" doesn't exists! --

  Possible bundles:
$(for variant in "${!bundles_map[@]}" ; do echo "  - $variant" ; done)

EOF
    exit 1
  fi

echo "-- Starting Deckhouse using bundle \"$bundle\" --"
ln -s ${MODULES_DIR}/values-${bundles_map[$bundle]}.yaml ${MODULES_DIR}/values.yaml

exec /deckhouse/deckhouse-controller "$@"
