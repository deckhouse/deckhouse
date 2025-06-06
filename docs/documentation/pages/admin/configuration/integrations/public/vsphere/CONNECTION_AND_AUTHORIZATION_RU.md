---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/vsphere/vsphere-authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse с VMware vSphere необходимо:

- Доступ к vCenter;
- Пользователь с необходимым набором прав;
- Созданные теги и категории тегов в vSphere;
- Сети с DHCP и интернетом;
- Доступные shared datastore на всех ESXi.

## Установка govc

Для настройки окружения используется CLI-инструмент [`govc`](https://github.com/vmware/govmomi/tree/main/govc). После установки задайте переменные окружения:

```console
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

## Настройка тегов и категорий

В vSphere нет встроенных понятий региона и зоны — вместо этого используются теги.

Создайте категории тегов:

```console
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Создайте теги:

```console
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Назначьте теги:

```console
govc tags.attach -c k8s-region test-region /<DatacenterName>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

## Настройка Datastore

Для корректной работы PersistentVolume необходимо, чтобы datastore был доступен на всех ESXi.

Назначьте теги:

```console
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

## Создание и назначение роли

Создайте роль с необходимыми правами:

```console
govc role.create deckhouse \
  Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
  Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
  $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Назначьте роль пользователю:

```console
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

> Для более детальной настройки прав обратитесь к [официальной документации](https://vmware.github.io/govmomi/).
