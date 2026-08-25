---
title: "Cloud provider — zVirt"
description: "Cloud resource management in Deckhouse Kubernetes Platform using zVirt."
---

The `cloud-provider-zvirt` module integrates Deckhouse Kubernetes Platform with [zVirt](https://www.zvirt.ru/). It allows the [node-manager](/modules/node-manager/) module to use zVirt resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-zvirt` module:

- Managing zVirt resources via `cloud-controller-manager`: updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in zVirt.
- Provisioning disks via the zVirt CSI driver (`csi.ovirt.org`) so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI). Virtual machine parameters are set in the [ZvirtInstanceClass](cr.html#zvirtinstanceclass) resource.
- Registering with [node-manager](/modules/node-manager/) so that `ZvirtInstanceClass` can be used when describing a `NodeGroup`.
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used in `DirectWithNodeRoutes` mode.
