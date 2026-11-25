---
title: Integration with VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/overview.html
---

Deckhouse Platform integrates with [VMware vSphere](https://www.vmware.com/products/vsphere.html) and provisions cluster nodes in it. Virtual machine parameters are set in the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing vSphere resources:
  - creates network routes for the `PodNetwork` network on the vSphere side;
  - updates virtual machine and cluster node metadata and removes from cluster nodes that no longer exist in vSphere.
- Provisioning disks via CSI on datastore. By default, CNS volumes with online resize are used. First-Class Disk (FCD) mode is available as legacy and is configured with the [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag) parameter.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vsphere.html#module-interactions) `terraform-provider-vsphere`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM) or Cluster API (CAPI).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="info" %}
Deckhouse Platform supports hybrid integration with VMware vSphere. For configuration details, see [Hybrid cluster with vSphere](../../hybrid/vsphere-hybrid.html) section.
{% endalert %}
