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
# If there is no kubelet.conf than node is not bootstrapped and there is nothing to do
kubeconfig="/etc/kubernetes/kubelet.conf"
if [ ! -f "$kubeconfig" ]; then
  exit 0
fi

node=$(bb-d8-node-name)
cgroup="$(stat -fc %T /sys/fs/cgroup)" || {
  bb-log-error "failed to get cgroup version from node $node"
  exit 1
}

if [[ "$cgroup" != "cgroup2fs" && "$cgroup" != "tmpfs" ]]; then
  cgroup="unknown"
fi

max_attempts=5
until bb-kubectl --kubeconfig $kubeconfig label --overwrite=true node "$node" node.deckhouse.io/cgroup="$cgroup"; do
  attempt=$(( attempt + 1 ))
  if [ "$attempt" -gt "$max_attempts" ]; then
    bb-log-error "failed to label node $node after $max_attempts attempts"
    exit 1
  fi
  echo "Waiting for label node $node (attempt $attempt of $max_attempts)..."
  sleep 5
done
{{- end  }}
