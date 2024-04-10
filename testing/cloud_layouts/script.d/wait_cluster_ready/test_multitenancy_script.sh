# Copyright 2024 Flant JSC
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
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail

function pause-the-test() {
  while true; do
    if ! { kubectl get configmap pause-the-test -o json | jq -re '.metadata.name == "pause-the-test"' >/dev/null ; }; then
      break
    fi

    >&2 echo 'Waiting until "kubectl delete cm pause-the-test" before destroying cluster'

    sleep 30
  done
}

trap pause-the-test EXIT

# Sleeping at the start for readability.
attempts=50
sync=""
for i in $(seq $attempts); do
  sleep 30
  sync="true"

  kubectl get projects.deckhouse.io -o json | jq -cr '.items[] | .status.sync' | while read status; do
    if ! $status; then
      sync="false"
      >&2 echo -n "project status sync false"
    fi
  done
  cat <<EOF

Multitenancy status check: $([ "$sync" == "true" ] && echo "success" || echo "pending")
EOF

  if [[ "$sync" == "true" ]]; then
    exit 0
  fi
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1