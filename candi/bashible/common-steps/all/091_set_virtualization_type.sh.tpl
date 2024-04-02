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

{{- if eq .runType "Normal" }}
# If there is no kubelet.conf than node is not bootstrapped and there is nothing to do
kubeconfig="/etc/kubernetes/kubelet.conf"
if [ ! -f "$kubeconfig" ]; then
  exit 0
fi

if [[ "${FIRST_BASHIBLE_RUN}" == "yes" ]]; then
  exit 0
fi

# if reboot flag set due to disruption update (for example, in case of CRI change) we pass this step.
# this step runs normally after node reboot.
if bb-flag? disruption && bb-flag? reboot; then
  exit 0
fi

virtualization="$(virt-what | awk 'FNR <= 1')"
if [[ "$virtualization" == "" ]]; then
  virtualization="unknown"
fi
max_attempts=5
node=${D8_NODE_HOSTNAME}

until bb-kubectl --kubeconfig $kubeconfig annotate --overwrite=true node "$node" node.deckhouse.io/virtualization="$virtualization"; do
  attempt=$(( attempt + 1 ))
  if [ "$attempt" -gt "$max_attempts" ]; then
    bb-log-error "failed to annotate node $node after $max_attempts attempts"
    exit 1
  fi
  echo "Waiting for annotate node $node (attempt $attempt of $max_attempts)..."
  sleep 5
done
{{- end  }}
