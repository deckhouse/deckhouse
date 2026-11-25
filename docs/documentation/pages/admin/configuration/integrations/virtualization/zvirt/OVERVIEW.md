---
title: Integration with zVirt
permalink: en/admin/integrations/virtualization/zvirt/overview.html
---

Deckhouse Platform integrates with [zVirt](https://www.zvirt.ru/) and provisions cluster nodes in it. Virtual machine parameters are set in the [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing zVirt resources, including updating virtual machine and cluster node metadata and removing from cluster nodes that no longer exist in zVirt.
- Provisioning disks via the zVirt CSI driver (`csi.ovirt.org`) so that PersistentVolumes can be requested from the cluster using [StorageClasses for zVirt storage domains](load-balancing-and-storage.html#storageclasses-for-storage-domains).
- Provisioning CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-zvirt.html#module-interactions) `terraform-provider-ovirt`.
- Provisioning CloudEphemeral nodes via Cluster API (CAPI).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
- Cloning virtual machines from a prepared template based on a cloud image with `cloud-init`.
- Creating the virtual machine network interface from a vNIC profile automatically.
- Getting node network settings over DHCP or the [static network configuration](layout.html#static-network-configuration-of-nodes) for the nodes of CloudPermanent groups.
