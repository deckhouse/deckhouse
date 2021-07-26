---
title: "Cloud provider - VMware vSphere: подготовка окружения"
---

## Конфигурация vSphere

Для конфигурации vSphere необходимо использовать vSphere CLI [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

### Настройка govc

```shell
export GOVC_URL=n-cs-5.hq.li.corp.kavvas.com
export GOVC_USERNAME=ewwefsadfsda
export GOVC_PASSWORD=weqrfweqfeds
export GOVC_INSECURE=1
```

### Создание тэгов и категорий тэгов

В vSphere нет понятия "регион" и "зона", поэтому для разграничения зон доступности используются тэги.
Например, если необходимо сделать 2 региона, и в каждом регионе будет 2 зоны доступности:

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

Созданные категории тэгов необходимо указать в `VsphereClusterConfiguration` в `.spec.provider`.

Тэги *регионов* навешиваются на Datacenter:

```shell
govc tags.attach -c k8s-region k8s-region-x1 /X1
```

Тэги *зон* навешиваются на Cluster и Datastores:

```shell
govc tags.attach -c k8s-zone k8s-zone-x1-a /X1/host/x1_cluster_prod
govc tags.attach -c k8s-zone k8s-zone-x1-a /X1/datastore/x1_lun_1
```

### Права

Необходимо создать роль (Role) с указанными правами и прикрепить её к одному или нескольким Datacenters, где нужно развернуть Kubernetes кластер.

Упущено создание пользователя, ввиду разнообразия SSO, подключаемых к vSphere.

```shell
govc role.create kubernetes Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement Global.GlobalTag Global.SystemTag InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag Network.Assign Resource.AssignVMToPool Resource.ColdMigrate Resource.HotMigrate Resource.CreatePool Resource.DeletePool Resource.RenamePool Resource.EditPool Resource.MovePool StorageProfile.View System.Anonymous System.Read System.View VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease VirtualMachine.Config.EditDevice VirtualMachine.Config.HostUSBDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory VirtualMachine.Config.MksControl VirtualMachine.Config.QueryFTCompatibility VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement VirtualMachine.Config.ToggleForkParent VirtualMachine.Config.UpgradeVirtualHardware VirtualMachine.GuestOperations.Execute VirtualMachine.GuestOperations.Modify VirtualMachine.GuestOperations.ModifyAliases VirtualMachine.GuestOperations.Query VirtualMachine.GuestOperations.QueryAliases VirtualMachine.Hbr.ConfigureReplication VirtualMachine.Hbr.MonitorReplication VirtualMachine.Hbr.ReplicaManagement VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.Backup VirtualMachine.Interact.ConsoleInteract VirtualMachine.Interact.CreateScreenshot VirtualMachine.Interact.CreateSecondary VirtualMachine.Interact.DefragmentAllDisks VirtualMachine.Interact.DeviceConnection VirtualMachine.Interact.DisableSecondary VirtualMachine.Interact.DnD VirtualMachine.Interact.EnableSecondary VirtualMachine.Interact.GuestControl VirtualMachine.Interact.MakePrimary VirtualMachine.Interact.Pause VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn VirtualMachine.Interact.PutUsbScanCodes VirtualMachine.Interact.Record VirtualMachine.Interact.Replay VirtualMachine.Interact.Reset VirtualMachine.Interact.SESparseMaintenance VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.SetFloppyMedia VirtualMachine.Interact.Suspend VirtualMachine.Interact.TerminateFaultTolerantVM VirtualMachine.Interact.ToolsInstall VirtualMachine.Interact.TurnOffFaultTolerance VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete VirtualMachine.Inventory.Move VirtualMachine.Inventory.Register VirtualMachine.Inventory.Unregister VirtualMachine.Namespace.Event VirtualMachine.Namespace.EventNotify VirtualMachine.Namespace.Management VirtualMachine.Namespace.ModifyContent VirtualMachine.Namespace.Query VirtualMachine.Namespace.ReadContent VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.CloneTemplate VirtualMachine.Provisioning.CreateTemplateFromVM VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate VirtualMachine.Provisioning.DiskRandomAccess VirtualMachine.Provisioning.DiskRandomRead VirtualMachine.Provisioning.FileRandomAccess VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.MarkAsTemplate VirtualMachine.Provisioning.MarkAsVM VirtualMachine.Provisioning.ModifyCustSpecs VirtualMachine.Provisioning.PromoteDisks VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot VirtualMachine.State.RevertToSnapshot

govc permissions.set  -principal имя_пользователя -role kubernetes /datacenter
```

## Инфраструктура

### Сети
Для работы кластера необходим VLAN с DHCP и доступом в Интернет
* Если VLAN публичный (белые адреса), то нужна вторая сеть, в которой необходимо развернуть сеть узлов кластера (в этой сети DHCP не нужен)
* Если VLAN внутренний (серые адреса), то эта же сеть будет сетью узлов кластера

### Входящий трафик
* Если у вас имеется внутренний балансировщик запросов, то можно обойтись им и направлять трафик напрямую на frontend-узлы кластера.
* Если балансировщика нет, то для организации отказоустойчивых Lоadbalancer'ов рекомендуется использовать MetalLB в режиме BGP. В кластере будут созданы frontend-узлы с двумя интерфейсами. Для этого дополнительно потребуются:
  * Отдельный VLAN для обмена трафиком между BGP-роутерами и MetalLB. В этом VLAN'e должен быть DHCP и доступ в Интернет
  * IP адреса BGP-роутеров
  * ASN (номер автономной системы) на BGP-роутере
  * ASN (номер автономной системы) в кластере
  * Диапазон, из которого анонсировать адреса

### Использование хранилища данных
В кластере может одновременно использоваться различное количество типов хранилищ. В минимальной конфигурации потребуются:
* `Datastore`, в котором Kubernetes кластер будет заказывать `PersistentVolume`
* `Datastore`, в котором будут заказываться root-диски для VM (может быть тот же `Datastore`, что и для `PersistentVolume`)
