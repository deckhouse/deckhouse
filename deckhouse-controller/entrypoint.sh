#!/bin/bash

# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o pipefail
set -e

# Prevent starting another instance.
if [[ -n "$DEBUG_UNIX_SOCKET" && -e "$DEBUG_UNIX_SOCKET" ]] ; then
  echo "deckhouse-controller already started"
  exit 1
fi

declare -A bundles_map; bundles_map=( ["Default"]="default" ["Minimal"]="minimal" ["Managed"]="managed" )

bundle=${DECKHOUSE_BUNDLE:-Default}
if [[ ! ${bundles_map[$bundle]+_} ]]; then
    cat <<EOF
{"msg": "-- Deckhouse bundle $bundle doesn't exists! -- Possible bundles: $(for variant in "${!bundles_map[@]}" ; do echo -n " $variant" ; done)"}

EOF
    exit 1
  fi

cat <<EOF
{"msg": "-- Starting Deckhouse using bundle $bundle --"}
EOF

cat ${MODULES_DIR}/values-${bundles_map[$bundle]}.yaml >> ${MODULES_DIR}/values.yaml

exec /sbin/tini -- /usr/bin/deckhouse-controller start
