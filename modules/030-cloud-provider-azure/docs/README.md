---
title: "Cloud provider — Azure"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Microsoft Azure."
---

The `cloud-provider-azure` module integrates Deckhouse Kubernetes Platform with [Microsoft Azure](https://portal.azure.com/). It allows the [node-manager](/modules/node-manager/) module to use Azure resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-azure` module:

- Managing Azure resources via `cloud-controller-manager`:
  - creates network routes for the `PodNetwork` network on the Azure side;
  - creates load balancers for Services of the `LoadBalancer` type;
  - updates cluster node metadata and removes from Kubernetes nodes that no longer exist in Azure.
- Provisioning disks via the Azure Disk CSI driver (`disk.csi.azure.com`) and creating StorageClasses for Azure disk types so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM). Virtual machine parameters are set in the [AzureInstanceClass](cr.html#azureinstanceclass) resource.
- Registering with [node-manager](/modules/node-manager/) so that `AzureInstanceClass` can be used when describing a `NodeGroup`.
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="warning" %}
For utilities such as `ntpdate` and `chrony` to work correctly, make sure the load balancer has rules for UDP traffic. If outbound UDP is blocked, add a rule to the existing load balancer or create a Service of the `LoadBalancer` type with a UDP port.
{% endalert %}
