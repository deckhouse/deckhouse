---
title: Хранилище и балансировка нагрузки в Basis Dynamix
permalink: ru/admin/integrations/private/dynamix/storage.html
lang: ru
---

## Хранилище

Размещение дисков виртуальных машин в Basis Dynamix определяет политика хранения (storage policy). Она задаёт набор доступных аккаунту пар из storage endpoint и пула, а также лимит IOPS. Конкретное размещение диска внутри политики выбирает Basis Dynamix.

Политику хранения для дисков всех виртуальных машин кластера задаёт параметр [`storagePolicy`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) в корне DynamixClusterConfiguration. Чтобы группа узлов использовала другую политику хранения, укажите `storagePolicy` в её `instanceClass`, в том числе в `masterNodeGroup.instanceClass`, или в [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass-v1-spec-storagepolicy). Размер корневого диска каждой виртуальной машины в гигабайтах задаёт параметр [`rootDiskSizeGb`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-rootdisksizegb).

Пример настройки:

```yaml
storagePolicy: storage_policy01
masterNodeGroup:
  replicas: 1
  instanceClass:
    rootDiskSizeGb: 50
```

Для каждой политики хранения в статусе `ENABLED` модуль создаёт StorageClass. Правила именования StorageClass и способ исключить их из кластера описаны [в FAQ модуля](/modules/cloud-provider-dynamix/faq.html#какие-storageclass-создаёт-модуль).

Отключайте политику хранения в Basis Dynamix, только когда её не используют узлы и тома кластера. При отключении политики модуль отвязывает её [от группы ресурсов кластера](/modules/cloud-provider-dynamix/environment.html#политики-хранения-в-группе-ресурсов-кластера) и удаляет её StorageClass.

### Смена политики хранения

Политика хранения диска задаётся при создании виртуальной машины и для существующей машины не меняется. Поэтому при смене политики хранения модуль пересоздаёт узлы с типом [`CloudEphemeral`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype):

- при смене `storagePolicy` в DynamixInstanceClass — узлы групп, которые используют этот DynamixInstanceClass;
- при смене `storagePolicy` в корне DynamixClusterConfiguration — узлы всех групп, в том числе групп с собственной политикой хранения.

Master-узлы и узлы групп из `nodeGroups` модуль не пересоздаёт. Новая политика хранения применяется к такому узлу только при его повторном создании.

## Балансировка нагрузки

Платформа Basis Dynamix не предоставляет встроенного балансировщика нагрузки. Для организации входящего трафика в кластер Deckhouse Platform рекомендуются следующие подходы:

1. Внешний балансировщик. Если в вашей инфраструктуре есть внешний балансировщик (аппаратный или программный), настройте проброс портов 80 и 443 на frontend-узлы кластера.

1. Использование MetalLB. Для обеспечения отказоустойчивой балансировки можно использовать MetalLB в L2-режиме.

Рекомендации:

- Выделите отдельную L2-сеть с DHCP и доступом в интернет.
- Настройте диапазон IP-адресов, из которого MetalLB будет анонсировать адреса.
- Обеспечьте подключение этой сети к frontend-узлам кластера.
- В конфигурации VirtualMachine Template оставьте сетевые интерфейсы пустыми. Deckhouse Platform создаст их автоматически.

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
