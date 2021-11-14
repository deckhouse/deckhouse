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

# Миграция!!!!! Удалить после выката
rm -f /etc/systemd/system/kubelet.service.d/cim.conf
rm -rf /var/lib/kubelet/manifests

# In case we adopting node bootstrapped by kubeadm
rm -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf     # for ubuntu
rm -f /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf # for centos
rm -f /var/lib/kubelet/kubeadm-flags.env

# Read previously discovered IP
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

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
ExecStart=
ExecStart=/usr/local/bin/d8-kubelet-forker /usr/bin/kubelet \\
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule,node.deckhouse.io/csi-not-bootstrapped=:NoSchedule \\
    --node-labels=node.deckhouse.io/group=worker \\
    --node-labels=node.deckhouse.io/type=CloudEphemeral \\
    --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \\
    --config=/var/lib/kubelet/config.yaml \\
    --cni-bin-dir=/opt/cni/bin/ \\
    --cni-conf-dir=/etc/cni/net.d/ \\
    --kubeconfig=/etc/kubernetes/kubelet.conf \\
    --network-plugin=cni \\
    --address=${discovered_node_ip:-0.0.0.0} \\
    --cloud-provider=external \\
    --pod-manifest-path=/etc/kubernetes/manifests \\
    --container-runtime=remote \\
    --container-runtime-endpoint=unix:/var/run/containerd/containerd.sock \\
    --v=2
EOF
