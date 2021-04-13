---
title: "Сloud provider — Azure: настройки"
---

Модуль настраивается автоматически исходя из выбранной схемы размещения (custom resource `AzureClusterConfiguration`). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера Azure — это custom resource [`AzureInstanceClass`](cr.html#azureinstanceclass), в котором указываются конкретные параметры самих машин.

## Storage

Модуль автоматически создаёт следующие StorageClasses:

| Имя | Тип диска |
|---|---|
|managed-standard-ssd|[StandardSSD_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-ssd)|
|managed-standard|[Standard_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-hdd)|
|managed-premium|[Premium_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#premium-ssd)|

Позволяет сконфигурировать дополнительные StorageClasses для дисков с настраиваемыми IOPS и Throughput. А также отфильтровать ненужные StorageClass, указанием их в параметре `exclude`.

* `provision` — дополнительные StorageClasses для [Azure ultra disks](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disk).
  * Формат — массив объектов.
    * `name` — имя будущего класса.
    * `diskIOPSReadWrite` — количество IOPS (лимит 300 IOPS/GiB, и максимум 160 K IOPS на диск).
    * `diskMBpsReadWrite` — скорость обращения к диску, `MBps` (лимит 256 KiB/s на каждый IOPS).
  * Опциональный параметр.
* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
  * Формат — массив строк.
  * Опциональный параметр.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию.
  * Формат — строка.
  * Опциональный параметр.
  * Если параметр не задан, фактическим StorageClass по умолчанию будет `managed-standard-ssd`.

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
