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
{{- if or (contains "Static" .nodeGroup.nodeType) (eq .runType "ClusterBootstrap") }}
bb-sync-file /var/lib/bashible/cleanup_static_node.sh - << "EOF"
#!/bin/bash

if [ -z $1 ] || [ "$1" != "--yes-i-am-sane-and-i-understand-what-i-am-doing" ];  then
  >&2 echo "Needed flag isn't passed, exit without any action (--yes-i-am-sane-and-i-understand-what-i-am-doing)"
  exit 1
fi

systemctl disable bashible.service bashible.timer
systemctl stop bashible.service bashible.timer
for pid in $(ps ax | grep "bash /var/lib/bashible/bashible" | grep -v grep | awk '{print $1}'); do
  kill $pid
done

systemctl disable sysctl-tuner.service sysctl-tuner.timer
systemctl disable old-csi-mount-cleaner.service old-csi-mount-cleaner.timer
systemctl disable d8-containerd-cgroup-migration.service
systemctl disable containerd-deckhouse.service
systemctl disable kubelet.service

systemctl stop sysctl-tuner.service sysctl-tuner.timer
systemctl stop old-csi-mount-cleaner.service old-csi-mount-cleaner.timer
systemctl stop d8-containerd-cgroup-migration.service
systemctl stop containerd-deckhouse.service
systemctl stop kubelet.service

# `killall` needs `psmisc` package
# `pkill` needs `procps` on debian-like systems and `procps-ng` on centos-like
# looks like procps(-ng) already installed by default in systems
# killall /opt/deckhouse/bin/containerd-shim-runc-v2
pkill containerd-shim

for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done

rm -rf /etc/systemd/system/bashible.*
rm -rf /etc/systemd/system/sysctl-tuner.*
rm -rf /etc/systemd/system/old-csi-mount-cleaner.*
rm -rf /etc/systemd/system/d8-containerd-cgroup-migration.*
rm -rf /etc/systemd/system/containerd-deckhouse.service /etc/systemd/system/containerd-deckhouse.service.d
rm -rf /etc/systemd/system/kubelet.service /etc/systemd/system/kubelet.service.d

systemctl daemon-reload

rm -rf /var/cache/registrypackages
rm -rf /etc/kubernetes
rm -rf /var/lib/kubelet
rm -rf /var/lib/containerd
rm -rf /etc/cni
rm -rf /var/lib/cni
rm -rf /var/lib/etcd
rm -rf /opt/cni
rm -rf /opt/deckhouse
rm -rf /var/lib/bashible
rm -rf /etc/containerd

# run reboot in the background to normally ends cleanup script and ssh session from client
(sleep 5 && shutdown -r now) &
EOF
{{- end }}
