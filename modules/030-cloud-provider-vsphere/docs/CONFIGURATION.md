---
title: "Сloud provider — VMware vSphere: настройки"
---

Модуль автоматически включается для всех облачных кластеров развёрнутых в vSphere.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера vSphere — это custom resource [`VsphereInstanceClass`](cr.html#vsphereinstanceclass), в котором указываются конкретные параметры самих машин.

## Параметры

Настройки модуля устанавливаются автоматически на основании [выбранной схемы размещения](/candi/). В
большинстве случаев нет необходимости в ручной конфигурации модуля.

Если вам необходимо настроить модуль, потому что, например, у вас bare metal кластер, для которого нужно включить
возможность добавлять дополнительные инстансы из vSphere, то смотрите раздел как [настроить Hybrid кластер в vSphere](faq.html#как-поднять-гибридный-вручную-заведённые-ноды-кластер).

## Storage

StorageClass будет создан автоматически для каждого Datastore и DatastoreCluster из зон(-ы). Для указания default StorageClass, необходимо в конфигурацию модуля добавить параметр `defaultDataStore`.

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и vSphere API, после увеличения размера PVC нужно:

1. Выполнить `kubectl cordon нода_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что ресайз произошёл успешно. В объекте PVC *не будет* condition `Resizing`. **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon нода_где_находится_pod`.

## Требования к окружениям

1. Требования к версии vSphere: `v6.7U2`.
2. vCenter, до которого есть доступ изнутри кластера с master нод.
3. Создать Datacenter, а в нём:

    1. VirtualMachine template со [специальным](https://github.com/vmware/cloud-init-vmware-guestinfo) cloud-init datasource внутри.
        * Подготовить образ Ubuntu 18.04, например, можно с помощью [скрипта](https://github.com/deckhouse/deckhouse/blob/master/install-kubernetes/vsphere/prepare-template).
    2. Network, доступная на всех ESXi, на которых будут создаваться VirtualMachines.
    3. Datastore (или несколько), подключённый ко всем ESXi, на которых будут создаваться VirtualMachines.
        * На Datastore-ы **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**. Все Cluster'а из конкретной зоны должны иметь доступ ко всем Datastore'ам, с идентичной зоной.
    4. Cluster, в который добавить необходимые используемые ESXi.
        * На Cluster **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**.
    5. Folder для создаваемых VirtualMachines.
        * Опциональный. По умолчанию будет использоваться root vm папка.
    6. Создать роль с необходимым [набором](#список-привилегий-для-использования-модуля) прав.
    7. Создать пользователя, привязав к нему роль из пункта #6.

4. На созданный Datacenter **необходимо** "повесить" тэг из категории тэгов, указанный в `regionTagCategory` (по умолчанию, `k8s-region`). Этот тэг будет обозначать **регион**.

## Список привилегий для использования модуля

```none
Datastore.AllocateSpace
Datastore.FileManagement
Global.GlobalTag
Global.SystemTag
InventoryService.Tagging.AttachTag
InventoryService.Tagging.CreateCategory
InventoryService.Tagging.CreateTag
InventoryService.Tagging.DeleteCategory
InventoryService.Tagging.DeleteTag
InventoryService.Tagging.EditCategory
InventoryService.Tagging.EditTag
InventoryService.Tagging.ModifyUsedByForCategory
InventoryService.Tagging.ModifyUsedByForTag
Network.Assign
Resource.AssignVMToPool
StorageProfile.View
System.Anonymous
System.Read
System.View
VirtualMachine.Config.AddExistingDisk
VirtualMachine.Config.AddNewDisk
VirtualMachine.Config.AddRemoveDevice
VirtualMachine.Config.AdvancedConfig
VirtualMachine.Config.Annotation
VirtualMachine.Config.CPUCount
VirtualMachine.Config.ChangeTracking
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

## Включение поддержки UUID для всех виртуальных машин

Для работы `vsphere-csi-driver` у всех виртуальных машин кластера необходимо включить поддержку параметра `disk.EnableUUID`.

Для этого в интерфейсе vSphere необходимо нажать правой кнопкой на каждую виртуальную машину и выбрать пункт меню: `Edit Settings...` и перейти на вкладку `VM Options`:

![](img/edit_settings.png)

Открыть раздел `Advanced`:

![](img/advanced.png)

И в `Configuration Parameters` нажать на `EDIT CONFIGURATION...`. В данном списке параметров необходимо найти `disk.EnableUUID`, если данного параметра нет, то его необходимо включить. Для этого необходимо:

* Выключить виртуальную машину;
* Перейти в раздел `EDIT CONFIGURATION...` (как было описано выше);
* В правом верхнем углу нажать на кнопку `ADD CONFIGURATION PARAMS`;

![](img/configuration_params.png)

* Ввести имя параметра `disk.EnableUUID` с значением `TRUE`;

![](img/add_new_configuration_params.png)

* Нажать на кнопку `OK`;
* Включить виртуальную машину.
