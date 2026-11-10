---
title: Storage and load balancing in zVirt
permalink: en/admin/integrations/virtualization/zvirt/storage.html
---

## Storage

The [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) module provisions zVirt disks for cluster volumes via the `csi.ovirt.org` CSI driver. Volumes are requested with standard PersistentVolumeClaim resources using the StorageClasses the module creates.

### StorageClasses for storage domains

For each active zVirt storage domain, the module creates a StorageClass whose name is derived from the storage domain name. Letters in the name are converted to lowercase, spaces are replaced with hyphens, and the other characters except Latin letters, digits, hyphens, and dots are removed. Hyphens and dots at the beginning and end of the name are also removed.

The created StorageClasses have the following properties:

- The disk for a volume is created in the storage domain the StorageClass corresponds to.
- Space for the disk is allocated in full at once (thin provisioning is not used).
- A volume is created when the workload that uses it is scheduled to a node (`volumeBindingMode: WaitForFirstConsumer`).
- The volume size can be increased (`allowVolumeExpansion: true`).

If the global [`defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) parameter is not set and the cluster has no other default StorageClass, the alphabetically first StorageClass of the module becomes the default StorageClass.

To prevent the module from creating StorageClasses for some storage domains, list the names of these StorageClasses in the [`excludedStorageClasses`](/modules/cloud-provider-zvirt/configuration.html#parameters-storage-parameters-excludedstorageclasses) parameter of the ModuleConfig resource. Each item of the list is a regular expression matched against the whole StorageClass name.

Example of excluding StorageClasses:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  settings:
    storage:
      parameters:
        excludedStorageClasses:
          - hostedstorage
          - backup-.*
```

In the example, the module does not create the `hostedstorage` StorageClass and the StorageClasses whose names start with `backup-`.

To disable disk provisioning in the cluster, set the [`storage.disabled`](/modules/cloud-provider-zvirt/configuration.html#parameters-storage-disabled) parameter to `true`. In this case, the module does not deploy the CSI driver and does not create StorageClasses.

### Virtual machine disks

The root disk of a virtual machine is created when the template is cloned, and then it is resized to the value of the [`rootDiskSizeGb`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-rootdisksizegb) parameter of the ZvirtInstanceClass resource.

On master nodes, a separate disk is created for etcd. Its size is set with the [`etcdDiskSizeGb`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-etcddisksizegb) parameter, and the storage domain is set with the [`storageDomainID`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-storagedomainid) parameter of the ZvirtInstanceClass resource of the `master` group. The storage domain must be available in the zVirt cluster set with the [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid) parameter.

## Load balancing

The `cloud-provider-zvirt` module does not create load balancers for Service objects of the `LoadBalancer` type. To accept incoming traffic, use one of the following options:

- An external load balancer, for example, a hardware or software one, that forwards traffic to ports `80` and `443` of the cluster [frontend nodes](../../../../glossary.html#frontend-node).
- The [`metallb`](/modules/metallb/) module in L2 mode that announces load balancer IP addresses in the node network.

For L2 mode, allocate an IP address range in the network the frontend nodes are connected to via the vNIC profile from the `vnicProfileID` parameter. Addresses from this range must not be assigned by the network DHCP server. A configuration example is described in the ["Example of metallb usage in L2 LoadBalancer mode"](/modules/metallb/examples.html#example-of-metallb-usage-in-l2-loadbalancer-mode) section.

{% alert level="info" %}
Operation of the `metallb` module in BGP mode in zVirt is not guaranteed and depends on the network infrastructure.
{% endalert %}
