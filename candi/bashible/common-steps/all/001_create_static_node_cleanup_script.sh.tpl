# Copyright 2023 Flant JSC
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
{{- if or (eq .nodeGroup.nodeType "Static") (eq .runType "ClusterBootstrap") }}
bb-sync-file /var/lib/bashible/cleanup_static_node.sh - << "EOF"
#!/bin/bash
if [ "$1" -ne "--yes-i-am-sane-and-i-understand-what-i-am-doing" ]; then
  >&2 echo "Needed flag isn't passed, exit without any action"
  exit 1
fi

systemctl stop bashible.service bashible.timer
systemctl stop sysctl.service sysctl.timer
systemctl stop old-csi-mount-cleaner.service old-csi-mount-cleaner.timer
systemctl stop d8-containerd-cgroup-migration.service
systemctl stop containerd-deckhouse.service
systemctl stop kubelet.service
systemctl daemon-reload

killall /opt/deckhouse/bin/containerd-shim-runc-v2

rm -rf /etc/systemd/system/bashible.*
rm -rf /etc/systemd/system/sysctl-tuner.*
rm -rf /etc/systemd/system/old-csi-mount-cleaner.*
rm -rf /etc/systemd/system/d8-containerd-cgroup-migration.*
rm -rf /etc/systemd/system/containerd-deckhouse.service /etc/systemd/system/containerd-deckhouse.service.d
rm -rf /etc/systemd/system/kubeler.service /etc/systemd/system/kubelet.service.d

rm -rf /var/lib/bashible
rm -rf /var/cache/registrypackages
rm -rf /etc/kubernetes
rm -rf /var/lib/kubelet
rm -rf /var/lib/containerd
rm -rf /etc/cni
rm -rf /var/lib/cni
rm -rf /var/lib/etcd
rm -rf /opt/cni
rm -rf /opt/deckhouse/bin
reboot
EOF
{{- end }}
