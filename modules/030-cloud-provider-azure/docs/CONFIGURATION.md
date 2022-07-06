---
title: "Cloud provider — Azure: configuration"
---

The module is configured automatically based on the chosen placement strategy (the `AzureClusterConfiguration` custom resource). In most cases, you do not need to configure the module manually.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the Azure cloud provider, the instance class is the [`AzureInstanceClass`](cr.html#azureinstanceclass) custom resource that stores specific parameters of the machines.

## Storage

The module automatically creates the following StorageClasses:

| Name | Disk type |
|---|---|
|managed-standard-ssd|[StandardSSD_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-ssd)|
|managed-standard|[Standard_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-hdd)|
|managed-premium|[Premium_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#premium-ssd)|

It allows you to configure additional StorageClasses for volumes with configurable IOPS and Throughput. Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

StorageClass Configuration Parameters:

* `provision` — sets additional StorageClasses for [Azure ultra disks](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disk):
  * `name` — the name of the class to create;
  * `type` — Azure disk storage account type. Available values are `Standard_LRS`, `Premium_LRS`, `StandardSSD_LRS`, `UltraSSD_LRS`, `Premium_ZRS`, `StandardSSD_ZRS`. Check out [Azure docs](https://docs.microsoft.com/en-us/azure/storage/common/storage-account-overview#types-of-storage-accounts) for more information.
  * `cachingMode` — string value that corresponds to destired caching mode;
      Can be `None`, `ReadOnly`, `ReadWrite`. If expected disk size is more than 4 TiB, you have to set `cachineMode: None`.
      Check out [Azure docs](https://docs.microsoft.com/en-us/azure/virtual-machines/premium-storage-performance#disk-caching) for more information.
  * `diskIOPSReadWrite` — disk IOPS (limit of 300 IOPS/GiB, up to a maximum of 160 K IOPS per disk);
  * `diskMBpsReadWrite` — disk throughput, `MBps`, (limit of a single disk is 256 KiB/s for each provisioned IOPS).
* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
* `default` — the name of StorageClass that will be used by default in the cluster:
  * If the parameter is omitted, the default StorageClass is `managed-standard-ssd`.

An example of Storage Class configuration:

```yaml
cloudProviderAzure: |
  storageClass:
    provision:
    - name: managed-ultra-ssd
      diskIOPSReadWrite: 600
      diskMBpsReadWrite: 150
    exclude:
    - managed-standard.*
    - managed-premium
    default: managed-ultra-ssd
```
