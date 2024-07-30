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
attempts=20

allow_alerts=(
"D8DeckhouseIsNotOnReleaseChannel" # Tests may be made on dev branch
"DeadMansSwitch" # Always active in system. Tells that monitoring works.
"CertmanagerCertificateExpired" # On some system do not have DNS
"CertmanagerCertificateExpiredSoon" # Same as above
"DeckhouseModuleUseEmptyDir" # TODO Need made split storage class
)

# In e2e tests with OS on older cores (AWS, Azure), ebpf_exporter does not initiliaze. Ignore this alerts
kernelVersion=$(uname -r | cut -c 1)$(uname -r | cut -c 3,4)
if [[ ${kernelVersion} < 508 ]]; then # Min kernel for ebpf exporter is 5.08
  allow_alerts+=("D8NodeHasUnmetKernelRequirements" "KubernetesDaemonSetReplicasUnavailable")
fi

# With sleep timeout of 30s, we have 25 minutes period in total to catch the 100% availability from upmeter
for i in $(seq $attempts); do
  # Sleeping at the start for readability. First iterations do not succeed anyway.
  sleep 30

  # Get alert names with kubectl command
  kube_alerts=$(kubectl get clusteralerts -o jsonpath='{range .items[*].alert}{.name}{" "}{end}')
  # Split the kube_alerts into an array of alerts
  IFS=' ' read -ra alerts <<< "$kube_alerts"

  # Loop through each alert in the output
  alerts_is_ok=true
  for alert in "${alerts[@]}"; do
    # Check if the alert is in the allow list
    if ! [[ " ${allow_alerts[@]} " =~ " ${alert} " ]]; then
      echo "Error: Unexpected alert: '$alert'"
      alerts_is_ok=false
    else
      echo "Alert '$alert' ignored"
    fi
  done
  if [[ $alerts_is_ok = true ]]; then
    echo "All alerts are in the allow list."
    exit 0
  fi

done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
