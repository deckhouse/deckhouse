---
title: Integration with Huawei Cloud
permalink: en/admin/integrations/private/huaweicloud/overview.html
---

Deckhouse Platform integrates with [Huawei Cloud](https://www.huaweicloud.com/) and provisions cluster nodes in it. Virtual machine parameters are set in the [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing Huawei Cloud resources:
  - updates instance and cluster node metadata and removes from cluster nodes that no longer exist in the cloud;
  - creates load balancers (ELB) for Services of the LoadBalancer type.
- Provisioning disks via the EVS CSI driver (`evs.csi.huaweicloud.com`) so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-huaweicloud.html#module-interactions) `terraform-provider-huaweicloud`.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
