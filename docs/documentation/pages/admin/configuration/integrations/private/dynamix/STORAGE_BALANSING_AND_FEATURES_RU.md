---
title: Хранилище и балансировка нагрузки в Basis Dynamix
permalink: ru/admin/integrations/private/dynamix/storage.html
lang: ru
---

## Хранилище

Размещение дисков виртуальных машин в облаке Basis Dynamix задаётся именем storage policy:

- [`storagePolicy`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) — имя storage policy, используемой по умолчанию для дисков всех виртуальных машин кластера. Storage policy определяет набор доступных пар storage endpoint + pool и лимит IOPS; конкретное размещение внутри политики выбирает платформа. Задаётся в корне DynamixClusterConfiguration и может быть переопределена для отдельного instanceClass, в том числе в секции `masterNodeGroup.instanceClass`;
- [`rootDiskSizeGb`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-rootdisksizegb) — размер корневого диска каждой виртуальной машины (в гигабайтах).

Пример настройки:

```yaml
storagePolicy: storage_policy01
masterNodeGroup:
  replicas: 1
  instanceClass:
    rootDiskSizeGb: 50
```

{% alert level="info" %}
Указывать `storagePolicy` в instanceClass нужно только для того, чтобы для конкретной группы узлов использовать storage policy, отличную от заданной в корне DynamixClusterConfiguration.
{% endalert %}

## Балансировка нагрузки

Платформа Basis Dynamix не предоставляет встроенного балансировщика нагрузки. Для организации входящего трафика в кластер Deckhouse Kubernetes Platform рекомендуются следующие подходы:

1. Внешний балансировщик. Если в вашей инфраструктуре есть внешний балансировщик (аппаратный или программный), настройте проброс портов 80 и 443 на frontend-узлы кластера.

1. Использование MetalLB. Для обеспечения отказоустойчивой балансировки можно использовать MetalLB в L2-режиме.

Рекомендации:

- Выделите отдельную L2-сеть с DHCP и доступом в интернет.
- Настройте диапазон IP-адресов, из которого MetalLB будет анонсировать адреса.
- Обеспечьте подключение этой сети к frontend-узлам кластера.
- В конфигурации VirtualMachine Template оставьте сетевые интерфейсы пустыми — Deckhouse создаст их автоматически.

{% alert level="info" %}
Поддержка BGP-режима зависит от сетевой инфраструктуры и не гарантируется в Basis Dynamix.
{% endalert %}

### Настройка Service типа LoadBalancer

Для настройки Service типа LoadBalancer добавьте в манифест Service следующие аннотации:

```yaml
metadata:
  annotations:
    dynamix.cpi.flant.com/internal-network-name: <internal_name>
    dynamix.cpi.flant.com/external-network-name: <external_name>
```

Обе аннотации обязательны:

- `dynamix.cpi.flant.com/internal-network-name` — имя внутренней сети в Basis Dynamix;
- `dynamix.cpi.flant.com/external-network-name` — имя внешней сети в Basis Dynamix.

Термины «внутренняя сеть» и «внешняя сеть» используются в контексте Basis Dynamix. Внешняя сеть не обязательно должна быть публичной и может использовать серые IP-адреса.

Если одна из аннотаций не указана, cloud-controller-manager завершит обработку Service с ошибкой.
