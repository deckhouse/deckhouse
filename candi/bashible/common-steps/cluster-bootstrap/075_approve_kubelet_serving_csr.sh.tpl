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

# Approve the first kubelet-serving CSR during the very first bootstrap so
# that kube-apiserver can reach the kubelet over its node IPs.
#
# Normally kubelet-serving CSRs are approved by the node-manager hook running
# inside the deckhouse pod, after the self-signed kubelet certificate keeps
# kube-apiserver happy via its hostname SAN. On dual-stack clusters that
# fallback no longer works: kube-apiserver picks the InternalIP, the
# self-signed certificate has no IP SAN, and TLS validation fails. The
# deckhouse pod cannot become Ready, so the node-manager hook never runs,
# and the kubelet-serving CSR sits unapproved forever.
#
# To break the deadlock the kubelet config (see 064_configure_kubelet.sh.tpl)
# enables serverTLSBootstrap from the first boot on dual-stack clusters, and
# this step approves the resulting CSR here on the node using the local
# super-admin kubeconfig. Single-stack clusters keep the old behaviour and
# do not exercise this code path.

{{- $dualStack := contains "," (.clusterBootstrap.clusterDNSAddress | toString) }}
{{- if and (eq .runType "ClusterBootstrap") $dualStack }}
export KUBECONFIG=/etc/kubernetes/super-admin.conf

bb-log-info "Waiting for kubelet-serving CSR from $(bb-d8-node-name)"

attempts=60
while (( attempts > 0 )); do
  csr_name="$(kubectl get csr \
    -o jsonpath="{range .items[?(@.spec.signerName=='kubernetes.io/kubelet-serving')]}{.metadata.name} {.spec.username} {.status.conditions[0].type}{'\n'}{end}" \
    2>/dev/null \
    | awk -v u="system:node:$(bb-d8-node-name)" '$2 == u && $3 != "Approved" {print $1; exit}')"

  if [[ -n "$csr_name" ]]; then
    bb-log-info "Approving kubelet-serving CSR $csr_name"
    kubectl certificate approve "$csr_name"
    break
  fi

  attempts=$(( attempts - 1 ))
  sleep 2
done

if (( attempts == 0 )); then
  bb-log-warning "No kubelet-serving CSR appeared within the timeout; node-manager hook will retry later"
fi
{{- end }}
