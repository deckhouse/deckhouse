---
title: Подключение и авторизация
permalink: ru/admin/integrations/virtualization/vsphere/authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse Kubernetes Platform с VMware vSphere необходимы:

- Доступ к vCenter;
- Пользователь с необходимым набором прав;
- Созданные теги и категории тегов в vSphere;
- Сети с DHCP и интернетом;
- Доступные shared datastore на всех ESXi.

* Версия vSphere: `7.x` или `8.x` с поддержкой механизма [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion).
* vCenter: доступен изнутри кластера с master-узлов.
* Созданный Datacenter, в котором:
  1. VirtualMachine template.
     * Образ виртуальной машины должен использовать `Virtual machines with hardware version 15 or later` (необходимо для работы online resize).
     * Необходимо наличие пакетов: `open-vm-tools`, `cloud-init` и [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (при использовании версии `cloud-init` ниже 21.3).
  2. Network.
     * Должна быть доступна на всех ESXi, на которых будут создаваться виртуальные машины.
  3. Datastore (один или несколько).
     * Подключен ко всем ESXi, на которых будут создаваться виртуальные машины.
     * **Необходимо** назначение тега из категории тегов, указанных в [zoneTagCategory](#parameters-zonetagcategory) (по умолчанию `k8s-zone`). Этот тег будет обозначать **зону**. Все Cluster'ы из конкретной зоны должны иметь доступ ко всем Datastore'ам с идентичной зоной.
  4. Cluster.
     * Добавлены используемые ESXi.
     * **Необходимо** назначение тега из категории тегов, указанных в [zoneTagCategory](#parameters-zonetagcategory) (по умолчанию `k8s-zone`). Этот тег будет обозначать **зону**.
  5. Folder для создаваемых виртуальных машин.
     * Опциональный (по умолчанию используется root vm-каталог).
  6. Роль.
     * Должна содержать необходимый [набор](#список-необходимых-привилегий) прав.
  7. Пользователь.
     * Привязывается роль из п. 6.
* На созданный Datacenter **необходимо** назначить тег из категории тегов, указанный в [regionTagCategory](#parameters-regiontagcategory) (по умолчанию `k8s-region`). Этот тег будет обозначать **регион**.

### Требования к образу виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

### Подготовка образа виртуальной машины

DKP использует `cloud-init` для настройки виртуальной машины после запуска. Чтобы подготовить `cloud-init` и образ ВМ, выполните следующие действия:

1. Установите необходимые пакеты:

   Если используется версия `cloud-init` ниже 21.3 (требуется поддержка VMware GuestInfo):

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init cloud-init-vmware-guestinfo
   ```

   Если используется версия `cloud-init` 21.3 и выше:

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init
   ```

1. Проверьте, что в файле `/etc/cloud/cloud.cfg` установлен параметр `disable_vmware_customization: false`.

1. Убедитесь, что в файле `/etc/cloud/cloud.cfg` указан параметр `default_user`. Он необходим для добавления SSH-ключа при запуске ВМ.

1. Добавьте datasource VMware GuestInfo — создайте файл `/etc/cloud/cloud.cfg.d/99-DataSourceVMwareGuestInfo.cfg`:

   ```yaml
   datasource:
     VMware:
       vmware_cust_file_max_wait: 10
   ```

1. Перед созданием шаблона ВМ сбросьте идентификаторы и состояние `cloud-init`:

   ```shell
   truncate -s 0 /etc/machine-id rm /var/lib/dbus/machine-id ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Очистите логи событий `cloud-init`:

   ```shell
   cloud-init clean --logs --seed
   ```

{% alert level="warning" %}

После запуска виртуальной машины в ней должны быть запущены следующие службы, связанные с пакетами, установленными при подготовке `cloud-init`:

- `cloud-config.service`,
- `cloud-final.service`,
- `cloud-init.service`.

Чтобы убедиться в том, что службы включены, используйте команду:

```shell
systemctl is-enabled cloud-config.service cloud-init.service cloud-final.service
```

Пример ответа для включенных служб:

```console
enabled
enabled
enabled
```

{% endalert %}

{% alert %}
DKP создаёт диски виртуальных машин с типом `eagerZeroedThick`, но тип дисков созданных ВМ будет изменён без уведомления, согласно настроенным в vSphere `VM Storage Policy`.
Подробнее можно прочитать в [документации](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-single-host-management-vmware-host-client-8-0/virtual-machine-management-with-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/configuring-virtual-machines-in-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/virtual-disk-configuration-vSphereSingleHostManagementVMwareHostClient/about-virtual-disk-provisioning-policies-vSphereSingleHostManagementVMwareHostClient.html).
{% endalert %}

{% alert %}
DKP использует интерфейс `ens192`, как интерфейс по умолчанию для виртуальных машин в vSphere. Поэтому, при использовании статических IP-адресов в [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork), вы должны в образе ОС создать интерфейс с именем `ens192`, как интерфейс по умолчанию.
{% endalert %}

## Установка govc

Для настройки окружения используется CLI-инструмент [`govc`](https://github.com/vmware/govmomi/tree/main/govc). После установки задайте переменные окружения:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

## Настройка тегов и категорий

В vSphere нет встроенных понятий региона и зоны — вместо этого используются теги.

Создайте категории тегов:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Создайте теги:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Назначьте теги:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

## Настройка Datastore

Для корректной работы PersistentVolume необходимо, чтобы datastore был доступен на всех ESXi.

Назначьте теги:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

## Создание и назначение роли

Создайте роль с необходимыми правами:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   VcIdentityProviders.Read \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.' -e 'Host.Cim.' -e 'Host.Config.' -e 'Profile.' -e 'VApp.' -e 'Infraprofile.')
```

Назначьте роль пользователю:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
Для более детальной настройки прав обратитесь к [официальной документации](https://vmware.github.io/govmomi/).
{% endalert %}
