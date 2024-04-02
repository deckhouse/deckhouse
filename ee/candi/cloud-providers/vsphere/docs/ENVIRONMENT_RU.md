---
title: "Cloud provider - VMware vSphere: подготовка окружения"
description: "Настройка VMware vSphere для работы облачного провайдера Deckhouse."
---

<!-- АВТОР! Не забудь актуализировать getting started, если это необходимо -->

## Список необходимых ресурсов vSphere

* **User** с необходимым [набором прав](#создание-и-назначение-роли).
* **Network** с DHCP и доступом в интернет.
* **Datacenter** с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
* **Cluster** с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве с соответствующими [тегами](#конфигурация-datastore).
* **Template** — [подготовленный](#подготовка-образа-виртуальной-машины) образ виртуальной машины.

## Конфигурация vSphere

### Установка govc

Для дальнейшей конфигурации vSphere вам понадобится vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

### Создание тегов и категорий тегов

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является `Datacenter`, а «зоной» — `Cluster`. Для создания этой связи используются теги.

Создайте категории тегов с помощью команд:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Создайте теги в каждой категории. Если вы планируете использовать несколько «зон» (`Cluster`), создайте тег для каждой из них:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Назначьте тег «региона» на `Datacenter`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Назначьте теги «зон» на объекты `Cluster`:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

#### Конфигурация Datastore

{% alert level="warning" %}
Для динамического заказа `PersistentVolume` необходимо, чтобы `Datastore` был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания `StorageClass` в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты `Datastore`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

### Создание и назначение роли

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя все возможные права для всех компонентов Deckhouse.
Для получения детального списка привилегий, обратитесь [к документации](/documentation/v1/modules/030-cloud-provider-vsphere/configuration.html#список-привилегий-для-использования-модуля).
При необходимости получения более гранулярных прав обратитесь в техподдержку Deckhouse.
{% endalert %}

Создайте роль с необходимыми правами:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Назначьте пользователю роль на объекте `vCenter`:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

### Подготовка образа виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

#### Требования к образу виртуальной машины

Deckhouse использует `cloud-init` для настройки виртуальной машины после запуска. Для этого в образе должны быть установлены следующие пакеты:

* `open-vm-tools`
* `cloud-init`
* [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (если используется версия `cloud-init` ниже 21.3)

Для добавления SSH-ключа, в файле `/etc/cloud/cloud.cfg` должен быть указан параметр `default_user`.

## Инфраструктура

### Сети

Для работы кластера необходим VLAN с DHCP и доступом в интернет:
* Если VLAN публичный (публичные адреса), нужна вторая сеть, в которой необходимо развернуть сеть узлов кластера (в этой сети DHCP не нужен).
* Если VLAN внутренний (приватные адреса), эта же сеть будет сетью узлов кластера.

### Входящий трафик

* Если у вас имеется внутренний балансировщик запросов, можно обойтись им и направлять трафик напрямую на frontend-узлы кластера.
* Если балансировщика нет, для организации отказоустойчивых LoadBalancer'ов рекомендуется использовать MetalLB в режиме BGP. В кластере будут созданы frontend-узлы с двумя интерфейсами. Для этого дополнительно потребуются:
  * отдельный VLAN для обмена трафиком между BGP-роутерами и MetalLB. В этом VLAN'e должны быть DHCP и доступ в интернет;
  * IP-адреса BGP-роутеров;
  * ASN (номер автономной системы) на BGP-роутере;
  * ASN (номер автономной системы) в кластере;
  * диапазон, из которого анонсировать адреса.

### Использование хранилища данных

В кластере может одновременно использоваться различное количество типов хранилищ. В минимальной конфигурации потребуются:
* `Datastore`, в котором Kubernetes-кластер будет заказывать `PersistentVolume`;
* `Datastore`, в котором будут заказываться root-диски для виртуальной машины (это может быть тот же `Datastore`, что и для `PersistentVolume`).
