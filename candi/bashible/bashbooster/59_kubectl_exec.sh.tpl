{{- /*
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
*/}}

bb-kubectl-exec() {
  local kubeconfig="/etc/kubernetes/kubelet.conf"
  local args=""
{{ if eq .runType "Normal" }}
  local kube_server
  kube_server=$(kubectl --kubeconfig="$kubeconfig" config view -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null)
  if [[ -n "$kube_server" ]]; then
    host=$(echo "$kube_server" | sed -E 's#https?://([^:/]+).*#\1#')
    port=$(echo "$kube_server" | sed -E 's#https?://[^:/]+:([0-9]+).*#\1#')
    # checking local kubernetes-api-proxy availability
    if ! (echo > /dev/tcp/"$host"/"$port") 2>/dev/null; then
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        host=$(echo "$server" | cut -d: -f1)
        port=$(echo "$server" | cut -d: -f2)
        # select the first available control plane
        if (echo > /dev/tcp/"$host"/"$port") 2>/dev/null; then
          args="--server=https://$server"
          break
        fi
      done
    fi
  fi
{{ end }}
  kubectl --request-timeout 60s --kubeconfig=$kubeconfig $args ${@}
}
