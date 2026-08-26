---
title: "Cloud provider — DVP"
description: "Integration of Deckhouse Kubernetes Platform with the Deckhouse Virtualization Platform."
---

The `cloud-provider-dvp` module integrates Deckhouse Kubernetes Platform with the [Deckhouse Virtualization Platform](https://deckhouse.io/products/virtualization-platform/). It allows the [`node-manager`](/modules/node-manager/) module to use DVP resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-dvp` module:

- Managing DVP resources via `cloud-controller-manager`: updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in DVP.
- Provisioning disks via the DVP CSI driver (`csi.dvp.deckhouse.io`) so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dvp.html#module-interactions) `hashicorp/kubernetes`.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI). Virtual machine parameters are set in the [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="warning" %}
If the cluster was installed with the [DVPClusterConfiguration](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration) schema, migrate to configuration via ModuleConfig.
Until the migration is complete, the `D8CloudProviderDVPMigrationPending` alert may fire and Deckhouse updates may be blocked.

See [How to migrate a cloud provider to ModuleConfig](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig).
{% endalert %}
