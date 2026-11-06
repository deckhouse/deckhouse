---
title: Control plane
description: The Kubernetes components that manage the cluster, such as kube-apiserver and etcd. In Deckhouse Platform they run on master nodes, and in HA mode with two master nodes the etcd quorum is provided by an arbiter node.
lang: en
---

Deckhouse Platform manages the certificates of the control plane, generates the configuration and manifests of its components, upgrades and downgrades their versions, manages the etcd cluster and its members, runs etcd defragmentation and maintains the kubeconfig files. The [`control-plane-manager`](/modules/control-plane-manager/) module runs on all [master nodes](glossary.html#master-node).

The control plane works in the [single-master](glossary.html#single-master) or [multi-master](glossary.html#multi-master) mode. In [HA mode](glossary.html#ha-mode) with two master nodes, the third etcd member runs on an [arbiter node](glossary.html#arbiter-node): it holds only etcd, without the other control plane components, so that the etcd cluster keeps its quorum.

For details, see [General management and configuration of the control plane](admin/configuration/platform-scaling/control-plane/control-plane-management-and-configuration.html).
