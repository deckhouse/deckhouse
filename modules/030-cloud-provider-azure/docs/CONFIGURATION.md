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

<!-- SCHEMA -->

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
