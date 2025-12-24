---
title: Хранилище и балансировка
permalink: ru/admin/integrations/virtualization/zvirt/storage.html
lang: ru
---

## Хранилище

В кластере, размещённом в инфраструктуре zVirt, используются хранилища (Storage Domain), доступные в пределах заданного [`clusterID`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-clusterid). Все диски виртуальных машин создаются внутри указанного хранилища.

### Требования

- Указанный в конфигурации [`storageDomainID`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-masternodegroup-instanceclass-storagedomainid) должен быть доступен для `clusterID`, заданного в [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration);
- Диск будет создан на основе шаблона ([`template`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-masternodegroup-instanceclass-template)) и размещён в этом домене хранения;
- При заказе PersistentVolume используются root-диски машин — отдельные PVC в zVirt пока не поддерживаются.

### Конфигурация

Фрагмент ZvirtClusterConfiguration с указанием домена хранения:

```yaml
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    rootDiskSizeGb: 40
    template: ALT-p10
    vnicProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
    storageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424"
```

{% alert level="info" %}
Используйте уникальные идентификаторы (UUID) для указания шаблона и хранилища. Получить их можно через zVirt API или интерфейс управления.
{% endalert %}

## Балансировка нагрузки

Платформа zVirt не предоставляет встроенного балансировщика нагрузки. Для организации входящего трафика рекомендуются следующие подходы:

1. Использование внешнего балансировщика. Если в вашей инфраструктуре уже есть внешний балансировщик (например, аппаратный или программный), настройте проброс трафика (`80/443`) на frontend-узлы кластера.

1. Использование MetalLB. Для обеспечения отказоустойчивой балансировки в кластере можно использовать MetalLB в L2-режиме.

Рекомендации:

- Выделите отдельную L2-сеть с DHCP и интернет-доступом;
- Настройте диапазон IP-адресов, из которого MetalLB будет анонсировать адреса;
- Поддержите подключение этой сети к frontend-узлам кластера;
- В конфигурации VirtualMachine Template оставьте конфигурацию сетевых интерфейсов пустой — Deckhouse Kubernetes Platform сам добавит их на этапе запуска ВМ.

{% alert level="info" %}
Поддержка BGP-режима MetalLB в zVirt не гарантируется и зависит от сетевой инфраструктуры.
{% endalert %}
