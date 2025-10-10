---
title: Хранилище и балансировка
permalink: ru/admin/integrations/private/dynamix/storage.html
lang: ru
---

## Хранилище

Хранение данных в облаке Dynamix осуществляется с использованием:

- [`storageEndpoint`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-storageendpoint) — имя хранилища, предоставленное провайдером;
- [`pool`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-pool) — имя пула хранения внутри указанного хранилища;
- [`rootDiskSizeGb`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-rootdisksizegb) — размер корневого диска каждой виртуальной машины (в гигабайтах).

Эти параметры задаются в секции instanceClass как для master-узлов, так и для рабочих групп узлов (NodeGroup).

Пример настройки:

```yaml
masterNodeGroup:
  replicas: 1
  instanceClass:
    rootDiskSizeGb: 50
    storageEndpoint: SharedTatlin_G1_SEP
    pool: pool_a
```

{% alert level="info" %}
В текущей версии поддерживается только одно хранилище на группу узлов.
{% endalert %}

## Балансировка нагрузки

Платформа Dynamix не предоставляет встроенного балансировщика нагрузки. Для организации входящего трафика в кластер Deckhouse Kubernetes Platform рекомендуются следующие подходы:

1. Внешний балансировщик. Если в вашей инфраструктуре есть внешний балансировщик (аппаратный или программный), настройте проброс портов 80 и 443 на frontend-узлы кластера.

1. Использование MetalLB. Для обеспечения отказоустойчивой балансировки можно использовать MetalLB в L2-режиме.

Рекомендации:

- Выделите отдельную L2-сеть с DHCP и доступом в интернет.
- Настройте диапазон IP-адресов, из которого MetalLB будет анонсировать адреса.
- Обеспечьте подключение этой сети к frontend-узлам кластера.
- В конфигурации VirtualMachine Template оставьте сетевые интерфейсы пустыми — Deckhouse создаст их автоматически.

{% alert level="info" %}
Поддержка BGP-режима зависит от сетевой инфраструктуры и не гарантируется в Dynamix.
{% endalert %}
