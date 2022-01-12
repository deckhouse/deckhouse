---
title: "Сloud provider — VMware vSphere: настройки"
---

Модуль автоматически включается для всех облачных кластеров развёрнутых в vSphere.

Количество и параметры процесса заказа машин в облаке настраиваются в Custom Resource [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup). Instance-класс для cloud-провайдера vSphere — это Custom Resource [`VsphereInstanceClass`](cr.html#vsphereinstanceclass), в котором указываются конкретные параметры самих машин.

## Параметры

Настройки модуля устанавливаются автоматически на основании выбранной схемы размещения. В большинстве случаев нет необходимости в ручной конфигурации модуля.

Если у вас bare metal кластер, для которого нужно включить
возможность добавлять дополнительные инстансы из vSphere, и вам необходимо настроить модуль, — смотрите раздел как [настроить Hybrid кластер в vSphere](faq.html#как-поднять-гибридный-кластер).

**Внимание!** При изменении конфигурационных параметров приведенных в этой секции (указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `VsphereInstanceClass`. См. подробнее в документации модуля [node-manager](../../modules/040-node-manager/faq.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `host` — домен vCenter сервера.
* `username` — логин.
* `password` — пароль.
* `insecure` — можно выставить в `true`, если vCenter имеет самоподписанный сертификат.
  * Формат — bool.
  * Опциональный параметр. По умолчанию `false`.
* `vmFolderPath` — путь до VirtualMachine Folder, в котором будут создаваться склонированные виртуальные машины.
  * Пример — `dev/test`
* `regionTagCategory`— имя **категории** тэгов, использующихся для идентификации региона (vSphere Datacenter).
  * Формат — string.
  * Опциональный параметр. По умолчанию `k8s-region`.
* `zoneTagCategory` — имя **категории** тэгов, использующихся для идентификации зоны (vSphere Cluster).
    * Формат — string.
    * Опциональный параметр. По умолчанию `k8s-zone`.
* `disableTimesync` — отключить ли синхронизацию времени со стороны vSphere. **Внимание!** это не отключит NTP демоны в гостевой ОС, а лишь отключит "подруливание" временем со стороны ESXi.
  * Формат — bool.
  * Опциональный параметр. По умолчанию `true`.
* `region` — тэг, прикреплённый к vSphere Datacenter, в котором будут происходить все операции: заказ VirtualMachines, размещение их дисков на datastore, подключение к network.
* `sshKeys` — список public SSH ключей в plain-text формате.
  * Формат — массив строк.
  * Опциональный параметр. По умолчанию разрешённых ключей для пользователя по умолчанию не будет.
* `externalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и используемые vsphere-cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

  Пример:
  ```yaml
  externalNetworkNames:
  - MAIN-1
  - public
  ```

  * Опциональный параметр.
* `internalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и используемые vsphere-cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

  Пример:
  ```yaml
  internalNetworkNames:
  - KUBE-3
  - devops-internal
  ```

  * Опциональный параметр.

## Storage

Модуль автоматически создаёт StorageClass для каждого Datastore и DatastoreCluster из зон(-ы). А также позволяет отфильтровать ненужные, указанием их в параметре `exclude`.

* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
  * Формат — массив строк.
  * Опциональный параметр.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию.
  * Формат — строка.
  * Опциональный параметр.
  * Если параметр не задан, фактическим StorageClass по умолчанию будет либо:
    * Присутствующий в кластере произвольный StorageClass с default аннотацией.
    * Лексикографически первый StorageClass из создаваемых модулем.

```yaml
cloudProviderVsphere: |
  storageClass:
    exclude:
    - ".*-lun101-.*"
    - slow-lun103-1c280603
    default: fast-lun102-7d0bf578
```

### CSI

Подсистема хранения по умолчанию использует CNS-диски с возможностью изменения их размера на-лету. Но также поддерживается работа и в legacy-режиме, с использованием FCD-дисков.

* `compatibilityFlag` — флаг, позволяющий использовать старую версию CSI.
  * Формат — строка.
  * Возможные значения:
      * `legacy` — использовать старую версию драйвера. Только FCD диски без изменения размера на-лету.
      * `migration` — в этом случае в кластере будут одновременно доступны оба драйвера. Данный режим используется для миграции со старого драйвера.
  * Опциональный параметр.

```yaml
cloudProviderVsphere: |
  storageClass:
    compatibilityFlag: legacy
```

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и vSphere API, после увеличения размера PVC нужно:

1. Выполнить `kubectl cordon узел_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что ресайз произошёл успешно. В объекте PVC *не будет* condition `Resizing`. **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon узел_где_находится_pod`.

## Требования к окружениям

1. Требования к версии vSphere: `v7.0U2` ([необходимо](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) для работы механизма `Online volume expansion`).
2. vCenter, до которого есть доступ изнутри кластера с master-узлов.
3. Создать Datacenter, в котором создать:

    1. VirtualMachine template со [специальным](https://github.com/vmware/cloud-init-vmware-guestinfo) cloud-init datasource внутри.
       * Образ ВМ должен использовать `Virtual machines with hardware version 15 or later` (необходимо для работы online resize).
    2. Network, доступную на всех ESXi, на которых будут создаваться VirtualMachines.
    3. Datastore (или несколько), подключённый ко всем ESXi, на которых будут создаваться VirtualMachines.
        * На Datastore-ы **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**. Все Cluster'а из конкретной зоны должны иметь доступ ко всем Datastore'ам, с идентичной зоной.
    4. Cluster, в который добавить необходимые используемые ESXi.
        * На Cluster **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**.
    5. Folder для создаваемых VirtualMachines.
        * Опциональный. По умолчанию будет использоваться root vm папка.
    6. Роль с необходимым [набором](#список-привилегий-для-использования-модуля) прав.
    7. Пользователя, привязав к нему роль из пункта #6.

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
