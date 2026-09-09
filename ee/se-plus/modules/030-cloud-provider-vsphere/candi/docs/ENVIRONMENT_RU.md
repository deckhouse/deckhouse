---
title: "Cloud provider — VMware vSphere: подготовка окружения"
description: "Настройка VMware vSphere для работы облачного провайдера Deckhouse."
---

<!-- АВТОР! Не забудь актуализировать getting started, если это необходимо -->

## Требования к окружению

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
     - Создавать директорию заранее не нужно, установщик создаёт её по пути из параметра [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath).
     - Если директория уже существует, задайте [`vmFolderExists: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists), иначе установка завершится ошибкой `The name '<FOLDER>' already exists`.
     - В одной директории нельзя разместить более одного кластера.
  1. Role:
     - Роль должна содержать необходимый [набор привилегий](/modules/cloud-provider-vsphere/environment.html#список-необходимых-привилегий).
  1. User:
     - Пользователю должна быть назначена роль, указанная в предыдущем пункте.
- На созданный Datacenter должен быть назначен тег из категории, указанной в параметре [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) (по умолчанию — `k8s-region`). Этот тег определяет регион.

## Список необходимых привилегий

Роль для учётной записи платформы включает перечисленные ниже привилегии. Они сгруппированы по операциям, которые платформа выполняет во vSphere.

Как создать роль и назначить её пользователю, описано в разделах [«Создание и назначение роли с использованием vSphere Client»](#создание-и-назначение-роли-с-использованием-vsphere-client) и [«Создание и назначение роли с использованием govc»](#создание-и-назначение-роли-с-использованием-govc).

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
| Create folder | `Folder.Create` | Создание директории по пути из параметра [`vmFolderPath`](cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) |
| Delete folder | `Folder.Delete` | Удаление этой директории вместе с кластером |
| Move folder | `Folder.Move` | Перемещение директории при изменении пути |
| Rename folder | `Folder.Rename` | Переименование директории при изменении пути |
| Assign virtual machine to resource pool | `Resource.AssignVMToPool` | Размещение виртуальных машин в пуле ресурсов |
| Create resource pool | `Resource.CreatePool` | Создание вложенного пула ресурсов в каждой зоне |
| Modify resource pool | `Resource.EditPool` | Изменение параметров этого пула |
| Remove resource pool | `Resource.DeletePool` | Удаление пула вместе с кластером |
| Rename resource pool | `Resource.RenamePool` | Переименование пула |
| Assign network | `Network.Assign` | Подключение виртуальных машин к сетям из параметров [`mainNetwork`](cr.html#vsphereinstanceclass-v1-spec-mainnetwork) и [`additionalNetworks`](cr.html#vsphereinstanceclass-v1-spec-additionalnetworks) |

### Создание виртуальных машин

Виртуальные машины создаются клонированием подготовленного шаблона и регистрируются в инвентаре vSphere.

| Привилегия в UI | Привилегия в API | Назначение в DKP |
| --- | --- | --- |
| Clone virtual machine | `VirtualMachine.Provisioning.Clone` | Клонирование шаблона из параметра [`template`](cr.html#vsphereinstanceclass-v1-spec-template) |
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
| Extend virtual disk | `VirtualMachine.Config.DiskExtend` | Увеличение диска до размера из параметра [`rootDiskSize`](cr.html#vsphereinstanceclass-v1-spec-rootdisksize) и расширение томов |
| Acquire disk lease | `VirtualMachine.Config.DiskLease` | Получение блокировки диска на время операций с ним |
| Toggle disk change tracking | `VirtualMachine.Config.ChangeTracking` | Управление отслеживанием изменённых блоков диска |
| Configure Raw device | `VirtualMachine.Config.RawDevice` | Настройка проброшенных устройств (RDM) |
| Change CPU count | `VirtualMachine.Config.CPUCount` | Задание числа vCPU из параметра [`numCPUs`](cr.html#vsphereinstanceclass-v1-spec-numcpus) |
| Change Memory | `VirtualMachine.Config.Memory` | Задание объёма памяти из параметра [`memory`](cr.html#vsphereinstanceclass-v1-spec-memory) |
| Change resource | `VirtualMachine.Config.Resource` | Резервирование памяти из параметра [`memoryReservation`](cr.html#vsphereinstanceclass-v1-spec-runtimeoptions-memoryreservation) и ограничение ресурсов |
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

## Конфигурация vSphere

### Настройка через vSphere Client

#### Создание тегов и категорий тегов с использованием vSphere Client

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является Datacenter, а «зоной» — Cluster. Для создания этой связи используются теги.

1. Откройте vSphere Client и перейдите в «Menu» → «Tags & Custom Attributes» → «Tags».

   ![Создание тегов и категорий тегов, шаг 1](images/tags-categories-setup/menu-tags-and-custom-attributes.png)

1. Откройте вкладку «Categories» и нажмите «NEW». Создайте категорию для регионов (например `k8s-region`): установите значение «One tag» для параметра «Tags Per Object» и задайте связываемые типы, включая Datacenter.

   ![Создание тегов и категорий тегов, шаг 2](images/tags-categories-setup/create-category-region.png)

1. Создайте вторую категорию для зон (например, `k8s-zone`) с типами объектов Host, Cluster и Datastore.

   ![Создание тегов и категорий тегов, шаг 3](images/tags-categories-setup/create-category-zone.png)

1. Перейдите на вкладку «Tags» и создайте минимум по одному тегу в категории региона и в категории зон (например, `test-region`, `test-zone-1`).

   ![Создание тегов и категорий тегов, шаг 4](images/tags-categories-setup/tags-list.png)

1. Во вкладке «Inventory» выберите целевой Datacenter, перейдите в панель «Summary», откройте «Actions» → «Tags & Custom Attributes» → «Assign Tag» и назначьте тег региона.
   Повторите для каждого Cluster, на котором будут узлы, назначая соответствующие теги зон.

   ![Создание тегов и категорий тегов, шаг 5.1](images/tags-categories-setup/datacenter-actions-assign-tag.png)
   ![Создание тегов и категорий тегов, шаг 5.2](images/tags-categories-setup/assign-tag-to-datacenter.png)

#### Настройка Datastore с использованием vSphere Client

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi в зоне (shared datastore).
{% endalert %}

Во вкладке «Inventory» выберите Datastore, перейдите в панель «Summary», затем откройте меню «Actions» → «Tags & Custom Attributes» → «Assign Tag». Назначьте Datastore тот же тег региона, что и у соответствующего Datacenter, а также тот же тег зоны, что и у соответствующего Cluster.

![Создание тегов и категорий тегов, шаг 6](images/tags-categories-setup/assign-tags-to-datastore.png)

Чтобы убедиться, что Datastore подключён ко всем хостам ESXi зоны, откройте «Menu» → «Inventory» → «Storage», выберите Datastore и перейдите на вкладку «Hosts». В списке приводятся хосты, которым Datastore доступен.

![Проверка доступности Datastore](images/datastore-setup/datastore-hosts.png)

#### Создание директории для виртуальных машин с использованием vSphere Client

Установщик создаёт директорию для виртуальных машин по пути из параметра [`vmFolderPath`](cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath). Если директория уже существует, задайте [`vmFolderExists: true`](cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists).

Чтобы создать директорию заранее, откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите в списке объект Datacenter, затем откройте «ACTIONS» → «New Folder» → «New VM and Template Folder...» и задайте имя.

![Создание директории для виртуальных машин](images/vm-folder-setup/new-vm-and-template-folder.png)

#### Создание пула ресурсов с использованием vSphere Client

В каждой зоне установщик создаёт пул ресурсов с именем из параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) ресурса ClusterConfiguration. Если задан параметр [`baseResourcePool`](cluster_configuration.html#vsphereclusterconfiguration-baseresourcepool), пул создаётся внутри указанного в нём пула. Родительский пул должен существовать, установщик его не создаёт. То же относится к пулу из параметра [`resourcePool`](cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-resourcepool) в описании группы узлов.

Чтобы создать пул, откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите в списке объект Cluster, затем откройте «ACTIONS» → «New Resource Pool...» и задайте имя.

![Создание пула ресурсов](images/resource-pool-setup/new-resource-pool.png)

Со значением [`useNestedResourcePool: false`](cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool) установщик не создаёт вложенный пул. Виртуальные машины размещаются в пуле из параметра `resourcePool`. Если этот параметр не задан, машины размещаются в корневом пуле Cluster.

#### Создание и назначение роли с использованием vSphere Client

1. Откройте «Menu» → «Administration». Откроется отдельная страница управления vCenter, на которой в левой панели в разделе «Access Control» находится пункт «Roles».

   ![Создание и назначение роли, шаг 1](images/role-setup/access-control-roles.png)

1. Нажмите «NEW», введите имя роли (например, `deckhouse`) и добавьте привилегии из [списка](#список-необходимых-привилегий).

   ![Создание и назначение роли, шаг 2](images/role-setup/new-role-privileges.png)

1. Назначьте роль для учётной записи Deckhouse, во вкладке «Menu» → «Administration» → «Access Control» → «Global Permissions» нажмите «ADD» и выберите пользователя и роль `deckhouse`.

   ![Создание и назначение роли, шаг 3](images/role-setup/add-global-permission.png)

### Настройка через govc

#### Установка govc

Для дальнейшей настройки vSphere потребуется CLI-утилита [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter.

{% alert level="warning" %}
Обязательно указывайте имя пользователя вместе с доменом, например: `username@vsphere.local`.
{% endalert %}

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<USERNAME>@vsphere.local
export GOVC_PASSWORD=<PASSWORD>
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
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>
```

Назначьте теги «зон» на объекты Cluster:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/host/<CLUSTER_NAME_1>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/host/<CLUSTER_NAME_2>
```

#### Настройка Datastore с использованием govc

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания StorageClass в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты Datastore:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>

govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
```

#### Создание и назначение роли с использованием govc

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя привилегии из раздела [«Список необходимых привилегий»](#список-необходимых-привилегий). При необходимости более гранулярных прав обратитесь в техподдержку Deckhouse.
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
govc permissions.set -principal <USERNAME>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
Описание привилегий vSphere приведено в [документации VMware](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/defined-privileges.html).
{% endalert %}

#### Область назначения роли

Роль назначается на корневом объекте vCenter, а не на директории с виртуальными машинами кластера. Компоненты платформы обращаются к объектам за пределами этой директории:

- CSI-драйвер определяет топологию томов по хостам ESXi, связанным с Datastore, поэтому обращается к объектам Cluster и Host;
- компонент обнаружения ресурсов ищет диски CNS во всём vCenter, и для этого ему нужна привилегия `Cns.Searchable` из группы [«Хранилище»](#хранилище);
- установщик создаёт пул ресурсов в объекте Cluster и директорию в Datacenter, для чего нужны привилегии из группы [«Размещение виртуальных машин»](#размещение-виртуальных-машин).

Если ограничить роль директорией виртуальных машин, эти операции завершатся ошибкой.

#### Проверка привилегий

CSI-драйвер проверяет привилегии учётной записи на каждом Datastore и исключает те, на которых привилегий недостаточно. Такой Datastore не попадает в список доступных, и заказ PersistentVolume через соответствующий StorageClass не проходит.

Если для размеченного тегами Datastore не создаётся рабочий StorageClass, проверьте привилегии учётной записи на этом объекте:

```shell
govc permissions.ls /<DATACENTER_NAME>/datastore/<DATASTORE_NAME>
```

Привилегии учётной записи на объекте можно посмотреть и в vSphere Client. Выберите объект, откройте вкладку «Permissions» и найдите учётную запись в списке. Колонка «Defined In» показывает объект, на котором задано разрешение. Значение «Global Permission» означает, что разрешение назначено глобально.

![Просмотр разрешений на объекте](images/permissions-check/datastore-permissions.png)

### Проверка TLS-сертификата vCenter

DKP подключается к vCenter по TLS и проверяет его сертификат. Если сертификат vCenter выпущен собственным или корпоративным центром сертификации, передайте цепочку сертификатов этого центра в параметре [`caBundle`](cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle). Проверка сертификата при этом остаётся включённой.

Цепочку укажите в формате PEM. Настройка одна и та же, но путь к ней зависит от того, где описано подключение к vCenter:

- при установке кластера подключение описано в секции [`provider`](cluster_configuration.html#vsphereclusterconfiguration-provider) ресурса [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) рядом с параметром [`provider.server`](cluster_configuration.html#vsphereclusterconfiguration-provider-server), поэтому цепочка задаётся в параметре [`provider.caBundle`](cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle);
- в уже работающем кластере подключение описано на верхнем уровне настроек модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) рядом с параметром [`host`](configuration.html#parameters-host), поэтому цепочка задаётся в параметре [`caBundle`](configuration.html#parameters-cabundle).

Пример для устанавливаемого кластера:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  caBundle: |
    -----BEGIN CERTIFICATE-----
    <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
    -----END CERTIFICATE-----
```

Пример для работающего кластера:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    host: "<VCENTER_FQDN>"
    username: "<USERNAME@DOMAIN.LOCAL>"
    password: "<PASSWORD>"
    caBundle: |
      -----BEGIN CERTIFICATE-----
      <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
      -----END CERTIFICATE-----
```

Параметр [`insecure: true`](cluster_configuration.html#vsphereclusterconfiguration-provider-insecure) полностью отключает проверку сертификата vCenter. В настройках работающего кластера тот же параметр называется [`insecure`](configuration.html#parameters-insecure). Задайте либо `caBundle`, либо `insecure: true`. Если заданы оба параметра, DKP отклонит конфигурацию.

Для подключения к NSX-T цепочка сертификатов задаётся отдельным параметром [`nsxt.caBundle`](cluster_configuration.html#vsphereclusterconfiguration-nsxt-cabundle). В настройках работающего кластера это параметры [`nsxt.caBundle`](configuration.html#parameters-nsxt-cabundle) и [`nsxt.insecureFlag`](configuration.html#parameters-nsxt-insecureflag). Задайте либо `nsxt.caBundle`, либо [`nsxt.insecureFlag: true`](cluster_configuration.html#vsphereclusterconfiguration-nsxt-insecureflag), иначе DKP отклонит конфигурацию.

{% alert level="warning" %}
Модуль [`csi-vsphere`](/modules/csi-vsphere/) не поддерживает параметр `caBundle` и подключается к vCenter только с проверкой сертификата по системным центрам сертификации либо с параметром [`insecure`](/modules/csi-vsphere/configuration.html#parameters-insecure).
{% endalert %}

### Требования к образу виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

- [**Ubuntu**](https://cloud-images.ubuntu.com/)
- [**Debian**](https://cloud.debian.org/images/cloud/)
- [**CentOS**](https://cloud.centos.org/)
- [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

#### Подготовка образа виртуальной машины

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

#### Подготовка шаблона в vSphere Client

После подготовки образа выполните в vSphere Client следующие действия:

1. Импортируйте образ операционной системы. Выберите Datacenter или Cluster, откройте «ACTIONS» → «Deploy OVF Template...» и укажите OVA-файл вендора.

   ![Подготовка шаблона, шаг 1](images/vm-template-setup/deploy-ovf-template.png)

1. Проверьте параметры полученной виртуальной машины. Строка «Compatibility» в панели «VM Hardware» на вкладке «Summary» содержит версию оборудования. Там же перечислены подключённые устройства. Требуются версия 15 или выше и один диск.

   ![Подготовка шаблона, шаг 2](images/vm-template-setup/vm-hardware-compatibility.png)

1. Если версия оборудования ниже 15, повысьте её. Выключите виртуальную машину и откройте «ACTIONS» → «Compatibility» → «Upgrade VM Compatibility...». Для включённой машины этот пункт недоступен. Запланировать повышение версии на следующее выключение можно пунктом «Schedule VM Compatibility Upgrade...».

   ![Подготовка шаблона, шаг 3](images/vm-template-setup/upgrade-vm-compatibility.png)

1. Преобразуйте виртуальную машину в шаблон. Откройте «ACTIONS» → «Template» → «Convert to Template». В параметре `template` можно указать как шаблон, так и выключенную виртуальную машину.

   ![Подготовка шаблона, шаг 4](images/vm-template-setup/convert-to-template.png)

## Инфраструктура

### Подключение к vCenter

Источник настроек подключения зависит от того, где размещён control plane кластера. Если control plane работает в облаке, адрес vCenter и учётные данные задаются в секции [`provider`](cluster_configuration.html#vsphereclusterconfiguration-provider) ресурса VsphereClusterConfiguration. Если control plane работает на виртуальных машинах или bare metal, те же данные задаются параметрами модуля [`host`](configuration.html#parameters-host), [`username`](configuration.html#parameters-username) и [`password`](configuration.html#parameters-password).

Платформа подключается к vCenter по TLS и проверяет сертификат. Как передать цепочку сертификатов центра сертификации, описано в разделе [«Проверка TLS-сертификата vCenter»](#проверка-tls-сертификата-vcenter).

### Сети

Узлы кластера подключаются к сети, путь к которой задаётся параметром [`mainNetwork`](cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetwork). Через эту сеть узлы обращаются к vCenter, к хранилищу образов контейнеров и друг к другу.

Сеть должна отвечать следующим требованиям:

- она доступна на всех хостах ESXi, на которых создаются виртуальные машины;
- в ней работает DHCP, если адреса узлов не заданы статически;
- из неё доступны vCenter и хранилище образов контейнеров.

#### Адресация узлов

По умолчанию узлы получают адреса по DHCP. Для узлов, которые создаёт установщик, адреса задаются статически в параметре [`mainNetworkIPAddresses`](cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetworkipaddresses). Для узлов, которые заказываются по ресурсу [VsphereInstanceClass](cr.html#vsphereinstanceclass), статическая адресация не поддерживается, такие узлы получают адреса по DHCP.

Адреса назначаются узлам группы по порядку, поэтому задайте их столько же, сколько узлов в группе. Если адресов меньше, список используется повторно и один адрес достаётся нескольким узлам.

Пример статической адресации для трёх master-узлов:

```yaml
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: k8s-msk-178
    mainNetworkIPAddresses:
    - address: 10.1.14.20/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.21/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.22/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
```

#### Несколько сетей

Дополнительные интерфейсы виртуальных машин подключаются к сетям из параметра [`additionalNetworks`](cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-additionalnetworks). Например, через дополнительный интерфейс подключают сеть для трафика BGP.

Когда сетей несколько, укажите, какие из них платформа считает внутренними, а какие внешними. По именам сетей из параметров [`internalNetworkNames`](cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) и [`externalNetworkNames`](cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) компонент `vsphere-cloud-controller-manager` проставляет адреса InternalIP и ExternalIP в объекте Node. В этих параметрах указывается имя сети, а не путь к ней.

Параметр [`internalNetworkCIDR`](cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) задаёт подсеть, из которой master-узлы получают адреса во внутренней сети. Адреса выделяются начиная с десятого. Параметр обязателен, если в конфигурации есть секция [`nodeGroups`](cluster_configuration.html#vsphereclusterconfiguration-nodegroups).

Пример конфигурации с двумя сетями:

```yaml
externalNetworkNames:
- net3-k8s
internalNetworkNames:
- K8S_3
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: net3-k8s
    additionalNetworks:
    - K8S_3
```

### Входящий трафик

Входящий трафик балансируется одним из трёх способов.

1. **Через внешний балансировщик.** Балансировщик, который уже есть в инфраструктуре, направляет трафик на frontend-узлы кластера. Со стороны vSphere требуется только сеть, в которой frontend-узлы доступны балансировщику. Трафик в кластере принимает Ingress-контроллер модуля [`ingress-nginx`](/modules/ingress-nginx/).

1. **Через MetalLB.** Способ подходит, когда внешнего балансировщика нет. MetalLB выдаёт адреса сервисам с типом LoadBalancer и работает в режимах L2 и BGP. Требования к сети, доступность режимов в редакциях и настройка описаны в [документации модуля `metallb`](/modules/metallb/). Для режима BGP со стороны vSphere потребуются:

   - отдельная сеть, в которой frontend-узлы обмениваются трафиком с BGP-роутерами;
   - второй сетевой интерфейс у frontend-узлов, подключённый к этой сети параметром [`additionalNetworks`](cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-additionalnetworks);
   - DHCP в этой сети. Адрес на дополнительном интерфейсе платформа задаёт только master-узлам, из подсети [`internalNetworkCIDR`](cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) начиная с десятого адреса. Узлам групп адрес в этой сети платформа не назначает;
   - IP-адреса BGP-роутеров, номер автономной системы (ASN) роутеров и номер ASN кластера, а также диапазон адресов, который кластер анонсирует.

1. **Через NSX-T.** Если в инфраструктуре развёрнут NSX-T, `cloud-controller-manager` заказывает в нём балансировщик для каждого сервиса с типом LoadBalancer. Имя пула адресов, путь к шлюзу Tier-1 и учётные данные задаются в секции [`nsxt`](configuration.html#parameters-nsxt).

### Использование хранилища данных

Кластер использует Datastore для двух задач:

- размещение root-дисков виртуальных машин;
- размещение томов PersistentVolume.

Один Datastore может обслуживать обе задачи. При планировании свободного места учитывайте и диски узлов, и тома.

Datastore должен отвечать следующим требованиям:

- подключён ко всем хостам ESXi зоны;
- размечен тегами региона и зоны;
- доступен учётной записи платформы. CSI-драйвер проверяет привилегии на каждом Datastore и исключает те, где привилегий недостаточно.

#### Диски узлов

Datastore для root-дисков задаётся параметром [`datastore`](cr.html#vsphereinstanceclass-v1-spec-datastore) группы узлов, путь указывается относительно Datacenter. Размер диска задаётся параметром [`rootDiskSize`](cr.html#vsphereinstanceclass-v1-spec-rootdisksize). Для узлов, которые создаёт установщик, размер по умолчанию составляет 50 ГиБ. Для узлов по ресурсу [VsphereInstanceClass](cr.html#vsphereinstanceclass) он составляет 20 ГиБ. Значение меньше размера диска в шаблоне приводит к ошибке клонирования.

Пример группы узлов с отдельным Datastore и увеличенным root-диском:

```yaml
nodeGroups:
- name: worker
  replicas: 2
  zones:
  - test-zone-1
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    mainNetwork: k8s-msk-178
    datastore: lun_1
    rootDiskSize: 50
```

Политику хранения SPBM для дисков узлов задаёт параметр [`storagePolicyID`](cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid). В параметре указывается идентификатор политики.

Идентификатор можно посмотреть в vSphere Client. Откройте «Menu» → «Policies and Profiles» → «VM Storage Policies» и выберите политику по имени. В адресной строке браузера найдите параметр `lvSelectedItemId`. Идентификатор расположен между `PbmRequirementStorageProfile:` и первым символом `%`.

![Список политик хранения](images/storage-policy-setup/vm-storage-policies.png)

Идентификатор политики также выводит команда `govc`:

```shell
govc storage.policy.ls "<POLICY_NAME>"
```

Замените `<POLICY_NAME>` на имя политики, как оно показано в списке «VM Storage Policies», например `vSAN Default Storage Policy`.

#### Тома PersistentVolume

Для каждого Datastore, размеченного тегом зоны, платформа создаёт StorageClass. Если во vSphere настроены политики хранения SPBM, дополнительно создаётся StorageClass для каждого сочетания Datastore и политики. Для DatastoreCluster StorageClass создаются только в legacy-режиме, который включается параметром [`compatibilityFlag`](configuration.html#parameters-storageclass-compatibilityflag).

Имя StorageClass с политикой складывается из имени Datastore и имени политики. Платформа приводит его к нижнему регистру, заменяет пробелы на дефисы и удаляет остальные символы, кроме дефиса и точки. Например, для Datastore `lun_1` и политики `Gold Policy` получается StorageClass `lun1-gold-policy`.

Чтобы платформа обнаружила политики, учётной записи vSphere нужна привилегия `StorageProfile.View` из [списка необходимых привилегий](#список-необходимых-привилегий). Набор StorageClass формируется автоматически, поэтому политика для отдельного StorageClass вручную не задаётся.

Чтобы для части Datastore StorageClass не создавались, перечислите их в параметре [`exclude`](configuration.html#parameters-storageclass-exclude):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    storageClass:
      exclude:
      - ".*-lun101-.*"
      - slow-lun103
```

Параметр принимает имена и регулярные выражения, каждое из которых должно совпадать с именем Datastore целиком. Частичное совпадение не учитывается. Например, для Datastore `vsanDatastore` выражение `vsan` не исключает ни одного StorageClass, а `vsan.*` исключает все. Исключение убирает и базовый StorageClass, и StorageClass с политиками для этого Datastore. Отдельный StorageClass с политикой по его собственному имени параметр не исключает.

Чтобы задать StorageClass по умолчанию, используйте глобальный параметр [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass).

#### Режим работы CSI

По умолчанию подсистема хранения использует диски CNS с возможностью изменения размера без отключения тома от узла (online resize). Также поддерживается работа в legacy-режиме с дисками FCD, в котором изменение размера без отключения тома недоступно. Режим выбирается параметром [`compatibilityFlag`](configuration.html#parameters-storageclass-compatibilityflag).

#### Увеличение размера PersistentVolumeClaim

Чтобы увеличить том, измените запрошенный размер в PersistentVolumeClaim:

```shell
d8 k -n <NAMESPACE> patch pvc <PVC_NAME> -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
```

Дополнительные действия не требуются. Платформа расширяет том в vSphere, затем kubelet расширяет файловую систему на узле, к которому том подключён. Рабочая нагрузка при этом не перезапускается.

Пока расширение выполняется, в статусе PersistentVolumeClaim присутствуют condition `Resizing` и `FileSystemResizePending`. После того как kubelet расширит файловую систему, оба condition удаляются, а поле `status.capacity` содержит новый размер. Команда ниже выводит текущий размер тома и список condition:

```shell
d8 k -n <NAMESPACE> get pvc <PVC_NAME> \
  -o jsonpath='{.status.capacity.storage}{"\n"}{range .status.conditions[*]}{.type}={.status} {end}'
```

Пример вывода для тома, расширение которого завершилось. В первой строке размер, вторая строка пустая, потому что condition в статусе не осталось:

```console
2Gi

```

Если condition `Resizing` остаётся в статусе, расширение без отключения тома в этой конфигурации недоступно, например в legacy-режиме с дисками FCD. Ограничение описано в [issue проекта external-resizer](https://github.com/kubernetes-csi/external-resizer/issues/44). Чтобы расширить такой том, отключите его от узла:

1. Запретите планирование новых нагрузок на узел, к которому подключён том:

   ```shell
   d8 k cordon <NODE_NAME>
   ```

   Замените `<NODE_NAME>` на имя узла, где работает рабочая нагрузка, использующая PersistentVolumeClaim.

1. Удалите рабочую нагрузку, использующую PersistentVolumeClaim, чтобы том отключился от узла.

1. Дождитесь удаления condition `Resizing` из статуса PersistentVolumeClaim.

1. Разрешите планирование на узел:

   ```shell
   d8 k uncordon <NODE_NAME>
   ```

#### Просмотр томов в vSphere Client

Тома, заказанные через CSI, отображаются в vSphere Client. Откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите объект Cluster, перейдите на вкладку «Monitor» и в разделе «Cloud Native Storage» выберите «Container Volumes». Для каждого тома приводятся имя, лейблы, Datastore, соответствие политике хранения («Compliance Status»), доступность («Health Status») и размер.

![Список CNS-томов](images/cns-volumes/container-volumes.png)

Имя тома в vSphere совпадает с именем PersistentVolume в кластере.

По значку слева от имени тома открывается панель с подробностями. На вкладке «Kubernetes objects» приводятся неймспейс, имя и лейблы PersistentVolumeClaim, а также рабочая нагрузка, которая использует том.

![Подробности CNS-тома](images/cns-volumes/container-volume-details.png)
