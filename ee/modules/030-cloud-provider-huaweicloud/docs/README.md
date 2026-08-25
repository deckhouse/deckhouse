---
title: "Cloud provider — Huawei Cloud"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Huawei Cloud."
---

The `cloud-provider-huaweicloud` module integrates Deckhouse Kubernetes Platform with [Huawei Cloud](https://www.huaweicloud.com/). It allows the [`node-manager`](/modules/node-manager/) module to use Huawei Cloud resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-huaweicloud` module:

- Managing Huawei Cloud resources via `cloud-controller-manager`:
  - updates instance and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in the cloud;
  - creates load balancers (ELB) for Services of the LoadBalancer type.
- Provisioning disks via the EVS CSI driver (`evs.csi.huaweicloud.com`) so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI). Virtual machine parameters are set in the [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
