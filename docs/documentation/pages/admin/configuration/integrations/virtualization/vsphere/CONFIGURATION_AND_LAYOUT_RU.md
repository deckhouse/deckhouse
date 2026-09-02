---
title: Схемы размещения и настройка VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/layout.html
lang: ru
---

## Standard

Схема Standard предназначена для размещения кластера внутри инфраструктуры vSphere с возможностью управления ресурсами, сетями и хранилищем.

Особенности:

- Использование vSphere Datacenter в качестве региона ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region));
- Использование vSphere Cluster в качестве зоны ([`zone`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones));
- Поддержка нескольких зон и размещения узлов по зонам;
- Использование различных datastore для дисков и volume’ов;
- Поддержка подключения сетей, включая дополнительную сетевую изоляцию (например, MetalLB + BGP).

![resources](../../../../images/cloud-provider-vsphere/vsphere-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11345&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
masterNodeGroup:
  replicas: 1
  zones:
    - ru-central1-a
    - ru-central1-b
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
nodeGroups:
  - name: khm
    replicas: 1
    zones:
      - ru-central1-a
    instanceClass:
      numCPUs: 4
      memory: 8192
      template: dev/golden_image
      datastore: dev/lun_1
      mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
  - ru-central1-a
  - ru-central1-b
```

Обязательные параметры [ресурса VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration):

- `layout` — название схемы размещения. Поддерживается только `Standard`;
- `provider` — параметры подключения к vCenter;
- `region` — тег, присвоенный объекту Datacenter;
- `zoneTagCategory` и `regionTagCategory` — категории тегов, по которым распознаются регионы и зоны;
- `zones` — список зон, доступных для размещения узлов;
- `masterNodeGroup` — параметры группы master-узлов;
- `vmFolderPath` — путь до директории, в которой будут размещаться виртуальные машины кластера;
- `sshPublicKey` — публичный SSH-ключ для доступа к узлам.

Параметр `internalNetworkCIDR` необязательный. DKP применяет его, только если в `masterInstanceClass` заданы `additionalNetworks`, и выделяет адреса master-узлов из этой подсети начиная с десятого адреса. Если группа master-узлов использует одну сеть, параметр можно не указывать.

{% alert level="info" %}
Все узлы, размещённые в разных зонах, должны иметь доступ к общим datastore с аналогичными тегами зоны.
{% endalert %}

## Список необходимых привилегий

Роль для учётной записи платформы включает привилегии, перечисленные ниже. Привилегии сгруппированы по задачам, которые платформа выполняет во vSphere.

Как создать роль и назначить её пользователю, описано в разделах [«Создание и назначение роли с использованием vSphere Client»](authorization.html#создание-и-назначение-роли-с-использованием-vsphere-client) и [«Создание и назначение роли с использованием govc»](authorization.html#создание-и-назначение-роли-с-использованием-govc).

### Базовый доступ

vSphere назначает эти привилегии автоматически при создании любой роли. Они дают компонентам платформы доступ на чтение объектов vSphere Inventory.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| — | `System.Anonymous` |
| — | `System.Read` |
| — | `System.View` |

### Теги регионов и зон

По тегам платформа определяет доступные ей объекты Datacenter, Cluster и Datastore и помечает виртуальные машины, которыми управляет.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Global tag | `Global.GlobalTag` |
| System tag | `Global.SystemTag` |
| Assign or Unassign vSphere Tag | `InventoryService.Tagging.AttachTag` |
| Assign or Unassign vSphere Tag on Object | `InventoryService.Tagging.ObjectAttachable` |
| Create vSphere Tag | `InventoryService.Tagging.CreateTag` |
| Create vSphere Tag Category | `InventoryService.Tagging.CreateCategory` |
| Delete vSphere Tag | `InventoryService.Tagging.DeleteTag` |
| Delete vSphere Tag Category | `InventoryService.Tagging.DeleteCategory` |
| Edit vSphere Tag | `InventoryService.Tagging.EditTag` |
| Edit vSphere Tag Category | `InventoryService.Tagging.EditCategory` |
| Modify UsedBy Field for Category | `InventoryService.Tagging.ModifyUsedByForCategory` |
| Modify UsedBy Field for Tag | `InventoryService.Tagging.ModifyUsedByForTag` |

### Хранилище

Привилегии нужны для размещения дисков виртуальных машин, динамического заказа PersistentVolume и чтения политик хранения SPBM. В vSphere 7 категория `StorageProfile` называется «Profile-driven storage».

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Searchable | `Cns.Searchable` |
| Allocate space | `Datastore.AllocateSpace` |
| Browse datastore | `Datastore.Browse` |
| Low level file operations | `Datastore.FileManagement` |
| View VM storage policies | `StorageProfile.View` |

### Размещение виртуальных машин

Платформа группирует виртуальные машины кластера в отдельной директории, размещает их в пуле ресурсов и подключает к сетям.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Create folder | `Folder.Create` |
| Delete folder | `Folder.Delete` |
| Move folder | `Folder.Move` |
| Rename folder | `Folder.Rename` |
| Assign virtual machine to resource pool | `Resource.AssignVMToPool` |
| Create resource pool | `Resource.CreatePool` |
| Modify resource pool | `Resource.EditPool` |
| Remove resource pool | `Resource.DeletePool` |
| Rename resource pool | `Resource.RenamePool` |
| Assign network | `Network.Assign` |

### Создание виртуальных машин

Виртуальные машины создаются клонированием подготовленного шаблона и регистрируются в инвентаре vSphere.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Clone virtual machine | `VirtualMachine.Provisioning.Clone` |
| Deploy template | `VirtualMachine.Provisioning.DeployTemplate` |
| Customize guest | `VirtualMachine.Provisioning.Customize` |
| Read customization specifications | `VirtualMachine.Provisioning.ReadCustSpecs` |
| Allow virtual machine download | `VirtualMachine.Provisioning.GetVmFiles` |
| Allow virtual machine files upload | `VirtualMachine.Provisioning.PutVmFiles` |
| Create new | `VirtualMachine.Inventory.Create` |
| Create from existing | `VirtualMachine.Inventory.CreateFromExisting` |
| Remove | `VirtualMachine.Inventory.Delete` |
| Move | `VirtualMachine.Inventory.Move` |

### Настройка виртуальных машин

Платформа задаёт параметры виртуальных машин при создании и меняет их при изменении группы узлов или инстанс-класса.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Add new disk | `VirtualMachine.Config.AddNewDisk` |
| Add existing disk | `VirtualMachine.Config.AddExistingDisk` |
| Remove disk | `VirtualMachine.Config.RemoveDisk` |
| Extend virtual disk | `VirtualMachine.Config.DiskExtend` |
| Acquire disk lease | `VirtualMachine.Config.DiskLease` |
| Toggle disk change tracking | `VirtualMachine.Config.ChangeTracking` |
| Configure Raw device | `VirtualMachine.Config.RawDevice` |
| Change CPU count | `VirtualMachine.Config.CPUCount` |
| Change Memory | `VirtualMachine.Config.Memory` |
| Change resource | `VirtualMachine.Config.Resource` |
| Change Swapfile placement | `VirtualMachine.Config.SwapPlacement` |
| Add or remove device | `VirtualMachine.Config.AddRemoveDevice` |
| Modify device settings | `VirtualMachine.Config.EditDevice` |
| Change Settings | `VirtualMachine.Config.Settings` |
| Advanced configuration | `VirtualMachine.Config.AdvancedConfig` |
| Set annotation | `VirtualMachine.Config.Annotation` |
| Rename | `VirtualMachine.Config.Rename` |
| Configure managedBy | `VirtualMachine.Config.ManagedBy` |
| Reset guest information | `VirtualMachine.Config.ResetGuestInfo` |
| Query unowned files | `VirtualMachine.Config.QueryUnownedFiles` |
| Reload from path | `VirtualMachine.Config.ReloadFromPath` |
| Upgrade virtual machine compatibility | `VirtualMachine.Config.UpgradeVirtualHardware` |

### Управление состоянием виртуальных машин

Привилегии нужны для включения и выключения виртуальных машин, подключения устройств, чтения сведений из гостевой операционной системы и работы со снимками.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Power On | `VirtualMachine.Interact.PowerOn` |
| Power Off | `VirtualMachine.Interact.PowerOff` |
| Reset | `VirtualMachine.Interact.Reset` |
| Answer question | `VirtualMachine.Interact.AnswerQuestion` |
| Device connection | `VirtualMachine.Interact.DeviceConnection` |
| Configure CD media | `VirtualMachine.Interact.SetCDMedia` |
| Install VMware Tools | `VirtualMachine.Interact.ToolsInstall` |
| Guest operating system management by VIX API | `VirtualMachine.Interact.GuestControl` |
| Guest Operation Queries | `VirtualMachine.GuestOperations.Query` |
| Create snapshot | `VirtualMachine.State.CreateSnapshot` |
| Remove Snapshot | `VirtualMachine.State.RemoveSnapshot` |
| Rename Snapshot | `VirtualMachine.State.RenameSnapshot` |

### vApp

Операции с vApp и OVF-шаблонами. Требуются, если шаблоны виртуальных машин или сами машины входят в состав vApp.

| Привилегия в UI | Привилегия в API |
| --- | --- |
| Create | `VApp.Create` |
| Delete | `VApp.Delete` |
| Import | `VApp.Import` |
| Add virtual machine | `VApp.AssignVM` |
| Assign resource pool | `VApp.AssignResourcePool` |
| Power On | `VApp.PowerOn` |
| Power Off | `VApp.PowerOff` |
| vApp application configuration | `VApp.ApplicationConfig` |
| vApp instance configuration | `VApp.InstanceConfig` |
| vApp resource configuration | `VApp.ResourceConfig` |
| View OVF Environment | `VApp.ExtractOvfEnvironment` |
