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

{{- if not (eq .nodeGroup.nodeType "Static") }}
if [ -f /etc/kubernetes/kubelet.conf ] ; then
  if bb-kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node $HOSTNAME >/dev/null 2>&1 ; then
    bb-log-error "ERROR: A node with the hostname $HOSTNAME already exists in the cluster\nPlease change the hostname, it should be unique in the cluster.\nThen clean up the server by running the script /var/lib/bashible/cleanup_static_node.sh and try again."
  fi
else
  bb-log-error "Error: /etc/kubernetes/kubelet.conf not found"
  exit 1
fi
{{- end }}

