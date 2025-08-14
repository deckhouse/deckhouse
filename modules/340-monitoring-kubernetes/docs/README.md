---
title: "The monitoring-kubernetes module"
description: "Basic monitoring of cluster nodes in Deckhouse Kubernetes Platform."
---

The `monitoring-kubernetes` module provides transparent and timely monitoring of the status of all cluster nodes and key infrastructure components.

Module features:

- provides an opportunity to plan infrastructure resources (Capacity planning);
- monitors the container runtime version (docker, containerd) on each node and checks it for compliance with the allowed versions;
- monitors the performance of the cluster monitoring subsystem itself (Dead man's switch);
- gets metrics about the availability of file descriptors, sockets, free space, and inodes on each node;
- monitors the correct operation of key monitoring components: kube-state-metrics, node-exporter, kube-dns;
- checks the status of all nodes (`NotReady`, `drain`, `cordon`) and promptly reports problems;
- monitors time synchronization and notifies about deviations;
- detects cases of prolonged CPU steal overrun (when the node does not receive the required CPU time);
- controls the status of the Conntrack table on the nodes;
- shows pods with incorrect statuses, for example, if kubelet failed to do its job;
- allows you to export metrics to external monitoring systems for a single point of control.
