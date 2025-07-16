---
title: Подключение и авторизация
permalink: ru/admin/integrations/virtualization/vsphere/vsphere-authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse Kubernetes Platform с VMware vSphere необходимы:

- Доступ к vCenter;
- Пользователь с необходимым набором прав;
- Созданные теги и категории тегов в vSphere;
- Сети с DHCP и интернетом;
- Доступные shared datastore на всех ESXi.

### Подготовка образа виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

### Требования к образу виртуальной машины

DKP использует `cloud-init` для настройки виртуальной машины после запуска. Для этого в образе должны быть установлены следующие пакеты:

* `open-vm-tools`
* `cloud-init`
* [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (если используется версия `cloud-init` ниже 21.3)

Также после запуска виртуальной машины должны быть запущены следующие службы, связанные с этими пакетами:

* `cloud-config.service`
* `cloud-final.service`
* `cloud-init.service`

Для добавления SSH-ключа, в файле `/etc/cloud/cloud.cfg` должен быть указан параметр `default_user`.

{% alert %}
DKP создаёт диски виртуальных машин с типом `eagerZeroedThick`, но тип дисков созданных ВМ будет изменён без уведомления, согласно настроенным в vSphere `VM Storage Policy`.
Подробнее можно прочитать в [документации](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-single-host-management-vmware-host-client-8-0/virtual-machine-management-with-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/configuring-virtual-machines-in-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/virtual-disk-configuration-vSphereSingleHostManagementVMwareHostClient/about-virtual-disk-provisioning-policies-vSphereSingleHostManagementVMwareHostClient.html).
{% endalert %}

{% alert %}
DKP использует интерфейс `ens192`, как интерфейс по умолчанию для виртуальных машин в vSphere. Поэтому, при использовании статических IP-адресов в `mainNetwork`, вы должны в образе ОС создать интерфейс с именем `ens192`, как интерфейс по умолчанию.
{% endalert %}

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

{% alert level="info" %}
Для более детальной настройки прав обратитесь к [официальной документации](https://vmware.github.io/govmomi/).
{% endalert %}
