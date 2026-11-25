---
title: Integration with built-in virtualization
permalink: en/admin/integrations/virtualization/dvp/overview.html
---

Deckhouse Platform integrates with [built-in virtualization](../../../configuration/virtualization/) and provisions cluster nodes in it. Virtual machine parameters are set in the [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing built-in virtualization resources, including updating virtual machine and cluster node metadata and removing from cluster nodes that no longer exist in built-in virtualization.
- Provisioning disks via the built-in virtualization CSI driver (`csi.dvp.deckhouse.io`) so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dvp.html#module-interactions) `terraform-provider-kubernetes`.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="info" %}
The virtualization integration is enabled automatically for all [cloud clusters](../../../../glossary.html#cloud-cluster) deployed in built-in virtualization.
No additional configuration is required.
{% endalert %}
