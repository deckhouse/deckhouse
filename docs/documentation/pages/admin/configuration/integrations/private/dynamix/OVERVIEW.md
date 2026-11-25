---
title: Integration with Basis Dynamix
permalink: en/admin/integrations/private/dynamix/overview.html
lang: en
---

Deckhouse Platform integrates with the [Basis Dynamix](https://basistech.ru/products/basis-dynamix/) platform and provisions cluster nodes in it. Virtual machine parameters are set in the [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing Dynamix resources:
  - updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in Dynamix;
  - creates load balancers for Services of the LoadBalancer type.
- Provisioning disks via the Dynamix CSI driver (`dynamix.deckhouse.io`) so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dynamix.html#module-interactions) `terraform-provider-decort`.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="info" %}
The module is in the Preview stage.
{% endalert %}
