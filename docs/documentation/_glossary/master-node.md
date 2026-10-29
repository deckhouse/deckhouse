---
title: Master node
description: A node that runs the control plane components. Master nodes carry the label `node-role.kubernetes.io/control-plane`, and the [`control-plane-manager`](/modules/control-plane-manager/) module runs on them.
lang: en
---

Deckhouse Platform manages the [control plane](glossary.html#control-plane) on master nodes: certificates, component configuration, versions, the etcd cluster and kubeconfig files. In a cloud cluster, master nodes are [CloudPermanent nodes](glossary.html#cloud-permanent-node).

The number of master nodes defines whether the cluster is [single-master](glossary.html#single-master) or [multi-master](glossary.html#multi-master). With more than one master node, [HA mode](glossary.html#ha-mode) is enabled automatically.

For details, see [General management and configuration of the control plane](admin/configuration/platform-scaling/control-plane/control-plane-management-and-configuration.html).
