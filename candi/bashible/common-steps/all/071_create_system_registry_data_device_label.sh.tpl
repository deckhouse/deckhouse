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

{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}

function enable_registry_data_device_label() {
  local label="node.deckhouse.io/registry-data-device-ready=true"
  local node="$D8_NODE_HOSTNAME"

  echo "Label node $node with labels $label"
  error=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf label node $node --overwrite $label 2>&1)
  if [ $? -ne 0 ]; then
    >&2 echo "Failed to label node $node. Error from kubectl: ${error}"
    exit 1
  fi
  echo "Successful label node $node with labels $label"
}

function disable_registry_data_device_label() {
  local label="node.deckhouse.io/registry-data-device-ready="
  local node="$D8_NODE_HOSTNAME"

  echo "Label node $node with labels $label"
  error=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf label node $node --overwrite $label 2>&1)
  if [ $? -ne 0 ]; then
    >&2 echo "Failed to label node $node. Error from kubectl: ${error}"
    exit 1
  fi
  echo "Successful label node $node with labels $label"
}

# Skip for
if [ -f /var/lib/bashible/lock_mount_registry_data_device ]; then
  exit 0
fi

# Only one times for first bashible run
if [[ "$FIRST_BASHIBLE_RUN" == "yes" ]]; then
  if [ -f /var/lib/bashible/system-registry-data-device-installed ]; then
    enable_registry_data_device_label
  else
    disable_registry_data_device_label
  fi
fi

  {{- end  }}
{{- end  }}
