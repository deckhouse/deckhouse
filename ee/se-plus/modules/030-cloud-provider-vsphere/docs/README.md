---
title: "Cloud provider — VMware vSphere"
description: "Managing cloud resources in Deckhouse Kubernetes Platform based on VMware vSphere."
---

The `cloud-provider-vsphere` module is responsible for interacting with the [VMware vSphere-based](https://www.vmware.com/products/vsphere.html) cloud resources. It allows the [node manager](../../modules/node-manager/) module to use vSphere resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-vsphere` module:
- Manages vSphere resources using the `cloud-controller-manager` (CCM) module:
  * The CCM module creates network routes for the `PodNetwork` network on the vSphere side.
  * The CCM module updates the metadata of the vSphere VirtualMachines and Kubernetes Nodes and deletes nodes that are no longer in vSphere.
- Provisions disks on datastore in vSphere via the First-Class Disk mechanism using the `CSI storage` component.
- Registers with the [node-manager](../../modules/node-manager/) module so that [VsphereInstanceClasses](cr.html#vsphereinstanceclass) can be used when creating the [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
