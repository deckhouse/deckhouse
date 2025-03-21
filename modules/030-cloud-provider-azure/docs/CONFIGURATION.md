---
title: "Cloud provider — Azure: configuration"
---

The module is configured automatically based on the chosen placement strategy (the [AzureClusterConfiguration](cluster_configuration.html#azureclusterconfiguration) custom resource). In most cases, you do not need to configure the module manually.

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the [cloudInstances.ClassReference](../node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) parameter). In the case of the Azure cloud provider, the instance class is the [`AzureInstanceClass`](cr.html#azureinstanceclass) custom resource that stores specific parameters of the machines.

<div markdown="0" style="height: 0;" id="storage"></div>
The module automatically creates the following StorageClasses:

| Name | Disk type |
|---|---|
|managed-standard-ssd|[StandardSSD_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-ssd)|
|managed-standard|[Standard_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-hdd)|
|managed-premium|[Premium_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#premium-ssd)|

It allows you to configure additional StorageClasses for volumes with configurable IOPS and Throughput. Also, you can filter out the unnecessary StorageClasses  via the [exclude](#parameters-storageclass-exclude) parameter.

An example of Storage Class configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-azure
spec:
  version: 1
  enabled: true
  settings:
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

{% include module-settings.liquid %}
