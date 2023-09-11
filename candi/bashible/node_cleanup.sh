#!/bin/bash

function node_cleanup() {
  systemctl stop kubernetes-api-proxy.service
  systemctl stop kubernetes-api-proxy-configurator.service
  systemctl stop kubernetes-api-proxy-configurator.timer

  systemctl stop bashible.service bashible.timer
  systemctl stop kubelet.service
  systemctl stop containerd

  for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done

  rm -rf /var/lib/bashible
  rm -rf /var/cache/registrypackages
  rm -rf /etc/kubernetes
  rm -rf /var/lib/kubelet
  rm -rf /var/lib/docker
  rm -rf /var/lib/containerd
  rm -rf /etc/cni
  rm -rf /var/lib/cni
  rm -rf /var/lib/etcd
  rm -rf /etc/systemd/system/kubernetes-api-proxy*
  rm -rf /etc/systemd/system/bashible*
  rm -rf /etc/systemd/system/sysctl-tuner*
  rm -rf /etc/systemd/system/kubelet*
}
