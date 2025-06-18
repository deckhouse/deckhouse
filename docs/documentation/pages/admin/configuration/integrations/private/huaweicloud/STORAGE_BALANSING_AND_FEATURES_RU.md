---
title: Хранилище и балансировка
permalink: ru/admin/integrations/privat/huaweicloud/huawei-storage.html
lang: ru
---

## Хранилище

Deckhouse Kubernetes Platform заказывает диски в Huawei Cloud с помощью CSI-драйвера. Для настройки типа хранилища используются параметры в объекте HuaweiCloudClusterConfiguration, в частности поле `volumeTypeMap`.

Пример настройки:

```yaml
masterNodeGroup:
  volumeTypeMap:
    ru-moscow-1a: SSD
```

При указании параметра `rootDiskSize` будет использоваться тот же тип диска в качестве загрузочного (root) тома.

Также в HuaweiCloudInstanceClass можно задать:

- `rootDiskSize` — размер диска в ГиБ;
- `rootDiskType` — тип корневого диска (например, SSD, GPSSD, SAS, и т.д.).

Пример:

```yaml
spec:
  rootDiskSize: 50
  rootDiskType: GPSSD
```

Рекомендуется использовать наиболее быстрые типы дисков, доступные в регионе провайдера.

## Балансировка нагрузки

Балансировка нагрузки в Huawei Cloud осуществляется с использованием сервисов ELB (Elastic Load Balancer). Для получения доступа к API балансировщика в IAM-политике пользователя необходимо разрешение [ELB FullAccess](./huawei-authorization.html).

В схеме размещения [Standard](./huawei-layout.html#standard) доступен параметр `enableEIP`, который позволяет назначать Elastic IP для master-узлов.

По умолчанию узлы, управляемые Cluster API, не получают публичные IP-адреса.
