---
title: "Cloud provider — Azure: настройки"
---

Модуль настраивается автоматически, исходя из выбранной схемы размещения (custom resource [AzureClusterConfiguration](cluster_configuration.html#azureclusterconfiguration)). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр [cloudInstances.ClassReference](../040-node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference)). Инстанс-класс для cloud провайдера Azure — это custom resource [`AzureInstanceClass`](cr.html#azureinstanceclass), в котором указываются конкретные параметры самих машин.

Модуль автоматически создаёт следующие StorageClass'ы:

| Имя | Тип диска |
|---|---|
|managed-standard-ssd|[StandardSSD_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-ssd)|
|managed-standard|[Standard_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-hdd)|
|managed-premium|[Premium_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#premium-ssd)|

Также он позволяет сконфигурировать дополнительные StorageClass'ы для дисков с настраиваемыми IOPS и Throughput и отфильтровать ненужные StorageClass'ы, указав их с помощью параметра [exclude](#parameters-storageclass-exclude).

Пример конфигурации StorageClass:

```yaml
cloudProviderAzure: |
  storageClass:
    provision:
    - name: managed-ultra-ssd
      type: UltraSSD_LRS
      cachingMode: None
      diskIOPSReadWrite: 600
      diskMBpsReadWrite: 150
    exclude:
    - managed-standard.*
    - managed-premium
    default: managed-ultra-ssd
```

{% include module-settings.liquid %}
