---
title: "Cloud provider — Yandex Cloud"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Yandex Cloud."
---

The `cloud-provider-yandex` module integrates Deckhouse Kubernetes Platform with [Yandex Cloud](https://cloud.yandex.com/). It allows the [`node-manager`](/modules/node-manager/) module to use Yandex Cloud resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-yandex` module:

- Managing Yandex Cloud resources via `cloud-controller-manager`:
  - creates network routes for the `PodNetwork` network on the Yandex Cloud side;
  - creates Network Load Balancers and target groups for Services of the LoadBalancer type;
  - updates instance and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in Yandex Cloud.
- Provisioning disks via the Yandex CSI driver (`yandex.csi.flant.com`) and creating StorageClasses for Yandex Cloud disk types so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM) or Cluster API (CAPI). Virtual machine parameters are set in the [YandexInstanceClass](cr.html#yandexinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that YandexInstanceClass can be used when describing a NodeGroup.
- Enabling CNI for new clusters automatically. Starting with DKP 1.76, [`cni-cilium`](/modules/cni-cilium/) in `VXLAN` mode with source IP translation via [BPF](/products/kubernetes-platform/documentation/v1/admin/configuration/network/other/bpflb.html) is used by default.

{% alert level="warning" %}
The module is migrating CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing NodeGroups continue to use MCM, while new ones are created with CAPI by default. For migrating existing groups, see [How to migrate node groups to Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi).
{% endalert %}
