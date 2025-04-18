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

{{- if eq .runType "Normal" }}
  {{- if eq .cri "Containerd" }}
  
kubeconfig="/etc/kubernetes/kubelet.conf"
if [ ! -f "$kubeconfig" ]; then
  exit 0
fi

max_attempts=5
node=${D8_NODE_HOSTNAME}

# Check additional configs containerd
if ls /etc/containerd/conf.d/*.toml >/dev/null 2>/dev/null; then
  label="node.deckhouse.io/containerd=custom-config"
else
  #clean label if exist
  label="node.deckhouse.io/containerd-"
fi

until bb-kubectl --kubeconfig $kubeconfig label --overwrite=true node "$node" $label; do
  attempt=$(( attempt + 1 ))
  if [ "$attempt" -gt "$max_attempts" ]; then
    bb-log-error "failed to annotate node $node after $max_attempts attempts"
    exit 1
  fi
  echo "Waiting for annotate node $node (attempt $attempt of $max_attempts)..."
  sleep 5
done
  {{- end  }}
{{- end  }}
