---
title: Подключение и авторизация в VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse Kubernetes Platform с VMware vSphere необходимы:

- Доступ к vCenter;
- Пользователь с необходимым набором привилегий;
- Созданные теги и категории тегов в vSphere;
- Сети с DHCP и доступом в Интернет;
- Доступные shared Datastore на всех используемых ESXi;
- Версия vSphere — `7.x` или `8.x` с поддержкой механизма [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion);
- vCenter, доступный изнутри кластера с master-узлов;
- Созданный Datacenter, в котором должны быть настроены следующие объекты:
  1. VirtualMachine template:
     - Образ виртуальной машины должен использовать `Virtual machines with hardware version 15 or later` — это необходимо для работы online resize.
     - В образе должны быть установлены пакеты `open-vm-tools`, `cloud-init` и [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) — при использовании версии `cloud-init` ниже `21.3`.
  1. Network:
     - Сеть должна быть доступна на всех ESXi, на которых планируется создание виртуальных машин.
  1. Datastore (один или несколько):
     - Datastore должен быть подключен ко всем ESXi, на которых планируется создание виртуальных машин.
     - На Datastore должен быть назначен тег из категории, указанной в параметре [`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) (по умолчанию — `k8s-zone`).  Этот тег определяет зону.
     - Все Cluster в пределах одной зоны должны иметь доступ ко всем Datastore с той же зоной.
  1. Cluster:
     - В Cluster должны быть добавлены все используемые ESXi.
     - На Cluster должен быть назначен тег из категории, указанной в параметре [zoneTagCategory](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) (по умолчанию — `k8s-zone`). Этот тег определяет зону.
  1. Folder для создаваемых виртуальных машин:
     - Параметр опционален.
     - По умолчанию используется корневой каталог виртуальных машин.
  1. Role:
     - Роль должна содержать необходимый [набор привилегий](/modules/cloud-provider-vsphere/environment.html#список-необходимых-привилегий).
  1. User:
     - Пользователю должна быть назначена роль, указанная в предыдущем пункте.
- На созданный Datacenter должен быть назначен тег из категории, указанной в параметре [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) (по умолчанию — `k8s-region`). Этот тег определяет регион.

### Требования к образу виртуальной машины

Для создания шаблона виртуальной машины (Template) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

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

DKP использует `cloud-init` для настройки виртуальной машины после запуска.

{% alert level="warning" %}
Отключите VMware Guest OS Customization (а также любые механизмы vApp/OS customization, если они применимы в вашей схеме) для шаблона и виртуальных машин кластера в vSphere. DKP выполняет первичную настройку узлов через `cloud-init` (datasource VMware GuestInfo). Включенная customization может конфликтовать с `cloud-init` и приводить к некорректной инициализации узла.
{% endalert %}

Чтобы подготовить `cloud-init` и образ ВМ, выполните следующие действия:

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

1. Перед созданием шаблона ВМ сбросьте идентификаторы и состояние `cloud-init`, используя следующие команды:

   ```shell
   truncate -s 0 /etc/machine-id &&
   rm /var/lib/dbus/machine-id &&
   ln -s /etc/machine-id /var/lib/dbus/machine-id
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

## Конфигурация vSphere

### Настройка через vSphere Client

#### Создание тегов и категорий тегов с использованием vSphere Client

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является Datacenter, а «зоной» — Cluster. Для создания этой связи используются теги.

1. Откройте vSphere Client и перейдите в «Menu» → «Tags & Custom Attributes» → «Tags».

   ![Создание тегов и категорий тегов, шаг 1](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-1.png)

1. Откройте вкладку «Categories» и нажмите «NEW». Создайте категорию для регионов (например `k8s-region`): установите значение «One tag» для параметра «Tags Per Object» и задайте связываемые типы, включая Datacenter.

   ![Создание тегов и категорий тегов, шаг 2](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-2.png)

1. Создайте вторую категорию для зон (например, `k8s-zone`) с типами объектов Host, Cluster и Datastore.

   ![Создание тегов и категорий тегов, шаг 3](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-3.png)

1. Перейдите на вкладку «Tags» и создайте минимум по одному тегу в категории региона и в категории зон (например, `test-region`, `test-zone-1`).

   ![Создание тегов и категорий тегов, шаг 4](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-4.png)

1. Во вкладке «Inventory» выберите целевой Datacenter, перейдите в панель «Summary», откройте «Actions» → «Tags & Custom Attributes» → «Assign Tag» и назначьте тег региона.
   Повторите для каждого Cluster, на котором будут узлы, назначая соответствующие теги зон.

   ![Создание тегов и категорий тегов, шаг 5.1](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-5-1.png)
   ![Создание тегов и категорий тегов, шаг 5.2](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-5-2.png)

#### Настройка Datastore с использованием vSphere Client

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi в зоне (shared datastore).
{% endalert %}

Во вкладке «Inventory» выберите Datastore, перейдите в панель «Summary», затем откройте меню «Actions» → «Tags & Custom Attributes» → «Assign Tag». Назначьте Datastore тот же тег региона, что и у соответствующего Datacenter, а также тот же тег зоны, что и у соответствующего Cluster.

![Создание тегов и категорий тегов, шаг 6](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-6.png)

#### Создание и назначение роли с использованием vSphere Client

1. Перейдите в «Menu» → «Administration» → «Access Control» → «Roles».

   ![Создание и назначение роли, шаг 1](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-1.png)

1. Нажмите «NEW», введите имя роли (например, `deckhouse`) и добавьте привилегии из [списка](/modules/cloud-provider-vsphere/environment.html#список-необходимых-привилегий).

   ![Создание и назначение роли, шаг 2](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-2.png)

1. Назначьте роль для учётной записи Deckhouse, во вкладке «Menu» → «Administration» → «Access Control» → «Global Permissions» нажмите «ADD» и выберите пользователя и роль `deckhouse`.

   ![Создание и назначение роли, шаг 3](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-3.png)

### Настройка через govc

#### Установка govc

Для дальнейшей настройки vSphere потребуется CLI-утилита [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter.

{% alert level="warning" %}
Обязательно указывайте имя пользователя вместе с доменом, например: `username@vsphere.local`.
{% endalert %}

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

#### Создание тегов и категорий тегов с использованием govc

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является Datacenter, а «зоной» — Cluster. Для создания этой связи используются теги.

Создайте категории тегов с помощью команд:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Создайте теги в каждой категории. Если вы планируете использовать несколько «зон» (Cluster), создайте тег для каждой из них:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Назначьте тег «региона» на Datacenter:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Назначьте теги «зон» на объекты Cluster:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

#### Настройка Datastore с использованием govc

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания StorageClass в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты Datastore:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

#### Создание и назначение роли с использованием govc

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя привилегии из раздела [«Список необходимых привилегий»](/modules/cloud-provider-vsphere/environment.html#список-необходимых-привилегий). При необходимости более гранулярных прав обратитесь в техподдержку Deckhouse.
{% endalert %}

Создайте роль с необходимыми привилегиями:

```shell
govc role.create deckhouse \
   Cns.Searchable \
   Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Folder.Create Folder.Delete Folder.Move Folder.Rename \
   Global.GlobalTag Global.SystemTag \
   InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory \
   InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory \
   InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory \
   InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory \
   InventoryService.Tagging.ModifyUsedByForTag InventoryService.Tagging.ObjectAttachable \
   Network.Assign \
   Resource.AssignVMToPool Resource.CreatePool Resource.DeletePool Resource.EditPool Resource.RenamePool \
   StorageProfile.View \
   System.Anonymous System.Read System.View \
   VApp.ApplicationConfig VApp.AssignResourcePool VApp.AssignVM VApp.Create VApp.Delete \
   VApp.ExtractOvfEnvironment VApp.Import VApp.InstanceConfig VApp.PowerOff VApp.PowerOn VApp.ResourceConfig \
   VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice \
   VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount \
   VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease \
   VirtualMachine.Config.EditDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory \
   VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath \
   VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo \
   VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement \
   VirtualMachine.Config.UpgradeVirtualHardware \
   VirtualMachine.GuestOperations.Query \
   VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.DeviceConnection \
   VirtualMachine.Interact.GuestControl VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn \
   VirtualMachine.Interact.Reset VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.ToolsInstall \
   VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete \
   VirtualMachine.Inventory.Move \
   VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate \
   VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs \
   VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot
```

Назначьте пользователю роль на объекте vCenter.

{% alert level="warning" %}
Обязательно указывайте имя пользователя вместе с доменом, например: `username@vsphere.local`.
{% endalert %}

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
Для более детальной настройки прав обратитесь к [официальной документации](https://pkg.go.dev/github.com/vmware/govmomi).
{% endalert %}
