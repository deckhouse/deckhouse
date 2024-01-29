---
title: "Cloud provider — VMware vSphere: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развернутых в vSphere.

Если control plane кластера размещен на виртуальных машинах или bare-metal-серверах, cloud-провайдер использует настройки модуля `cloud-provider-vsphere` в конфигурации Deckhouse (см. ниже). Иначе, если control plane кластера размещен в облаке, cloud-провайдер использует структуру [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) для настройки.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) модуля `node-manager`, в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference` NodeGroup). Инстанс-класс для cloud-провайдера vSphere — это custom resource [`VsphereInstanceClass`](cr.html#vsphereinstanceclass), в котором указываются конкретные параметры самих машин.

{% include module-settings.liquid %}

## Storage

Модуль автоматически создает StorageClass для каждого Datastore и DatastoreCluster из зон (зоны).

Также он позволяет настроить имя StorageClass'а, который будет использоваться в кластере по умолчанию (параметр [default](#parameters-storageclass-default)) и отфильтровать ненужные StorageClass'ы (параметр [exclude](#parameters-storageclass-exclude)).

### CSI

Подсистема хранения по умолчанию использует CNS-диски с возможностью изменения их размера на лету. Но также поддерживается работа и в legacy-режиме с использованием FCD-дисков. Поведение настраивается параметром [compatibilityFlag](#parameters-storageclass-compatibilityflag).

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer CSI и vSphere API после увеличения размера PVC нужно сделать следующее:

1. На узле, где находится под, выполнить команду `kubectl cordon <имя_узла>`.
2. Удалить под.
3. Убедиться, что изменение размера прошло успешно. В объекте PVC *не будет* condition `Resizing`.
   > Состояние `FileSystemResizePending` не является проблемой.
4. На узле, где находится под, выполнить команду `kubectl uncordon <имя_узла>`.

## Требования к окружениям

* Требования к версии vSphere: `v7.0U2` ([необходимо](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) для работы механизма `Online volume expansion`).
* vCenter, до которого есть доступ изнутри кластера с master-узлов.
* Создать Datacenter, в котором создать:
  1. VirtualMachine template [со специальным](https://github.com/vmware/cloud-init-vmware-guestinfo) cloud-init datasource внутри.
     * Образ виртуальной машины должен использовать `Virtual machines with hardware version 15 or later` (необходимо для работы online resize).
  2. Network, доступную на всех ESXi, на которых будут создаваться виртуальные машины.
  3. Datastore (или несколько), подключенный ко всем ESXi, на которых будут создаваться виртуальные машины.
     * На Datastore'ы **необходимо** «повесить» тег из категории тегов, указанных в [zoneTagCategory](#parameters-zonetagcategory) (по умолчанию `k8s-zone`). Этот тег будет обозначать **зону**. Все Cluster'ы из конкретной зоны должны иметь доступ ко всем Datastore'ам с идентичной зоной.
  4. Cluster, в который добавить необходимые используемые ESXi.
     * На Cluster **необходимо** «повесить» тег из категории тегов, указанных в [zoneTagCategory](#parameters-zonetagcategory) (по умолчанию `k8s-zone`). Этот тег будет обозначать **зону**.
  5. Folder для создаваемых виртуальных машин.
     * Опциональный. По умолчанию будет использоваться root vm-каталог.
  6. Роль с необходимым [набором](#список-привилегий-для-использования-модуля) прав.
  7. Пользователя, привязав к нему роль из п. 6.
* На созданный Datacenter **необходимо** «повесить» тег из категории тегов, указанный в [regionTagCategory](#parameters-regiontagcategory) (по умолчанию `k8s-region`). Этот тег будет обозначать **регион**.

## Список привилегий для использования модуля

Список, приведенный ниже, эквивалентен списку привилегий, назначаемых при настройке окружения vSphere для работы с Deckhouse Kubernetes Platform, но расписан детальнее. Вы можете получить данный список самостоятельно, используя `govc role.ls` и [команду из соответсвующей секции документации](../../modules/030-cloud-provider-vsphere/environment.html#создание-и-назначение-роли).

```none
Cns.Searchable
Datastore.AllocateSpace
Datastore.Browse
Datastore.FileManagement
Folder.Create
Folder.Delete
Folder.Move
Folder.Rename
Global.GlobalTag
Global.SystemTag
Network.Assign
StorageProfile.View
InventoryService.Tagging.AttachTag
InventoryService.Tagging.CreateCategory
InventoryService.Tagging.CreateTag
InventoryService.Tagging.DeleteCategory
InventoryService.Tagging.DeleteTag
InventoryService.Tagging.EditCategory
InventoryService.Tagging.EditTag
InventoryService.Tagging.ModifyUsedByForCategory
InventoryService.Tagging.ModifyUsedByForTag
InventoryService.Tagging.ObjectAttachable
Resource.ApplyRecommendation
Resource.AssignVAppToPool
Resource.AssignVMToPool
Resource.ColdMigrate
Resource.CreatePool
Resource.DeletePool
Resource.EditPool
Resource.HotMigrate
Resource.MovePool
Resource.QueryVMotion
Resource.RenamePool
VirtualMachine.Config.AddExistingDisk
VirtualMachine.Config.AddNewDisk
VirtualMachine.Config.AddRemoveDevice
VirtualMachine.Config.AdvancedConfig
VirtualMachine.Config.Annotation
VirtualMachine.Config.ChangeTracking
VirtualMachine.Config.CPUCount
VirtualMachine.Config.DiskExtend
VirtualMachine.Config.DiskLease
VirtualMachine.Config.EditDevice
VirtualMachine.Config.HostUSBDevice
VirtualMachine.Config.ManagedBy
VirtualMachine.Config.Memory
VirtualMachine.Config.MksControl
VirtualMachine.Config.QueryFTCompatibility
VirtualMachine.Config.QueryUnownedFiles
VirtualMachine.Config.RawDevice
VirtualMachine.Config.ReloadFromPath
VirtualMachine.Config.RemoveDisk
VirtualMachine.Config.Rename
VirtualMachine.Config.ResetGuestInfo
VirtualMachine.Config.Resource
VirtualMachine.Config.Settings
VirtualMachine.Config.SwapPlacement
VirtualMachine.Config.ToggleForkParent
VirtualMachine.Config.UpgradeVirtualHardware
VirtualMachine.GuestOperations.Execute
VirtualMachine.GuestOperations.Modify
VirtualMachine.GuestOperations.ModifyAliases
VirtualMachine.GuestOperations.Query
VirtualMachine.GuestOperations.QueryAliases
VirtualMachine.Hbr.ConfigureReplication
VirtualMachine.Hbr.MonitorReplication
VirtualMachine.Hbr.ReplicaManagement
VirtualMachine.Interact.AnswerQuestion
VirtualMachine.Interact.Backup
VirtualMachine.Interact.ConsoleInteract
VirtualMachine.Interact.CreateScreenshot
VirtualMachine.Interact.CreateSecondary
VirtualMachine.Interact.DefragmentAllDisks
VirtualMachine.Interact.DeviceConnection
VirtualMachine.Interact.DisableSecondary
VirtualMachine.Interact.DnD
VirtualMachine.Interact.EnableSecondary
VirtualMachine.Interact.GuestControl
VirtualMachine.Interact.MakePrimary
VirtualMachine.Interact.Pause
VirtualMachine.Interact.PowerOff
VirtualMachine.Interact.PowerOn
VirtualMachine.Interact.PutUsbScanCodes
VirtualMachine.Interact.Record
VirtualMachine.Interact.Replay
VirtualMachine.Interact.Reset
VirtualMachine.Interact.SESparseMaintenance
VirtualMachine.Interact.SetCDMedia
VirtualMachine.Interact.SetFloppyMedia
VirtualMachine.Interact.Suspend
VirtualMachine.Interact.SuspendToMemory
VirtualMachine.Interact.TerminateFaultTolerantVM
VirtualMachine.Interact.ToolsInstall
VirtualMachine.Interact.TurnOffFaultTolerance
VirtualMachine.Inventory.Create
VirtualMachine.Inventory.CreateFromExisting
VirtualMachine.Inventory.Delete
VirtualMachine.Inventory.Move
VirtualMachine.Inventory.Register
VirtualMachine.Inventory.Unregister
VirtualMachine.Namespace.Event
VirtualMachine.Namespace.EventNotify
VirtualMachine.Namespace.Management
VirtualMachine.Namespace.ModifyContent
VirtualMachine.Namespace.Query
VirtualMachine.Namespace.ReadContent
VirtualMachine.Provisioning.Clone
VirtualMachine.Provisioning.CloneTemplate
VirtualMachine.Provisioning.CreateTemplateFromVM
VirtualMachine.Provisioning.Customize
VirtualMachine.Provisioning.DeployTemplate
VirtualMachine.Provisioning.DiskRandomAccess
VirtualMachine.Provisioning.DiskRandomRead
VirtualMachine.Provisioning.FileRandomAccess
VirtualMachine.Provisioning.GetVmFiles
VirtualMachine.Provisioning.MarkAsTemplate
VirtualMachine.Provisioning.MarkAsVM
VirtualMachine.Provisioning.ModifyCustSpecs
VirtualMachine.Provisioning.PromoteDisks
VirtualMachine.Provisioning.PutVmFiles
VirtualMachine.Provisioning.ReadCustSpecs
VirtualMachine.State.CreateSnapshot
VirtualMachine.State.RemoveSnapshot
VirtualMachine.State.RenameSnapshot
VirtualMachine.State.RevertToSnapshot
```
