# Copyright 2023 Flant JSC
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

availability=""
attempts=50

# With sleep timeout of 30s, we have 25 minutes period in total to catch the 100% availability from upmeter
for i in $(seq $attempts); do
  # Sleeping at the start for readability. First iterations do not succeed anyway.
  sleep 30
  alerts=$(kubectl get clusteralerts -o jsonpath='{range .items[*].alert}{.name}{"\n"}{end}')
  if [[ $alerts = "DeadMansSwitch" ]]; then
    echo "Alerts include only DeadMansSwitch. All is ok"
    exit 0
  else
    echo "ERROR: More than 1 alert. Alerts:"
    echo $alerts
  fi
  sleep 10
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
