---
title: "Cloud provider - VMware vSphere: подготовка окружения"
description: "Настройка VMware vSphere для работы облачного провайдера Deckhouse."
---

<!-- АВТОР! Не забудь актуализировать getting started, если это необходимо -->

## Требования к окружению

* Версия vSphere: `7.x` или `8.x` с поддержкой механизма [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion).
* vCenter, до которого есть доступ изнутри кластера с master-узлов.
* Создать Datacenter, в котором создать:
  1. VirtualMachine template.
     * Образ виртуальной машины должен использовать `Virtual machines with hardware version 15 or later` (необходимо для работы online resize).
     * В образе должны быть установлены следующие пакеты: `open-vm-tools`, `cloud-init` и [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (если используется версия `cloud-init` ниже 21.3).
  1. Network, доступную на всех ESXi, на которых будут создаваться виртуальные машины.
  1. Datastore (или несколько), подключенный ко всем ESXi, на которых будут создаваться виртуальные машины.
     * На Datastore'ы **необходимо** «повесить» тег из категории тегов, указанных в [`zoneTagCategory`](./configuration.html#parameters-zonetagcategory) (по умолчанию `k8s-zone`). Этот тег будет обозначать **зону**. Все Cluster'ы из конкретной зоны должны иметь доступ ко всем Datastore'ам с идентичной зоной.
  1. Cluster, в который добавить необходимые используемые ESXi.
     * На Cluster **необходимо** «повесить» тег из категории тегов, указанных в [`zoneTagCategory`](./configuration.html#parameters-zonetagcategory) (по умолчанию `k8s-zone`). Этот тег будет обозначать **зону**.
  1. Folder для создаваемых виртуальных машин.
     * Опциональный. По умолчанию будет использоваться root vm-каталог.
  1. Роль с необходимым [набором](#список-необходимых-привилегий) прав.
  1. Пользователя, привязав к нему роль из п. 6.
* На созданный Datacenter **необходимо** «повесить» тег из категории тегов, указанный в [`regionTagCategory`](./configuration.html#parameters-regiontagcategory) (по умолчанию `k8s-region`). Этот тег будет обозначать **регион**.

## Список необходимых ресурсов vSphere

* **User** с необходимым [набором прав](#создание-и-назначение-роли).
* **Network** с DHCP и доступом в интернет.
* **Datacenter** с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
* **Cluster** с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве с соответствующими [тегами](#конфигурация-datastore).
* **Template** — [подготовленный](#подготовка-образа-виртуальной-машины) образ виртуальной машины.

## Список необходимых привилегий

> О том, как создать и назначить роль пользователю, читайте в разделе [«Создание и назначение роли»](#создание-и-назначение-роли).

Детальный список привилегий, необходимых для работы Deckhouse Kubernetes Platform в vSphere:

<table>
  <thead>
    <tr>
        <th>Список привилегий</th>
        <th>Назначение</th>
    </tr>
  </thead>
  <tbody>
    <tr>
        <td><code>Cns.Searchable</code><br><code>StorageProfile.View</code><br><code>Datastore.AllocateSpace</code><br><code>Datastore.Browse</code><br><code>Datastore.FileManagement</code></td>
        <td>Выделение дисков при создании виртуальных машин и заказе <code>PersistentVolumes</code> в кластере.</td>
    </tr>
    <tr>
        <td><code>Global.GlobalTag</code><br><code>Global.SystemTag</code><br><code>InventoryService.Tagging.AttachTag</code><br><code>InventoryService.Tagging.CreateCategory</code><br><code>InventoryService.Tagging.CreateTag</code><br><code>InventoryService.Tagging.DeleteCategory</code><br><code>InventoryService.Tagging.DeleteTag</code><br><code>InventoryService.Tagging.EditCategory</code><br><code>InventoryService.Tagging.EditTag</code><br><code>InventoryService.Tagging.ModifyUsedByForCategory</code><br><code>InventoryService.Tagging.ModifyUsedByForTag</code><br><code>InventoryService.Tagging.ObjectAttachable</code></td>
        <td>Deckhouse Kubernetes Platform использует теги для определения доступных ему объектов <code>Datacenter</code>, <code>Cluster</code> и <code>Datastore</code>, а также для определения виртуальных машин, находящихся под его управлением.</td>
    </tr>
    <tr>
        <td><code>Folder.Create</code><br><code>Folder.Delete</code><br><code>Folder.Move</code><br><code>Folder.Rename</code></td>
        <td>Группировка кластера Deckhouse Kubernetes Platform в одном <code>Folder</code> в vSphere Inventory.</td>
    </tr>
    <tr>
        <td><code>Network.Assign</code><br><code>Resource.ApplyRecommendation</code><br><code>Resource.AssignVAppToPool</code><br><code>Resource.AssignVMToPool</code><br><code>Resource.ColdMigrate</code><br><code>Resource.CreatePool</code><br><code>Resource.DeletePool</code><br><code>Resource.EditPool</code><br><code>Resource.HotMigrate</code><br><code>Resource.MovePool</code><br><code>Resource.QueryVMotion</code><br><code>Resource.RenamePool</code><br><code>VirtualMachine.Config.AddExistingDisk</code><br><code>VirtualMachine.Config.AddNewDisk</code><br><code>VirtualMachine.Config.AddRemoveDevice</code><br><code>VirtualMachine.Config.AdvancedConfig</code><br><code>VirtualMachine.Config.Annotation</code><br><code>VirtualMachine.Config.ChangeTracking</code><br><code>VirtualMachine.Config.CPUCount</code><br><code>VirtualMachine.Config.DiskExtend</code><br><code>VirtualMachine.Config.DiskLease</code><br><code>VirtualMachine.Config.EditDevice</code><br><code>VirtualMachine.Config.HostUSBDevice</code><br><code>VirtualMachine.Config.ManagedBy</code><br><code>VirtualMachine.Config.Memory</code><br><code>VirtualMachine.Config.MksControl</code><br><code>VirtualMachine.Config.QueryFTCompatibility</code><br><code>VirtualMachine.Config.QueryUnownedFiles</code><br><code>VirtualMachine.Config.RawDevice</code><br><code>VirtualMachine.Config.ReloadFromPath</code><br><code>VirtualMachine.Config.RemoveDisk</code><br><code>VirtualMachine.Config.Rename</code><br><code>VirtualMachine.Config.ResetGuestInfo</code><br><code>VirtualMachine.Config.Resource</code><br><code>VirtualMachine.Config.Settings</code><br><code>VirtualMachine.Config.SwapPlacement</code><br><code>VirtualMachine.Config.ToggleForkParent</code><br><code>VirtualMachine.Config.UpgradeVirtualHardware</code><br><code>VirtualMachine.GuestOperations.Execute</code><br><code>VirtualMachine.GuestOperations.Modify</code><br><code>VirtualMachine.GuestOperations.ModifyAliases</code><br><code>VirtualMachine.GuestOperations.Query</code><br><code>VirtualMachine.GuestOperations.QueryAliases</code><br><code>VirtualMachine.Hbr.ConfigureReplication</code><br><code>VirtualMachine.Hbr.MonitorReplication</code><br><code>VirtualMachine.Hbr.ReplicaManagement</code><br><code>VirtualMachine.Interact.AnswerQuestion</code><br><code>VirtualMachine.Interact.Backup</code><br><code>VirtualMachine.Interact.ConsoleInteract</code><br><code>VirtualMachine.Interact.CreateScreenshot</code><br><code>VirtualMachine.Interact.CreateSecondary</code><br><code>VirtualMachine.Interact.DefragmentAllDisks</code><br><code>VirtualMachine.Interact.DeviceConnection</code><br><code>VirtualMachine.Interact.DisableSecondary</code><br><code>VirtualMachine.Interact.DnD</code><br><code>VirtualMachine.Interact.EnableSecondary</code><br><code>VirtualMachine.Interact.GuestControl</code><br><code>VirtualMachine.Interact.MakePrimary</code><br><code>VirtualMachine.Interact.Pause</code><br><code>VirtualMachine.Interact.PowerOff</code><br><code>VirtualMachine.Interact.PowerOn</code><br><code>VirtualMachine.Interact.PutUsbScanCodes</code><br><code>VirtualMachine.Interact.Record</code><br><code>VirtualMachine.Interact.Replay</code><br><code>VirtualMachine.Interact.Reset</code><br><code>VirtualMachine.Interact.SESparseMaintenance</code><br><code>VirtualMachine.Interact.SetCDMedia</code><br><code>VirtualMachine.Interact.SetFloppyMedia</code><br><code>VirtualMachine.Interact.Suspend</code><br><code>VirtualMachine.Interact.SuspendToMemory</code><br><code>VirtualMachine.Interact.TerminateFaultTolerantVM</code><br><code>VirtualMachine.Interact.ToolsInstall</code><br><code>VirtualMachine.Interact.TurnOffFaultTolerance</code><br><code>VirtualMachine.Inventory.Create</code><br><code>VirtualMachine.Inventory.CreateFromExisting</code><br><code>VirtualMachine.Inventory.Delete</code><br><code>VirtualMachine.Inventory.Move</code><br><code>VirtualMachine.Inventory.Register</code><br><code>VirtualMachine.Inventory.Unregister</code><br><code>VirtualMachine.Namespace.Event</code><br><code>VirtualMachine.Namespace.EventNotify</code><br><code>VirtualMachine.Namespace.Management</code><br><code>VirtualMachine.Namespace.ModifyContent</code><br><code>VirtualMachine.Namespace.Query</code><br><code>VirtualMachine.Namespace.ReadContent</code><br><code>VirtualMachine.Provisioning.Clone</code><br><code>VirtualMachine.Provisioning.CloneTemplate</code><br><code>VirtualMachine.Provisioning.CreateTemplateFromVM</code><br><code>VirtualMachine.Provisioning.Customize</code><br><code>VirtualMachine.Provisioning.DeployTemplate</code><br><code>VirtualMachine.Provisioning.DiskRandomAccess</code><br><code>VirtualMachine.Provisioning.DiskRandomRead</code><br><code>VirtualMachine.Provisioning.FileRandomAccess</code><br><code>VirtualMachine.Provisioning.GetVmFiles</code><br><code>VirtualMachine.Provisioning.MarkAsTemplate</code><br><code>VirtualMachine.Provisioning.MarkAsVM</code><br><code>VirtualMachine.Provisioning.ModifyCustSpecs</code><br><code>VirtualMachine.Provisioning.PromoteDisks</code><br><code>VirtualMachine.Provisioning.PutVmFiles</code><br><code>VirtualMachine.Provisioning.ReadCustSpecs</code><br><code>VirtualMachine.State.CreateSnapshot</code><br><code>VirtualMachine.State.RemoveSnapshot</code><br><code>VirtualMachine.State.RenameSnapshot</code><br><code>VirtualMachine.State.RevertToSnapshot</code></td>
        <td>Управление жизненным циклом виртуальных машин кластера Deckhouse Kubernetes Platform.</td>
    </tr>
  </tbody>
</table>

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

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

### Создание и назначение роли

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя все возможные права для всех компонентов Deckhouse.
Детальный список привилегий описан в разделе [«Список необходимых привилегий»](#список-необходимых-привилегий).
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

#### Подготовка образа виртуальной машины

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
