---
title: "Cloud provider — OpenStack"
description: "Cloud resource management in Deckhouse Kubernetes Platform using OpenStack."
---

The `cloud-provider-openstack` module is responsible for interacting with the [OpenStack-based](https://www.openstack.org/) cloud resources. It allows the [node manager](../../modules/node-manager/) module to use OpenStack resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-openstack` module:
- Manages OpenStack resources using the `cloud-controller-manager` (CCM) module:
  * The CCM module updates the metadata of the  OpenStack Servers and Kubernetes Nodes and deletes nodes that no longer exist in OpenStack.
- Provisions disks in Cinder (block) OpenStack using the `CSI storage` component; Manilla (shared filesystem service) is not supported yet.
- Registers with the [node-manager](../../modules/node-manager/) module so that [OpenStackInstanceClasses](cr.html#openstackinstanceclass) can be used when creating the [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
