---
title: "Cloud provider — OpenStack"
description: "Cloud resource management in Deckhouse Kubernetes Platform using OpenStack."
---

The `cloud-provider-openstack` module is responsible for interacting with the [OpenStack-based](https://www.openstack.org/) cloud resources. It allows the [node manager](/node-manager/) module to use OpenStack resources for provisioning nodes for the specified [node group](/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-openstack` module:

- Manages OpenStack resources using the `cloud-controller-manager` (CCM) module:
  - The CCM module updates the metadata of the  OpenStack Servers and Kubernetes Nodes and deletes nodes that no longer exist in OpenStack.
- Provisions disks in Cinder (block) OpenStack using the `CSI storage` component; Manilla (shared filesystem service) is not supported yet. The Cinder CSI driver supports OpenStack re-authentication with service catalog refresh, improving the reliability of volume operations in pods running for a long time without restart.
- Registers with the [node-manager](/node-manager/) module so that [OpenStackInstanceClasses](cr.html#openstackinstanceclass) can be used when creating the [NodeGroup](/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
This module is transitioning CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing NodeGroups continue to use MCM, while newly created NodeGroups use CAPI by default. To migrate an existing NodeGroup to CAPI, recreate it.

For details, see [How to migrate node groups to Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi) section.
{% endalert %}
