---
title: Integration with Yandex Cloud
permalink: en/admin/integrations/public/yandex/overview.html
---

Deckhouse Platform integrates with [Yandex Cloud](https://cloud.yandex.com/) and provisions cluster nodes in it. Virtual machine parameters are set in the [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing Yandex Cloud resources:
  - creates network routes for the `PodNetwork` network on the Yandex Cloud side;
  - creates Network Load Balancers and target groups for Services of the LoadBalancer type;
  - updates instance and cluster node metadata and removes from cluster nodes that no longer exist in Yandex Cloud.
- Provisioning disks via the Yandex CSI driver (`yandex.csi.flant.com`) and creating StorageClasses for Yandex Cloud disk types so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-yandex.html#module-interactions) `terraform-provider-yandex`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM) or Cluster API (CAPI).
- Enabling CNI in new clusters automatically. Starting with Deckhouse Platform 1.76, [`cni-cilium`](/modules/cni-cilium/) is used by default.
