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

# In case we adopting node bootstrapped by kubeadm
rm -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf     # for ubuntu
rm -f /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf # for centos
rm -f /var/lib/kubelet/kubeadm-flags.env

# Read previously discovered IP
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

cri_config=""

{{- if eq .cri "Containerd" }}
cri_config="--container-runtime=remote --container-runtime-endpoint=unix:/var/run/containerd/containerd.sock"
{{- else if eq .cri "NotManaged" }}
  {{- if .nodeGroup.cri.notManaged.criSocketPath }}
cri_socket_path={{ .nodeGroup.cri.notManaged.criSocketPath | quote }}
  {{- else }}
for socket_path in /var/run/docker.sock /run/containerd/containerd.sock; do
  if [[ -S "${socket_path}" ]]; then
    cri_socket_path="${socket_path}"
    break
  fi
done
  {{- end }}

if [[ -z "${cri_socket_path}" ]]; then
  bb-log-error 'CRI socket is not found, need to manually set "nodeGroup.cri.notManaged.criSocketPath"'
  exit 1
fi

if grep -q "docker" <<< "${cri_socket_path}"; then
  cri_config="--container-runtime=docker --docker-endpoint=unix://${cri_socket_path}"
else
  cri_config="--container-runtime=remote --container-runtime-endpoint=unix:${cri_socket_path}"
fi
{{- end }}

bb-event-on 'bb-sync-file-changed' '_enable_kubelet_service'
function _enable_kubelet_service() {
{{- if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
{{- end }}
  systemctl enable kubelet.service
  bb-flag-set kubelet-need-restart
}

# Generate kubelet unit
bb-sync-file /etc/systemd/system/kubelet.service.d/10-deckhouse.conf - << EOF
[Service]
Type=forking
ExecStart=
ExecStart=/usr/local/bin/d8-kubelet-forker /usr/bin/kubelet \\
{{- if not (eq .nodeGroup.nodeType "Static") }}
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule,node.deckhouse.io/csi-not-bootstrapped=:NoSchedule \\
{{- else }}
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule \\
{{- end }}
    --node-labels=node.deckhouse.io/group={{ .nodeGroup.name }} \\
    --node-labels=node.deckhouse.io/type={{ .nodeGroup.nodeType }} \\
    --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \\
    --config=/var/lib/kubelet/config.yaml \\
{{- if semverCompare "<1.24" .kubernetesVersion }}
    --cni-bin-dir=/opt/cni/bin/ \\
    --cni-conf-dir=/etc/cni/net.d/ \\
    --network-plugin=cni \\
{{- end }}
    --kubeconfig=/etc/kubernetes/kubelet.conf \\
    --address=${discovered_node_ip:-0.0.0.0} \\
{{- /* During the first multi-network Node bootstrap `kubelet` discovers external IP getting it by Node's hostname. */ -}}
{{- /* We have to bootstrap Node with the internal IP because the API certificate denies requests by external IP. */ -}}
{{- if or (eq .nodeGroup.nodeType "Static") (eq .runType "ClusterBootstrap") -}}
$([ -n "$discovered_node_ip" ] && echo -e "\n    --node-ip=${discovered_node_ip} \\")
{{- end }}
{{- if not (eq .nodeGroup.nodeType "Static") }}
    --cloud-provider=external \\
{{- end }}
    --pod-manifest-path=/etc/kubernetes/manifests \\
{{- if hasKey .nodeGroup "kubelet" }}
    --root-dir={{ .nodeGroup.kubelet.rootDir | default "/var/lib/kubelet" }} \\
{{- end }}
    ${cri_config} \\
    --v=2
EOF
