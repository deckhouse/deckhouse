---
title: Integration with zVirt services
permalink: en/admin/integrations/virtualization/zvirt/services.html
---

Deckhouse Platform integrates with the zVirt infrastructure and creates virtual machines for cluster nodes based on the definitions in the [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) resource.

## Key features

The integration with zVirt provides the following features:

- Creating and deleting virtual machines in zVirt when node groups (NodeGroup) are created and scaled.
- Cloning virtual machines from a prepared template based on a cloud image with `cloud-init`.
- Creating the virtual machine network interface from a vNIC profile automatically.
- Getting node network settings over DHCP or the [static network configuration](layout.html#static-network-configuration-of-nodes) for the nodes of CloudPermanent groups.
- Provisioning volumes via the CSI driver and [StorageClasses for zVirt storage domains](storage.html#storageclasses-for-storage-domains).
- Removing from the cluster the nodes whose virtual machines no longer exist in zVirt.
