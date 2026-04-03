{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

## Список необходимых ресурсов vSphere

{% alert %}
Deckhouse использует интерфейс `ens192`, как интерфейс по умолчанию для виртуальных машин в vSphere. Поэтому, при использовании статических IP-адресов в `mainNetwork`, вы должны в образе ОС создать интерфейс с именем `ens192`, как интерфейс по умолчанию.
{% endalert %}

* **User** с необходимым [набором привилегий](#создание-и-назначение-роли).
* **Network** с DHCP и доступом в интернет.
* **Datacenter** с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
* **Cluster** с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве, с соответствующими [тегами](#настройка-datastore).
* **Template** — [подготовленный](#подготовка-образа-виртуальной-машины) образ виртуальной машины.

## Конфигурация vSphere

### Установка govc

Для дальнейшей конфигурации vSphere вам понадобится vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter:

{% snippetcut %}
```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```
{% endsnippetcut %}

### Создание тегов и категорий тегов

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является `Datacenter`, а «зоной» — `Cluster`. Для создания этой связи используются теги.

Создайте категории тегов с помощью команд:

{% snippetcut %}
```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```
{% endsnippetcut %}

Создайте теги в каждой категории. Если вы планируете использовать несколько «зон» (`Cluster`), создайте тег для каждой из них:

{% snippetcut %}
```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```
{% endsnippetcut %}

Назначьте тег «региона» на `Datacenter`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```
{% endsnippetcut %}

Назначьте теги «зон» на объекты `Cluster`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```
{% endsnippetcut %}

#### Настройка Datastore

{% alert level="warning" %}
Для динамического заказа `PersistentVolume` необходимо, чтобы `Datastore` был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания `StorageClass` в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты `Datastore`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```
{% endsnippetcut %}

### Создание и назначение роли

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя привилегии из раздела [«Список необходимых привилегий»](/modules/cloud-provider-vsphere/environment.html#список-необходимых-привилегий). При необходимости более гранулярных прав обратитесь в техподдержку Deckhouse.
{% endalert %}

Создайте роль с необходимыми привилегиями:

{% snippetcut %}
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
{% endsnippetcut %}

Назначьте пользователю роль на объекте `vCenter`:

{% snippetcut %}
```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```
{% endsnippetcut %}

### Подготовка образа виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

Если вам необходимо использовать собственный образ, обратитесь к [документации](/modules/cloud-provider-vsphere/environment.html#требования-к-образу-виртуальной-машины).
