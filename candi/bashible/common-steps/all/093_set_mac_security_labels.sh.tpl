# Copyright 2026 Flant JSC
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

# bashible: parallel-group=light-labels

{{- if eq .runType "Normal" }}
# If there is no kubelet.conf than node is not bootstrapped and there is nothing to do
kubeconfig="/etc/kubernetes/kubelet.conf"
if [ ! -f "$kubeconfig" ]; then
  exit 0
fi

if [[ "${FIRST_BASHIBLE_RUN}" == "yes" ]]; then
  exit 0
fi

# if reboot flag set due to disruption update we pass this step.
if bb-flag? disruption && bb-flag? reboot; then
  exit 0
fi

node=$(bb-d8-node-name)

# Detect SELinux status:
#   enforcing  - SELinux is enabled and enforcing policies
#   permissive - SELinux is enabled but only logging denials
#   disabled   - SELinux kernel module is loaded but disabled
#   absent     - SELinux is not installed on this OS (e.g. Ubuntu family without policycoreutils)
if command -v getenforce >/dev/null 2>&1; then
  selinux="$(getenforce | tr '[:upper:]' '[:lower:]')"
else
  selinux="absent"
fi

# Detect AppArmor status:
#   enabled  - AppArmor is active and enforcing profiles
#   disabled - AppArmor is installed but not active
#   absent   - AppArmor is not installed on this OS (e.g. RHEL/AlmaLinux family)
if command -v aa-status >/dev/null 2>&1; then
  if aa-status --enabled 2>/dev/null; then
    apparmor="enabled"
  else
    apparmor="disabled"
  fi
else
  apparmor="absent"
fi

max_attempts=5

for kv in "node.deckhouse.io/selinux=${selinux}" "node.deckhouse.io/apparmor=${apparmor}"; do
  attempt=0
  until bb-curl-helper-patch-node-metadata "$node" "labels" "$kv"; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$max_attempts" ]; then
      bb-log-error "Failed to set $kv label on node $node after $max_attempts attempts"
      exit 1
    fi
    echo "Retrying to set $kv label on node $node (attempt $attempt of $max_attempts)"
    sleep 5
  done
done
{{- end  }}
