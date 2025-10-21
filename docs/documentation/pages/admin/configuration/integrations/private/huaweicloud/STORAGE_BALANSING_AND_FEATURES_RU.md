---
title: Хранилище и балансировка
permalink: ru/admin/integrations/private/huaweicloud/storage.html
lang: ru
---

## Хранилище

Deckhouse Kubernetes Platform заказывает диски в Huawei Cloud с помощью CSI-драйвера. Для настройки типа хранилища используются параметры в объекте HuaweiCloudClusterConfiguration, в частности [поле `volumeTypeMap`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-masternodegroup-volumetypemap).

Пример настройки:

```yaml
masterNodeGroup:
  volumeTypeMap:
    ru-moscow-1a: SSD
```

При указании параметра `rootDiskSize` будет использоваться тот же тип диска в качестве загрузочного (root) тома.

Также в HuaweiCloudInstanceClass можно задать:

- [`rootDiskSize`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-rootdisksize) — размер диска в ГиБ;
- [`rootDiskType`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-rootdisktype) — тип корневого диска (например, SSD, GPSSD, SAS, и т.д.).

Пример:

```yaml
spec:
  rootDiskSize: 50
  rootDiskType: GPSSD
```

Рекомендуется использовать наиболее быстрые типы дисков, доступные в регионе провайдера.

## Балансировка нагрузки

Балансировка нагрузки в Huawei Cloud осуществляется с использованием сервисов ELB (Elastic Load Balancer). Для получения доступа к API балансировщика в IAM-политике пользователя необходимо разрешение [`ELB FullAccess`](./authorization.html).

В схеме размещения [Standard](./layout.html#standard) доступен [параметр `enableEIP`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-standard-enableeip), который позволяет назначать Elastic IP для master-узлов.

По умолчанию узлы, управляемые Cluster API, не получают публичные IP-адреса.
