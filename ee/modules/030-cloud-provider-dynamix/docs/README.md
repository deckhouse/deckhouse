---
title: "Cloud provider — Basis Dynamix"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Basis Dynamix."
---

The `cloud-provider-dynamix` module integrates Deckhouse Kubernetes Platform with the [Basis Dynamix](https://basistech.ru/products/basis-dynamix/) platform. It allows the [`node-manager`](/modules/node-manager/) module to use Dynamix resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-dynamix` module:

- Managing Dynamix resources via `cloud-controller-manager`:
  - updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in Dynamix;
  - creates load balancers for Services of the LoadBalancer type. The Service must include annotations with the names of the internal and external networks.
- Provisioning disks via the Dynamix CSI driver (`dynamix.deckhouse.io`) so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudPermanent nodes using the [Terraform/OpenTofu provider](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dynamix.html#module-interactions) `terraform-provider-decort/decort`.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI). Virtual machine parameters are set in the [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="info" %}
The module is in the Preview stage.
{% endalert %}
