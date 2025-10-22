# Copyright 2025 Flant JSC
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
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable operator-trivy > /dev/null
kubectl label ns security-scanning.deckhouse.io/enabled="" --all > /dev/null
testRunAttempts=5
for ((i=1; i<=$testRunAttempts; i++)); do
  if kubectl get clustercompliancereports.aquasecurity.github.io cis > /dev/null; then
    break
  else
    sleep 30
  fi
done
kubectl patch clustercompliancereports.aquasecurity.github.io cis --type='json' -p='[{"op": "replace", "path": "/spec/cron", "value": "*/2 * * * *"}]' > /dev/null
testRunAttempts=20
for ((i=1; i<=$testRunAttempts; i++)); do
  FAILED=$(kubectl get clustercompliancereports.aquasecurity.github.io cis -o wide --no-headers 2>/dev/null | awk '{print $4}')
  PASSED=$(kubectl get clustercompliancereports.aquasecurity.github.io cis -o wide --no-headers 2>/dev/null | awk '{print $3}')
  CRAR=$(kubectl get clusterrbacassessmentreports.aquasecurity.github.io |wc -l)
  if [[ -z $FAILED ]]
    then
      FAILED=0
  fi
  if [[ -z $PASSED ]]
    then
      PASSED=0
  fi
  if [[ $PASSED && "$(($PASSED+$FAILED))" -gt '100' && $CRAR -gt 290 ]]; then
    >&2 echo "CIS report is ready"
    break
  else
    >&2 echo "CIS report is still not ready. Attemption: #$i"
    sleep 20
  fi
done
kubectl get clustercompliancereports.aquasecurity.github.io cis -o json |jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
