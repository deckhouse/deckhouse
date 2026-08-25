---
title: "Cloud provider — VMware vSphere"
description: "Cloud resource management in Deckhouse Kubernetes Platform using VMware vSphere."
---

The `cloud-provider-vsphere` module integrates Deckhouse Kubernetes Platform with [VMware vSphere](https://www.vmware.com/products/vsphere.html). It allows the [`node-manager`](/modules/node-manager/) module to use vSphere resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-vsphere` module:

- Managing vSphere resources via `cloud-controller-manager`:
  - creates network routes for the `PodNetwork` network on the vSphere side;
  - updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in vSphere.
- Provisioning disks via CSI on datastore. By default, CNS volumes with online resize are used. First-Class Disk (FCD) mode is available as legacy and is configured with the [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag) parameter.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM). Virtual machine parameters are set in the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
