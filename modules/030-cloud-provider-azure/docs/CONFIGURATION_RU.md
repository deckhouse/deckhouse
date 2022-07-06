---
title: "Cloud provider — Azure: настройки"
---

Модуль настраивается автоматически, исходя из выбранной схемы размещения (custom resource `AzureClusterConfiguration`). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference` NodeGroup). Инстанс-класс для cloud провайдера Azure — это custom resource [`AzureInstanceClass`](cr.html#azureinstanceclass), в котором указываются конкретные параметры самих машин.

## Storage

Модуль автоматически создаёт следующие StorageClass'ы:

| Имя | Тип диска |
|---|---|
|managed-standard-ssd|[StandardSSD_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-ssd)|
|managed-standard|[Standard_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#standard-hdd)|
|managed-premium|[Premium_LRS](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#premium-ssd)|

Также он позволяет сконфигурировать дополнительные StorageClass'ы для дисков с настраиваемыми IOPS и Throughput и отфильтровать ненужные StorageClass'ы, указав их в параметре `exclude`.

Параметры конфигурации StorageClass'ов:

* `provision` — дополнительные StorageClass'ы для [Azure ultra disks](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disk):
  * `name` — имя будущего класса;
  * `type` — тип Azure диска. Доступные значения: `Standard_LRS`, `Premium_LRS`, `StandardSSD_LRS`, `UltraSSD_LRS`, `Premium_ZRS`, `StandardSSD_ZRS`. В [Azure документации](https://docs.microsoft.com/ru-ru/azure/storage/common/storage-account-overview#types-of-storage-accounts) больше информации.
  * `cachingMode` — строка, обозначающая желаемый режим кэширования;
      Может быть `None`, `ReadOnly`, `ReadWrite`. Если размер диска превышает 4 TiB, необходимо использовать `cachineMode: None`.
      Обратитесь к [документации Azure](https://docs.microsoft.com/ru-ru/azure/virtual-machines/premium-storage-performance#disk-caching) за информацией.
  * `diskIOPSReadWrite` — количество IOPS (лимит 300 IOPS/GiB, и максимум 160 K IOPS на диск);
  * `diskMBpsReadWrite` — скорость обращения к диску, `MBps` (лимит 256 KiB/s на каждый IOPS).
* `exclude` — полные имена (или regex выражения имён) StorageClass'ов, которые не будут созданы в кластере;
* `default` — имя StorageClass'а, который будет использоваться в кластере по умолчанию:
  * Если параметр не задан, фактическим StorageClass'ом по умолчанию будет `managed-standard-ssd`.

Пример конфигурации StorageClass:

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
