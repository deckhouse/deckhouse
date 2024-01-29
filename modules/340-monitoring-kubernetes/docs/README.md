---
title: "The monitoring-kubernetes module"
---

The module is intended for the basic monitoring of cluster nodes.

It safely collects metrics and provides a basic set of rules for monitoring of:
- The current container runtime version (docker, containerd) on the node and if it complies with the requirements.
- The overall health of the cluster monitoring subsystem (Dead man's switch).
- The availability of file descriptors, sockets, abundance of free space and inodes.
- The operation of `kube-state-metrics`, `node-exporter`, `kube-dns`.
- The state of cluster nodes (NotReady, drain, cordon).
- The state of time synchronization between nodes.
- The cases of the prolonged CPU stealing.
- The state of the Conntrack table on nodes.
- The Pods that report an incorrect state (due to kubelet-related or other issues), etc.

To collect metrics about Linux Memory Overcommitment, Linux kernel version >= `5.8.0` is required.
