{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

## Список необходимых ресурсов vSphere

* **User** пользователь с необходимым [набором прав](#права).
* **Network** с DHCP и доступом в Интернет.
* **Datacenter** с соответствующим тэгом [`k8s-region`](#создание-тэгов-и-категорий-тэгов).
* **ComputeCluster** с соответствующим тэгом [`k8s-zone`](#создание-тэгов-и-категорий-тэгов).
* **Datastore** в любом количестве, с соответствующими [тэгами](#datastore-тэги).
* **Template** — [подготовленный](#сборка-образа-виртуальных-машин) образ vSphere.

## Конфигурация vSphere

Для конфигурации vSphere необходимо использовать vSphere CLI [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

### Настройка govc

{% snippetcut %}
```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<USER_NAME>
export GOVC_PASSWORD=<USER_PASSWORD>
export GOVC_INSECURE=1
```
{% endsnippetcut %}

### Создание тэгов и категорий тэгов

В vSphere нет понятия "регион" и "зона", поэтому для разграничения зон доступности используются тэги.

"Регионом" в vSphere является `Datacenter`, а "зоной" — `ComputeCluster`.

Например, если необходимо сделать 2 региона, и в каждом регионе будет 1 зона доступности:

{% snippetcut %}
```shell
govc tags.category.create -d "Kubernetes region" k8s-region
govc tags.category.create -d "Kubernetes zone" k8s-zone
govc tags.create -d "Kubernetes Region #1" -c k8s-region test_region_1
govc tags.create -d "Kubernetes Region #2" -c k8s-region test_region_2
govc tags.create -d "Kubernetes Zone Test" -c k8s-zone test_zone
```
{% endsnippetcut %}

Тэги *регионов* навешиваются на Datacenter:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test_region_1 /DC1
govc tags.attach -c k8s-region test_region_2 /DC2
```
{% endsnippetcut %}

Тэги *зон* навешиваются на Cluster и Datastores:

{% snippetcut %}
```shell
govc tags.attach -c k8s-zone test_zone /DC/host/test_cluster
govc tags.attach -c k8s-zone test_zone /DC/datastore/test_lun
```
{% endsnippetcut %}

#### Datastore тэги

При наличии Datastore'ов на **всех** ESXi, где будут размещаться виртуальные машины узлов кластера, возможно использовать динамический заказ PV.
Для автоматического создания StorageClass'ов в Kubernetes кластере, повесьте тэг региона и зоны, созданные ранее, на выбранные Datastore'ы.

### Права

> Ввиду разнообразия подключаемых к vSphere SSO-провайдеров, шаги по созданию пользователя в данной статье не рассматриваются.

Необходимо создать роль (Role) с указанными правами и прикрепить её к **vCenter**, где нужно развернуть кластер
Kubernetes.

{% snippetcut %}
```shell
govc role.create kubernetes \
  Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement Folder.Create Global.GlobalTag Global.SystemTag \
  InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateTag \
  InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory \
  InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag \
  InventoryService.Tagging.ObjectAttachable \
  Network.Assign Resource.AssignVMToPool Resource.ColdMigrate Resource.HotMigrate Resource.CreatePool \
  Resource.DeletePool Resource.RenamePool Resource.EditPool Resource.MovePool StorageProfile.View System.Anonymous System.Read System.View \
  VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice \
  VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount \
  VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease \
  VirtualMachine.Config.EditDevice VirtualMachine.Config.HostUSBDevice VirtualMachine.Config.ManagedBy \
  VirtualMachine.Config.Memory VirtualMachine.Config.MksControl VirtualMachine.Config.QueryFTCompatibility \
  VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath \
  VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo \
  VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement \
  VirtualMachine.Config.ToggleForkParent VirtualMachine.Config.UpgradeVirtualHardware VirtualMachine.GuestOperations.Execute \
  VirtualMachine.GuestOperations.Modify VirtualMachine.GuestOperations.ModifyAliases VirtualMachine.GuestOperations.Query \
  VirtualMachine.GuestOperations.QueryAliases VirtualMachine.Hbr.ConfigureReplication VirtualMachine.Hbr.MonitorReplication \
  VirtualMachine.Hbr.ReplicaManagement VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.Backup \
  VirtualMachine.Interact.ConsoleInteract VirtualMachine.Interact.CreateScreenshot VirtualMachine.Interact.CreateSecondary \
  VirtualMachine.Interact.DefragmentAllDisks VirtualMachine.Interact.DeviceConnection VirtualMachine.Interact.DisableSecondary \
  VirtualMachine.Interact.DnD VirtualMachine.Interact.EnableSecondary VirtualMachine.Interact.GuestControl \
  VirtualMachine.Interact.MakePrimary VirtualMachine.Interact.Pause VirtualMachine.Interact.PowerOff \
  VirtualMachine.Interact.PowerOn VirtualMachine.Interact.PutUsbScanCodes VirtualMachine.Interact.Record \
  VirtualMachine.Interact.Replay VirtualMachine.Interact.Reset VirtualMachine.Interact.SESparseMaintenance \
  VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.SetFloppyMedia VirtualMachine.Interact.Suspend \
  VirtualMachine.Interact.TerminateFaultTolerantVM VirtualMachine.Interact.ToolsInstall \
  VirtualMachine.Interact.TurnOffFaultTolerance VirtualMachine.Inventory.Create \
  VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete \
  VirtualMachine.Inventory.Move VirtualMachine.Inventory.Register VirtualMachine.Inventory.Unregister \
  VirtualMachine.Namespace.Event VirtualMachine.Namespace.EventNotify VirtualMachine.Namespace.Management \
  VirtualMachine.Namespace.ModifyContent VirtualMachine.Namespace.Query VirtualMachine.Namespace.ReadContent \
  VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.CloneTemplate VirtualMachine.Provisioning.CreateTemplateFromVM \
  VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate VirtualMachine.Provisioning.DiskRandomAccess \
  VirtualMachine.Provisioning.DiskRandomRead VirtualMachine.Provisioning.FileRandomAccess VirtualMachine.Provisioning.GetVmFiles \
  VirtualMachine.Provisioning.MarkAsTemplate VirtualMachine.Provisioning.MarkAsVM VirtualMachine.Provisioning.ModifyCustSpecs \
  VirtualMachine.Provisioning.PromoteDisks VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs \
  VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot VirtualMachine.State.RevertToSnapshot \
  Cns.Searchable StorageProfile.View

govc permissions.set  -principal имя_пользователя -role kubernetes /DC
```
{% endsnippetcut %}

### Сборка образа виртуальных машин

1. [Установить Packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).
1. Склонировать [репозиторий Deckhouse](https://github.com/deckhouse/deckhouse/):
   {% snippetcut %}
```bash
git clone https://github.com/deckhouse/deckhouse/
```
   {% endsnippetcut %}

1. Перейти в директорию `ee/modules/030-cloud-provider-vsphere/packer/` склонированного репозитория:
   {% snippetcut %}
```bash
cd deckhouse/ee/modules/030-cloud-provider-vsphere/packer/
```
   {% endsnippetcut %}

1. Создать файл `vsphere.auto.pkrvars.hcl` со следующим содержимым:
   {% snippetcut %}
```hcl
vcenter_server = "<hostname или IP vCenter>"
vcenter_username = "<имя пользователя>"
vcenter_password = "<пароль>"
vcenter_cluster = "<имя ComputeCluster, где будет создан образ>"
vcenter_datacenter = "<имя Datacenter>"
vcenter_resource_pool = "имя ResourcePool"
vcenter_datastore = "<имя Datastore, в котором будет создан образ>"
vcenter_folder = "<имя директории>"
vm_network = "<имя сети, к которой подключится виртуальная машина при сборке образа>"
```
   {% endsnippetcut %}
{% raw %}
1. Если ваш компьютер (с которого запущен Packer) не находится в одной сети с `vm_network`, а вы подключены через VPN-туннель, то замените `{{ .HTTPIP }}` в файле `<UbuntuVersion>.pkrvars.hcl` на IP-адрес вашего компьютера из VPN-сети в следующей строке:

    ```hcl
    " url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg",
    ```
{% endraw %}

1. Выберите версию Ubuntu и соберите образ:

   {% snippetcut %}
```shell
# Ubuntu 20.04
packer build --var-file=20.04.pkrvars.hcl .
# Ubuntu 18.04
packer build --var-file=18.04.pkrvars.hcl .
```
   {% endsnippetcut %}
