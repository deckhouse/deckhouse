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
internalNetworkCIDR: 192.168.199.0/24
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

Параметр `internalNetworkCIDR` обязателен, если в конфигурации есть `nodeGroups`. Установщик проверяет его при создании статических узлов и без него завершается с ошибкой. Он также нужен, если в `masterNodeGroup.instanceClass` заданы `additionalNetworks`. В этом случае DKP выделяет адреса master-узлов из указанной подсети начиная с десятого адреса. Если групп `nodeGroups` нет и master-узлы используют одну сеть, параметр можно не указывать.

{% alert level="info" %}
Все узлы, размещённые в разных зонах, должны иметь доступ к общим datastore с аналогичными тегами зоны.
{% endalert %}

## Список необходимых привилегий

Роль для учётной записи платформы включает привилегии, перечисленные ниже. Привилегии сгруппированы по задачам, которые платформа выполняет во vSphere.

Как создать роль и назначить её пользователю, описано в разделах [«Создание и назначение роли с использованием vSphere Client»](authorization.html#создание-и-назначение-роли-с-использованием-vsphere-client) и [«Создание и назначение роли с использованием govc»](authorization.html#создание-и-назначение-роли-с-использованием-govc).

### Базовый доступ

vSphere назначает эти привилегии автоматически при создании любой роли. Они дают компонентам платформы доступ на чтение объектов vSphere Inventory.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| — | `System.Anonymous` | Обращение к методам vCenter, не требующим авторизации |
| — | `System.Read` | Чтение состояния и параметров объектов |
| — | `System.View` | Просмотр объектов инвентаря |

### Теги регионов и зон

По тегам платформа определяет доступные ей объекты Datacenter, Cluster и Datastore и помечает виртуальные машины, которыми управляет.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Global tag | `Global.GlobalTag` | Работа с глобальными тегами vCenter |
| System tag | `Global.SystemTag` | Работа с системными тегами vCenter |
| Assign or Unassign vSphere Tag | `InventoryService.Tagging.AttachTag` | Чтение тегов региона и зоны и пометка виртуальных машин кластера |
| Assign or Unassign vSphere Tag on Object | `InventoryService.Tagging.ObjectAttachable` | Назначение тега конкретному объекту инвентаря |
| Create vSphere Tag | `InventoryService.Tagging.CreateTag` | Создание тегов, которыми помечаются виртуальные машины кластера |
| Create vSphere Tag Category | `InventoryService.Tagging.CreateCategory` | Создание категорий `deckhouse-cluster-name` и `deckhouse-node-role` для этих тегов |
| Delete vSphere Tag | `InventoryService.Tagging.DeleteTag` | Удаление тегов, созданных платформой |
| Delete vSphere Tag Category | `InventoryService.Tagging.DeleteCategory` | Удаление категорий, созданных платформой |
| Edit vSphere Tag | `InventoryService.Tagging.EditTag` | Изменение тегов, созданных платформой |
| Edit vSphere Tag Category | `InventoryService.Tagging.EditCategory` | Изменение категорий, созданных платформой |
| Modify UsedBy Field for Category | `InventoryService.Tagging.ModifyUsedByForCategory` | Изменение служебного поля UsedBy у категории |
| Modify UsedBy Field for Tag | `InventoryService.Tagging.ModifyUsedByForTag` | Изменение служебного поля UsedBy у тега |

### Хранилище

Привилегии нужны для размещения дисков виртуальных машин, динамического заказа PersistentVolume и чтения политик хранения SPBM.

{% alert level="info" %}
В vSphere 7 привилегия `StorageProfile.View` находится в интерфейсе в разделе «Profile-driven storage».
{% endalert %}

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Searchable | `Cns.Searchable` | Поиск дисков CNS во всём vCenter при обнаружении ресурсов |
| Allocate space | `Datastore.AllocateSpace` | Выделение места под диски узлов и тома PersistentVolume |
| Browse datastore | `Datastore.Browse` | Просмотр файлов на Datastore |
| Low level file operations | `Datastore.FileManagement` | Операции с файлами дисков на Datastore |
| View VM storage policies | `StorageProfile.View` | Чтение политик хранения SPBM для создания StorageClass |

### Размещение виртуальных машин

Платформа группирует виртуальные машины кластера в отдельной директории, размещает их в пуле ресурсов и подключает к сетям.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Create folder | `Folder.Create` | Создание директории по пути из параметра [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) |
| Delete folder | `Folder.Delete` | Удаление этой директории вместе с кластером |
| Move folder | `Folder.Move` | Перемещение директории при изменении пути |
| Rename folder | `Folder.Rename` | Переименование директории при изменении пути |
| Assign virtual machine to resource pool | `Resource.AssignVMToPool` | Размещение виртуальных машин в пуле ресурсов |
| Create resource pool | `Resource.CreatePool` | Создание вложенного пула ресурсов в каждой зоне |
| Modify resource pool | `Resource.EditPool` | Изменение параметров этого пула |
| Remove resource pool | `Resource.DeletePool` | Удаление пула вместе с кластером |
| Rename resource pool | `Resource.RenamePool` | Переименование пула |
| Assign network | `Network.Assign` | Подключение виртуальных машин к сетям из параметров [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) и [`additionalNetworks`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-additionalnetworks) |

### Создание виртуальных машин

Виртуальные машины создаются клонированием подготовленного шаблона и регистрируются в инвентаре vSphere.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Clone virtual machine | `VirtualMachine.Provisioning.Clone` | Клонирование шаблона из параметра [`template`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-template) |
| Deploy template | `VirtualMachine.Provisioning.DeployTemplate` | Развёртывание виртуальной машины из шаблона |
| Customize guest | `VirtualMachine.Provisioning.Customize` | Настройка гостевой операционной системы при клонировании |
| Read customization specifications | `VirtualMachine.Provisioning.ReadCustSpecs` | Чтение спецификаций настройки гостевой операционной системы |
| Allow virtual machine download | `VirtualMachine.Provisioning.GetVmFiles` | Чтение файлов виртуальной машины |
| Allow virtual machine files upload | `VirtualMachine.Provisioning.PutVmFiles` | Запись файлов виртуальной машины |
| Create new | `VirtualMachine.Inventory.Create` | Создание виртуальной машины в инвентаре |
| Create from existing | `VirtualMachine.Inventory.CreateFromExisting` | Создание виртуальной машины на основе существующей |
| Remove | `VirtualMachine.Inventory.Delete` | Удаление виртуальной машины при уменьшении числа узлов |
| Move | `VirtualMachine.Inventory.Move` | Перемещение виртуальной машины в директорию кластера |

### Настройка виртуальных машин

Платформа задаёт параметры виртуальных машин при создании и меняет их при изменении группы узлов или инстанс-класса.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Add new disk | `VirtualMachine.Config.AddNewDisk` | Создание root-диска виртуальной машины |
| Add existing disk | `VirtualMachine.Config.AddExistingDisk` | Подключение существующего диска |
| Remove disk | `VirtualMachine.Config.RemoveDisk` | Отключение диска |
| Extend virtual disk | `VirtualMachine.Config.DiskExtend` | Увеличение диска до размера из параметра [`rootDiskSize`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-rootdisksize) и расширение томов |
| Acquire disk lease | `VirtualMachine.Config.DiskLease` | Получение блокировки диска на время операций с ним |
| Toggle disk change tracking | `VirtualMachine.Config.ChangeTracking` | Управление отслеживанием изменённых блоков диска |
| Configure Raw device | `VirtualMachine.Config.RawDevice` | Настройка проброшенных устройств (RDM) |
| Change CPU count | `VirtualMachine.Config.CPUCount` | Задание числа vCPU из параметра [`numCPUs`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-numcpus) |
| Change Memory | `VirtualMachine.Config.Memory` | Задание объёма памяти из параметра [`memory`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-memory) |
| Change resource | `VirtualMachine.Config.Resource` | Резервирование памяти из параметра [`memoryReservation`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-runtimeoptions-memoryreservation) и ограничение ресурсов |
| Change Swapfile placement | `VirtualMachine.Config.SwapPlacement` | Выбор места для файла подкачки |
| Add or remove device | `VirtualMachine.Config.AddRemoveDevice` | Добавление и удаление устройств, в том числе сетевых адаптеров |
| Modify device settings | `VirtualMachine.Config.EditDevice` | Изменение параметров устройств |
| Change Settings | `VirtualMachine.Config.Settings` | Изменение общих параметров виртуальной машины |
| Advanced configuration | `VirtualMachine.Config.AdvancedConfig` | Передача конфигурации `cloud-init` через `guestinfo` |
| Set annotation | `VirtualMachine.Config.Annotation` | Запись заметок к виртуальной машине |
| Rename | `VirtualMachine.Config.Rename` | Переименование виртуальной машины |
| Configure managedBy | `VirtualMachine.Config.ManagedBy` | Пометка виртуальной машины как управляемой платформой |
| Reset guest information | `VirtualMachine.Config.ResetGuestInfo` | Сброс сведений, полученных от гостевой операционной системы |
| Query unowned files | `VirtualMachine.Config.QueryUnownedFiles` | Проверка файлов, не принадлежащих виртуальной машине |
| Reload from path | `VirtualMachine.Config.ReloadFromPath` | Перечитывание конфигурации виртуальной машины из файла |
| Upgrade virtual machine compatibility | `VirtualMachine.Config.UpgradeVirtualHardware` | Повышение версии оборудования виртуальной машины |

### Управление состоянием виртуальных машин

Привилегии нужны для включения и выключения виртуальных машин, подключения устройств, чтения сведений из гостевой операционной системы и работы со снимками. Снимки заказываются, если в кластере включён модуль [`snapshot-controller`](/modules/snapshot-controller/).

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Power On | `VirtualMachine.Interact.PowerOn` | Включение виртуальной машины |
| Power Off | `VirtualMachine.Interact.PowerOff` | Выключение виртуальной машины |
| Reset | `VirtualMachine.Interact.Reset` | Перезагрузка виртуальной машины |
| Answer question | `VirtualMachine.Interact.AnswerQuestion` | Ответ на запросы vSphere, которые блокируют работу машины |
| Device connection | `VirtualMachine.Interact.DeviceConnection` | Подключение и отключение устройств работающей машины |
| Configure CD media | `VirtualMachine.Interact.SetCDMedia` | Подключение образа к приводу CD/DVD |
| Install VMware Tools | `VirtualMachine.Interact.ToolsInstall` | Установка VMware Tools |
| Guest operating system management by VIX API | `VirtualMachine.Interact.GuestControl` | Управление гостевой операционной системой через VIX API |
| Guest Operation Queries | `VirtualMachine.GuestOperations.Query` | Чтение сведений о состоянии гостевой операционной системы |
| Create snapshot | `VirtualMachine.State.CreateSnapshot` | Создание снимка |
| Remove Snapshot | `VirtualMachine.State.RemoveSnapshot` | Удаление снимка |
| Rename Snapshot | `VirtualMachine.State.RenameSnapshot` | Переименование снимка |

### vApp

Операции с vApp и OVF-шаблонами. Требуются, если шаблоны виртуальных машин или сами машины входят в состав vApp.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Create | `VApp.Create` | Создание vApp |
| Delete | `VApp.Delete` | Удаление vApp |
| Import | `VApp.Import` | Импорт OVF или OVA в vApp |
| Add virtual machine | `VApp.AssignVM` | Добавление виртуальной машины в vApp |
| Assign resource pool | `VApp.AssignResourcePool` | Назначение пула ресурсов для vApp |
| Power On | `VApp.PowerOn` | Включение vApp |
| Power Off | `VApp.PowerOff` | Выключение vApp |
| vApp application configuration | `VApp.ApplicationConfig` | Изменение прикладных параметров vApp |
| vApp instance configuration | `VApp.InstanceConfig` | Изменение параметров экземпляра vApp |
| vApp resource configuration | `VApp.ResourceConfig` | Изменение параметров ресурсов vApp |
| View OVF Environment | `VApp.ExtractOvfEnvironment` | Чтение окружения OVF виртуальной машины |
