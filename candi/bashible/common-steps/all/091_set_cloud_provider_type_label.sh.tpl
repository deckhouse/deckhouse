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

# bashible: parallel-group=light-labels

mkdir -p /var/lib/node_labels/
{{- with .nodeGroup.cloudProviderType }}
echo "node.deckhouse.io/cloud-provider-type={{ . }}" > /var/lib/node_labels/cloud-provider-type

{{- if ne $.runType "Normal" }}
# 098_update_node_labels, which turns the file above into a label on the Node, is Normal-only.
if [ -f /etc/kubernetes/kubelet.conf ]; then
  node="$(bb-d8-node-name)"
  attempt=0
  max_attempts=5
  until bb-curl-helper-patch-node-metadata "$node" "labels" "node.deckhouse.io/cloud-provider-type={{ . }}"; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -ge "$max_attempts" ]; then
      >&2 echo "Failed to set node.deckhouse.io/cloud-provider-type on node $node after $max_attempts attempts"
      break
    fi
    echo "Retrying to set cloud-provider-type label on node $node (attempt $attempt of $max_attempts)"
    sleep 5
  done
fi
{{- end }}

{{- else }}
rm -f /var/lib/node_labels/cloud-provider-type
{{- end }}
