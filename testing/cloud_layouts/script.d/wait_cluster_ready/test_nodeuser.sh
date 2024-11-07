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

attempts=5

for i in $(seq $attempts); do
  nodeuser_errors=$(kubectl  get nodeusers.deckhouse.io user-e2e -o json | jq -r '.status.errors')
  echo "$nodeuser_errors"

  if [[ $nodeuser_errors == "{}" ]] ; then
    echo "NodeUser 'user-e2e' created successful."
    exit 0
  else
    >&2 echo "NodeUser 'user-e2e' status return error. Attempt $i/$attempts failed. Sleeping 30 seconds..."
    sleep 30
  fi
done

echo 'Timeout waiting for create NodeUser.'
exit 1
