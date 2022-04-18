---
title: "Cloud provider - VMware vSphere: подготовка окружения"
---

<!-- АВТОР! Не забудь актуализировать getting started если это необходимо -->

## Список необходимых ресурсов vSphere

* **User** — пользователь с необходимым [набором прав](#права).
* **Network** с DHCP и доступом в Интернет.
* **Datacenter** с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
* **ComputeCluster** с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве, с соответствующими [тегами](#datastore-теги).
* **Template** — [подготовленный](#сборка-образа-виртуальных-машин) образ vSphere.

## Конфигурация vSphere

Для конфигурации vSphere необходимо использовать утилиту vSphere CLI [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

### Настройка govc

Для настройки утилиты задайте следующие переменные окружения:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<USER_NAME>
export GOVC_PASSWORD=<USER_PASSWORD>
export GOVC_INSECURE=1
```

### Создание тегов и категорий тегов

В vSphere нет понятия *регион* и *зона*, поэтому для разграничения зон доступности используются теги.

*Регионом* в vSphere является `Datacenter`, а *зоной* — `ComputeCluster`.

Например, если необходимо сделать 2 региона, и в каждом регионе будет 2 зоны доступности, выполните следующие команды:

```shell
govc tags.category.create -d "Kubernetes region" k8s-region
govc tags.category.create -d "Kubernetes zone" k8s-zone
govc tags.create -d "Kubernetes Region X1" -c k8s-region k8s-region-x1
govc tags.create -d "Kubernetes Region X2" -c k8s-region k8s-region-x2
govc tags.create -d "Kubernetes Zone X1-A" -c k8s-zone k8s-zone-x1-a
govc tags.create -d "Kubernetes Zone X1-B" -c k8s-zone k8s-zone-x1-b
govc tags.create -d "Kubernetes Zone X2-A" -c k8s-zone k8s-zone-x2-a
govc tags.create -d "Kubernetes Zone X2-B" -c k8s-zone k8s-zone-x2-b
```

> Созданные категории тегов необходимо указать в `VsphereClusterConfiguration` в `.spec.provider`.

Теги *регионов* навешиваются на Datacenter. Пример:

```shell
govc tags.attach -c k8s-region k8s-region-x1 /X1
```

Теги *зон* навешиваются на Cluster и Datastores. Пример:

```shell
govc tags.attach -c k8s-zone k8s-zone-x1-a /X1/host/x1_cluster_prod
govc tags.attach -c k8s-zone k8s-zone-x1-a /X1/datastore/x1_lun_1
```

#### Datastore теги

При наличии Datastore'ов на **всех** ESXi, где будут размещаться виртуальные машины узлов кластера, возможно использовать динамический заказ PV.
Для автоматического создания StorageClass'ов в Kubernetes кластере, повесьте тег региона и зоны, созданные ранее, на выбранные Datastore'ы.

### Права

> Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Необходимо создать роль (Role) с указанными правами и прикрепить её к одному или нескольким Datacenter'ам, где нужно развернуть кластер Kubernetes.

```shell
govc role.create kubernetes Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement Global.GlobalTag Global.SystemTag InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag Network.Assign Resource.AssignVMToPool Resource.ColdMigrate Resource.HotMigrate Resource.CreatePool Resource.DeletePool Resource.RenamePool Resource.EditPool Resource.MovePool StorageProfile.View System.Anonymous System.Read System.View VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease VirtualMachine.Config.EditDevice VirtualMachine.Config.HostUSBDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory VirtualMachine.Config.MksControl VirtualMachine.Config.QueryFTCompatibility VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement VirtualMachine.Config.ToggleForkParent VirtualMachine.Config.UpgradeVirtualHardware VirtualMachine.GuestOperations.Execute VirtualMachine.GuestOperations.Modify VirtualMachine.GuestOperations.ModifyAliases VirtualMachine.GuestOperations.Query VirtualMachine.GuestOperations.QueryAliases VirtualMachine.Hbr.ConfigureReplication VirtualMachine.Hbr.MonitorReplication VirtualMachine.Hbr.ReplicaManagement VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.Backup VirtualMachine.Interact.ConsoleInteract VirtualMachine.Interact.CreateScreenshot VirtualMachine.Interact.CreateSecondary VirtualMachine.Interact.DefragmentAllDisks VirtualMachine.Interact.DeviceConnection VirtualMachine.Interact.DisableSecondary VirtualMachine.Interact.DnD VirtualMachine.Interact.EnableSecondary VirtualMachine.Interact.GuestControl VirtualMachine.Interact.MakePrimary VirtualMachine.Interact.Pause VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn VirtualMachine.Interact.PutUsbScanCodes VirtualMachine.Interact.Record VirtualMachine.Interact.Replay VirtualMachine.Interact.Reset VirtualMachine.Interact.SESparseMaintenance VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.SetFloppyMedia VirtualMachine.Interact.Suspend VirtualMachine.Interact.TerminateFaultTolerantVM VirtualMachine.Interact.ToolsInstall VirtualMachine.Interact.TurnOffFaultTolerance VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete VirtualMachine.Inventory.Move VirtualMachine.Inventory.Register VirtualMachine.Inventory.Unregister VirtualMachine.Namespace.Event VirtualMachine.Namespace.EventNotify VirtualMachine.Namespace.Management VirtualMachine.Namespace.ModifyContent VirtualMachine.Namespace.Query VirtualMachine.Namespace.ReadContent VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.CloneTemplate VirtualMachine.Provisioning.CreateTemplateFromVM VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate VirtualMachine.Provisioning.DiskRandomAccess VirtualMachine.Provisioning.DiskRandomRead VirtualMachine.Provisioning.FileRandomAccess VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.MarkAsTemplate VirtualMachine.Provisioning.MarkAsVM VirtualMachine.Provisioning.ModifyCustSpecs VirtualMachine.Provisioning.PromoteDisks VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot VirtualMachine.State.RevertToSnapshot

govc permissions.set  -principal имя_пользователя -role kubernetes /datacenter
```

## Инфраструктура

### Сети
Для работы кластера необходим VLAN с DHCP и доступом в Интернет:
* Если VLAN публичный (публичные адреса), то нужна вторая сеть, в которой необходимо развернуть сеть узлов кластера (в этой сети DHCP не нужен).
* Если VLAN внутренний (приватные адреса), то эта же сеть будет сетью узлов кластера.

### Входящий трафик
* Если у вас имеется внутренний балансировщик запросов, то можно обойтись им и направлять трафик напрямую на frontend-узлы кластера.
* Если балансировщика нет, то для организации отказоустойчивых Loadbalancer'ов рекомендуется использовать MetalLB в режиме BGP. В кластере будут созданы frontend-узлы с двумя интерфейсами. Для этого дополнительно потребуются:
  * Отдельный VLAN для обмена трафиком между BGP-роутерами и MetalLB. В этом VLAN'e должен быть DHCP и доступ в Интернет.
  * IP-адреса BGP-роутеров.
  * ASN (номер автономной системы) на BGP-роутере.
  * ASN (номер автономной системы) в кластере.
  * Диапазон, из которого анонсировать адреса.

### Использование хранилища данных
В кластере может одновременно использоваться различное количество типов хранилищ. В минимальной конфигурации потребуются:
* `Datastore`, в котором Kubernetes кластер будет заказывать `PersistentVolume`.
* `Datastore`, в котором будут заказываться root-диски для VM (может быть тот же `Datastore`, что и для `PersistentVolume`).

### Сборка образа виртуальных машин

Чтобы собрать образ виртуальной машины, выполните следующие шаги:

1. [Установите Packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).
1. Склонируйте [репозиторий Deckhouse](https://github.com/deckhouse/deckhouse/):
   ```bash
   git clone https://github.com/deckhouse/deckhouse/
   ```

1. Перейдите в директорию `ee/modules/030-cloud-provider-vsphere/packer/` склонированного репозитория:
   ```bash
   cd deckhouse/ee/modules/030-cloud-provider-vsphere/packer/
   ```

1. Создайте файл `vsphere.auto.pkrvars.hcl` со следующим содержимым:
   ```hcl
   vcenter_server = "<хостнейм или IP vCenter>"
   vcenter_username = "<имя пользователя>"
   vcenter_password = "<пароль>"
   vcenter_cluster = "<имя ComputeCluster, где будет создан образ>"
   vcenter_datacenter = "<имя Datacenter>"
   vcenter_resource_pool = "имя ResourcePool"
   vcenter_datastore = "<имя Datastore, в котором будет создан образ>"
   vcenter_folder = "<имя директории>"
   vm_network = "<имя сети, к которой подключится виртуальная машина при сборке образа>"
   ```
{% raw %}
1. Если ваш компьютер (с которого запущен Packer) не находится в одной сети с `vm_network`, а вы подключены через VPN-туннель, то замените `{{ .HTTPIP }}` в файле `<UbuntuVersion>.pkrvars.hcl` на IP-адрес вашего компьютера из VPN-сети в следующей строке:

    ```hcl
    " url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg",
    ```
{% endraw %}

1. Выберите версию Ubuntu и соберите образ:

   ```shell
   # Ubuntu 20.04
   packer build --var-file=20.04.pkrvars.hcl .
   # Ubuntu 18.04
   packer build --var-file=18.04.pkrvars.hcl .
   ```
