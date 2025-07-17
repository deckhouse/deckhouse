---
title: "Cloud provider — Azure"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Microsoft Azure."
---

The `cloud-provider-azure` module is responsible for interacting with the [Azure](https://portal.azure.com/) cloud resources. It allows the [node manager](../../modules/node-manager/) module to use Azure resources for provisioning nodes for the defined [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

Features of the `cloud-provider-azure` module:

- Managing Azure resources using the `cloud-controller-manager` (CCM) module:
  - The CCM module creates network routes for the `PodNetwork` network on the Azure side;
  - The CCM module creates LoadBalancers for Kubernetes Service objects that have the `LoadBalancer` type;
  - The CCM module updates the metadata of the cluster nodes according to the configuration parameters and deletes nodes that are no longer in Azure;
- Provisioning nodes in Azure using the `CSI storage` component;
- Enabling the necessary CNI plugin (using the [simple bridge](../../modules/cni-simple-bridge/));
- Registering with the [node-manager](../../modules/node-manager/) module so that [AzureInstanceClasses](cr.html#azureinstanceclass) can be used when creating the [NodeGroup](../../modules/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
When using load balancers, outgoing traffic also goes through it. If no balancer has UDP rules, all outgoing UDP traffic is blocked. As a result, such utilities as `ntpdate` and `chrony` do not work. To solve the problem, you need to add a load balancing rule with any UDP port to an existing balancer yourself, or create a service in the cluster with the LoadBalancer type with any UDP port.
{% endalert %}
