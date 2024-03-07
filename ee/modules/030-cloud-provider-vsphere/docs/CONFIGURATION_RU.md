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

## Требования к окружению

* Требования к версии vSphere: `v7.0U2` ([необходимо](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) для работы механизма `Online volume expansion`).
* vCenter, до которого есть доступ изнутри кластера с master-узлов.
* Создать Datacenter, в котором создать:
  1. VirtualMachine template.
     * Образ виртуальной машины должен использовать `Virtual machines with hardware version 15 or later` (необходимо для работы online resize).
     * В образе должны быть установлены следующие пакеты: `open-vm-tools`, `cloud-init` и [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (если используется версия `cloud-init` ниже 21.3).
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

> О том, как создать и назначить роль пользователю, читайте [в документации](environment.html#создание-и-назначение-роли).

Детальный список привилегий, необходимых для работы Deckhouse Kubernetes Platform в vSphere:

| Необходимые привилегии в vSphere API | Описание |
|--------------------------------------|----------|
| `Cns.Searchable`<br>`StorageProfile.View`<br>`Datastore.AllocateSpace`<br>`Datastore.Browse`<br>`Datastore.FileManagement` | Для выделения дисков при создании виртуальных машин и заказе `PersistentVolumes` в кластере. |
| `Global.GlobalTag`<br>`Global.SystemTag`<br>`InventoryService.Tagging.AttachTag`<br>`InventoryService.Tagging.CreateCategory`<br>`InventoryService.Tagging.CreateTag`<br>`InventoryService.Tagging.DeleteCategory`<br>`InventoryService.Tagging.DeleteTag`<br>`InventoryService.Tagging.EditCategory`<br>`InventoryService.Tagging.EditTag`<br>`InventoryService.Tagging.ModifyUsedByForCategory`<br>`InventoryService.Tagging.ModifyUsedByForTag`<br>`InventoryService.Tagging.ObjectAttachable` | Deckhouse Kubernetes Platform использует теги для определения доступных ему объектов `Datacenter`, `Cluster` и `Datastore`, а также, для опредения виртуальных машин, находящихся под его управлением. |
| `Folder.Create`<br>`Folder.Delete`<br>`Folder.Move`<br>`Folder.Rename` | Для группировки кластера Deckhouse Kubernetes Platform в одном `Folder` в vSphere Inventory. |
| `Network.Assign`<br>`Resource.ApplyRecommendation`<br>`Resource.AssignVAppToPool`<br>`Resource.AssignVMToPool`<br>`Resource.ColdMigrate`<br>`Resource.CreatePool`<br>`Resource.DeletePool`<br>`Resource.EditPool`<br>`Resource.HotMigrate`<br>`Resource.MovePool`<br>`Resource.QueryVMotion`<br>`Resource.RenamePool`<br>`VirtualMachine.Config.AddExistingDisk`<br>`VirtualMachine.Config.AddNewDisk`<br>`VirtualMachine.Config.AddRemoveDevice`<br>`VirtualMachine.Config.AdvancedConfig`<br>`VirtualMachine.Config.Annotation`<br>`VirtualMachine.Config.ChangeTracking`<br>`VirtualMachine.Config.CPUCount`<br>`VirtualMachine.Config.DiskExtend`<br>`VirtualMachine.Config.DiskLease`<br>`VirtualMachine.Config.EditDevice`<br>`VirtualMachine.Config.HostUSBDevice`<br>`VirtualMachine.Config.ManagedBy`<br>`VirtualMachine.Config.Memory`<br>`VirtualMachine.Config.MksControl`<br>`VirtualMachine.Config.QueryFTCompatibility`<br>`VirtualMachine.Config.QueryUnownedFiles`<br>`VirtualMachine.Config.RawDevice`<br>`VirtualMachine.Config.ReloadFromPath`<br>`VirtualMachine.Config.RemoveDisk`<br>`VirtualMachine.Config.Rename`<br>`VirtualMachine.Config.ResetGuestInfo`<br>`VirtualMachine.Config.Resource`<br>`VirtualMachine.Config.Settings`<br>`VirtualMachine.Config.SwapPlacement`<br>`VirtualMachine.Config.ToggleForkParent`<br>`VirtualMachine.Config.UpgradeVirtualHardware`<br>`VirtualMachine.GuestOperations.Execute`<br>`VirtualMachine.GuestOperations.Modify`<br>`VirtualMachine.GuestOperations.ModifyAliases`<br>`VirtualMachine.GuestOperations.Query`<br>`VirtualMachine.GuestOperations.QueryAliases`<br>`VirtualMachine.Hbr.ConfigureReplication`<br>`VirtualMachine.Hbr.MonitorReplication`<br>`VirtualMachine.Hbr.ReplicaManagement`<br>`VirtualMachine.Interact.AnswerQuestion`<br>`VirtualMachine.Interact.Backup`<br>`VirtualMachine.Interact.ConsoleInteract`<br>`VirtualMachine.Interact.CreateScreenshot`<br>`VirtualMachine.Interact.CreateSecondary`<br>`VirtualMachine.Interact.DefragmentAllDisks`<br>`VirtualMachine.Interact.DeviceConnection`<br>`VirtualMachine.Interact.DisableSecondary`<br>`VirtualMachine.Interact.DnD`<br>`VirtualMachine.Interact.EnableSecondary`<br>`VirtualMachine.Interact.GuestControl`<br>`VirtualMachine.Interact.MakePrimary`<br>`VirtualMachine.Interact.Pause`<br>`VirtualMachine.Interact.PowerOff`<br>`VirtualMachine.Interact.PowerOn`<br>`VirtualMachine.Interact.PutUsbScanCodes`<br>`VirtualMachine.Interact.Record`<br>`VirtualMachine.Interact.Replay`<br>`VirtualMachine.Interact.Reset`<br>`VirtualMachine.Interact.SESparseMaintenance`<br>`VirtualMachine.Interact.SetCDMedia`<br>`VirtualMachine.Interact.SetFloppyMedia`<br>`VirtualMachine.Interact.Suspend`<br>`VirtualMachine.Interact.SuspendToMemory`<br>`VirtualMachine.Interact.TerminateFaultTolerantVM`<br>`VirtualMachine.Interact.ToolsInstall`<br>`VirtualMachine.Interact.TurnOffFaultTolerance`<br>`VirtualMachine.Inventory.Create`<br>`VirtualMachine.Inventory.CreateFromExisting`<br>`VirtualMachine.Inventory.Delete`<br>`VirtualMachine.Inventory.Move`<br>`VirtualMachine.Inventory.Register`<br>`VirtualMachine.Inventory.Unregister`<br>`VirtualMachine.Namespace.Event`<br>`VirtualMachine.Namespace.EventNotify`<br>`VirtualMachine.Namespace.Management`<br>`VirtualMachine.Namespace.ModifyContent`<br>`VirtualMachine.Namespace.Query`<br>`VirtualMachine.Namespace.ReadContent`<br>`VirtualMachine.Provisioning.Clone`<br>`VirtualMachine.Provisioning.CloneTemplate`<br>`VirtualMachine.Provisioning.CreateTemplateFromVM`<br>`VirtualMachine.Provisioning.Customize`<br>`VirtualMachine.Provisioning.DeployTemplate`<br>`VirtualMachine.Provisioning.DiskRandomAccess`<br>`VirtualMachine.Provisioning.DiskRandomRead`<br>`VirtualMachine.Provisioning.FileRandomAccess`<br>`VirtualMachine.Provisioning.GetVmFiles`<br>`VirtualMachine.Provisioning.MarkAsTemplate`<br>`VirtualMachine.Provisioning.MarkAsVM`<br>`VirtualMachine.Provisioning.ModifyCustSpecs`<br>`VirtualMachine.Provisioning.PromoteDisks`<br>`VirtualMachine.Provisioning.PutVmFiles`<br>`VirtualMachine.Provisioning.ReadCustSpecs`<br>`VirtualMachine.State.CreateSnapshot`<br>`VirtualMachine.State.RemoveSnapshot`<br>`VirtualMachine.State.RenameSnapshot`<br>`VirtualMachine.State.RevertToSnapshot` | Для управления жизненным циклом виртуальных машин кластера Deckhouse Kubernetes Platform. |
