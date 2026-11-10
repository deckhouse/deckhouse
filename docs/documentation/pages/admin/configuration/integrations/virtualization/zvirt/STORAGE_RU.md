---
title: Хранилище и балансировка нагрузки в zVirt
permalink: ru/admin/integrations/virtualization/zvirt/storage.html
lang: ru
---

## Хранилище

Модуль [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) заказывает диски zVirt для томов кластера через CSI-драйвер `csi.ovirt.org`. Тома заказываются стандартными ресурсами PersistentVolumeClaim со StorageClass, которые создаёт модуль.

### StorageClass для доменов хранения

Для каждого активного домена хранения (Storage Domain) zVirt модуль создаёт StorageClass, имя которого формируется из имени домена хранения. Буквы в имени переводятся в нижний регистр, пробелы заменяются дефисами, а остальные символы, кроме латинских букв, цифр, дефиса и точки, удаляются. Дефисы и точки в начале и в конце имени также удаляются.

Созданные StorageClass обладают следующими свойствами:

- диск для тома создаётся в домене хранения, которому соответствует StorageClass;
- место под диск выделяется сразу на весь объём (thin provisioning не используется);
- том создаётся, когда использующая его рабочая нагрузка назначается на узел (`volumeBindingMode: WaitForFirstConsumer`);
- размер тома можно увеличить (`allowVolumeExpansion: true`).

Если глобальный параметр [`defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) не задан и в кластере нет другого StorageClass по умолчанию, первый по алфавиту StorageClass модуля становится StorageClass по умолчанию.

Чтобы модуль не создавал StorageClass для некоторых доменов хранения, перечислите имена этих StorageClass в параметре [`excludedStorageClasses`](/modules/cloud-provider-zvirt/configuration.html#parameters-storage-parameters-excludedstorageclasses) ресурса ModuleConfig. Каждый элемент списка задаёт регулярное выражение, которое сопоставляется с именем StorageClass целиком.

Пример исключения StorageClass:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  settings:
    storage:
      parameters:
        excludedStorageClasses:
          - hostedstorage
          - backup-.*
```

В примере модуль не создаёт StorageClass `hostedstorage` и StorageClass, имена которых начинаются с `backup-`.

Чтобы отключить заказ дисков в кластере, установите для параметра [`storage.disabled`](/modules/cloud-provider-zvirt/configuration.html#parameters-storage-disabled) значение `true`. В этом случае модуль не развёртывает CSI-драйвер и не создаёт StorageClass.

### Диски виртуальных машин

Корневой диск виртуальной машины создаётся при клонировании шаблона, после чего его размер изменяется до значения параметра [`rootDiskSizeGb`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-rootdisksizegb) ресурса ZvirtInstanceClass.

На master-узлах для etcd создаётся отдельный диск. Его размер задаётся параметром [`etcdDiskSizeGb`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-etcddisksizegb), а домен хранения задаётся параметром [`storageDomainID`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-storagedomainid) ресурса ZvirtInstanceClass группы `master`. Домен хранения должен быть доступен в кластере zVirt, заданном параметром [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid).

## Балансировка нагрузки

Модуль `cloud-provider-zvirt` не создаёт балансировщики нагрузки для объектов Service с типом `LoadBalancer`. Чтобы принимать входящий трафик, используйте один из следующих вариантов:

- внешний балансировщик нагрузки, например аппаратный или программный, который направляет трафик на порты `80` и `443` [frontend-узлов](../../../../glossary.html#frontend-node) кластера;
- модуль [`metallb`](/modules/metallb/) в режиме L2, который анонсирует IP-адреса балансировщиков в сети узлов.

Для режима L2 выделите диапазон IP-адресов в сети, к которой подключены frontend-узлы через профиль vNIC из параметра `vnicProfileID`. Адреса из этого диапазона не должны выдаваться DHCP-сервером сети. Пример настройки описан [в разделе «Пример использования metallb в режиме L2 LoadBalancer»](/modules/metallb/examples.html#пример-использования-metallb-в-режиме-l2-loadbalancer).

{% alert level="info" %}
Работа модуля `metallb` в режиме BGP в zVirt не гарантируется и зависит от сетевой инфраструктуры.
{% endalert %}
