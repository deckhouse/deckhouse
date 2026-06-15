---
title: "Модуль csi-vsphere: подготовка окружения"
description: "Настройка тегов, datastore, пользователя и прав в vSphere перед включением модуля csi-vsphere."
---

В этом разделе описана подготовка vCenter и vSphere к работе модуля `csi-vsphere`.

## Список необходимых ресурсов

Для работы модуля требуется наличие следующих ресурсов:

- User с необходимым [набором прав](#создание-и-назначение-роли).
- Network с DHCP и доступом в интернет.
- Datacenter с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
- Cluster с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
- Datastore в любом количестве с соответствующими [тегами](#конфигурация-datastore).

## Установка govc

Для дальнейшей конфигурации `csi-vsphere` понадобится vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

## Создание тегов и категорий тегов

В `csi-vsphere` нет понятий «регион» и «зона». «Регионом» в `csi-vsphere` является Datacenter, а «зоной» — Cluster. Связь между этими объектами задаётся через теги.

Чтобы связать объекты Cluster и Datacenter, выполните следующие действия:

1. Создайте категории тегов:

   ```shell
   govc tags.category.create -d "Kubernetes Region" k8s-region
   govc tags.category.create -d "Kubernetes Zone" k8s-zone
   ```

1. Создайте теги в каждой категории. Если планируется несколько зон (Cluster), создайте отдельный тег для каждой:

   ```shell
   govc tags.create -d "Kubernetes Region" -c k8s-region test-region
   govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
   govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
   ```

1. Назначьте тег региона на Datacenter:

   ```shell
   govc tags.attach -c k8s-region test-region /<DatacenterName>
   ```

1. Назначьте теги зон на объекты Cluster:

   ```shell
   govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
   govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
   ```

## Конфигурация Datastore

{% alert level="info" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на каждом хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания StorageClass в кластере назначьте теги региона и зоны на объекты Datastore:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

## Создание и назначение роли

{% alert level="info" %}
Ввиду разнообразия подключаемых к `csi-vsphere` SSO-провайдеров шаги по созданию пользователя в данном разделе не рассматриваются.

Предлагаемая ниже роль включает права, достаточные для всех компонентов DKP. Детальный перечень привилегий описан в документации модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/environment.html#список-необходимых-привилегий).
{% endalert %}

Создайте роль с необходимыми правами:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Назначьте пользователю роль на объекте vCenter:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```
