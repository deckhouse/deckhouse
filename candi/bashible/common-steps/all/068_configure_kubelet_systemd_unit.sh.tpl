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
discovered_node_ip="$(bb-d8-node-ip)"

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
cri_socket_path="/run/containerd/containerd.sock"
{{- end }}

{{- if eq .cri "NotManaged" }}
  {{- if (((.nodeGroup.cri).notManaged).criSocketPath) }}
cri_socket_path={{ .nodeGroup.cri.notManaged.criSocketPath | quote }}
  {{- else }}
  if [[ -S "/run/containerd/containerd.sock" ]]; then
    cri_socket_path="/run/containerd/containerd.sock"
    break
  fi
  {{- end }}

if [[ -z "${cri_socket_path}" ]]; then
  bb-log-error 'CRI socket is not found, need to manually set "nodeGroup.cri.notManaged.criSocketPath"'
  exit 1
fi
{{- end }}

cri_config="--container-runtime-endpoint=unix://${cri_socket_path}"

credential_provider_flags=""

if bb-flag? kubelet-enable-credential-provider; then
  credential_provider_flags="--image-credential-provider-config=/var/lib/bashible/kubelet-credential-provider-config.yaml --image-credential-provider-bin-dir=/opt/deckhouse/bin"
  bb-flag-unset kubelet-enable-credential-provider
fi

bb-event-on 'bb-sync-file-changed' '_enable_kubelet_service'
function _enable_kubelet_service() {
  systemctl daemon-reload
  systemctl enable kubelet.service
  bb-flag-set kubelet-need-restart
}

# Generate kubelet unit
bb-sync-file /etc/systemd/system/kubelet.service.d/10-deckhouse.conf - << EOF
[Service]
Type=forking
Environment="PATH=/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin"
ExecStart=
ExecStart=/opt/deckhouse/bin/d8-kubelet-forker /opt/deckhouse/bin/kubelet \\
{{- if not (eq .nodeGroup.nodeType "Static") }}
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule,node.deckhouse.io/csi-not-bootstrapped=:NoSchedule \\
{{- else }}
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule \\
{{- end }}
    --node-labels=node.deckhouse.io/group={{ .nodeGroup.name }} \\
    --node-labels=node.deckhouse.io/type={{ .nodeGroup.nodeType }} \\
    --node-labels=node.deckhouse.io/dvp-nesting-level=$(bb-dvp-nesting-level) \\
    --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \\
    --config=/var/lib/kubelet/config.yaml \\
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
    ${credential_provider_flags} \\
    --hostname-override=$(bb-d8-node-name) \\
    --v=2
EOF

# CIS becnhmark purposes
chmod 600 /etc/systemd/system/kubelet.service.d/10-deckhouse.conf
chmod 600 /lib/systemd/system/kubelet.service
