#!/usr/bin/env bash

# TODO: remove /opt/deckhouse and disable kubelet

# Stop all the services and running containers:
systemctl stop bashible.service bashible.timer
systemctl stop kubelet.service
systemctl stop containerd-deckhouse.service
for i in $(ps ax | grep containerd-shim | grep -v grep | awk '{print $1}'); do kill $i; done

# Unmount all mounted partitions:
for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i; done

# Delete all directories and files:
rm -rf /var/cache/registrypackages
rm -rf /etc/kubernetes
rm -rf /var/lib/kubelet
rm -rf /var/lib/containerd
rm -rf /etc/cni
rm -rf /var/lib/cni
rm -rf /var/lib/etcd
rm -rf /etc/systemd/system/bashible*
rm -rf /etc/systemd/system/sysctl-tuner*
rm -rf /etc/systemd/system/kubelet*

# Delete cilium interface:
ip link show cilium_host up &>/dev/null && ip link set cilium_host down && ip link delete cilium_host

# Cleanup systemd:
systemctl daemon-reload
systemctl reset-failed

rm -rf /var/lib/bashible

reboot
