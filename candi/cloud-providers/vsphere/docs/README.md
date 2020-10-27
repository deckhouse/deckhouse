---
title: Cloud provider - Vsphere
---

## Поддерживаемые схемы размещения

Схема размещения описывается объектом `VsphereClusterConfiguration`. Его поля:

* `layout` — архитектура расположения ресурсов в облаке.
  * Варианты — `Standard` (описание ниже).
* `provider` — параметры подключения к vCenter.
  * `server` — хост или IP vCenter сервера.
  * `username` — логин.
  * `password` — пароль.
  * `insecure` — можно выставить в `true`, если vCenter имеет самоподписанный сертификат.
    * Формат — bool.
    * Опциональный параметр. По-умолчанию `false`.
* `masterNodeGroup` — описание master NodeGroup.
  * `replicas` — сколько мастер-узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [VsphereInstanceClass](/modules/030-cloud-provider-vsphere/#vsphereinstanceclass-custom-resource). Обязательными параметрами являются `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.  Параметры, обозначенные **жирным** шрифтом уникальны для `VsphereClusterConfiguration`. Допустимые параметры:
    * `numCPUs`
    * `memory`
    * `template`
    * `mainNetwork`
    * `additionalNetworks`
    * `datastore`
    * `rootDiskSize`
    * `resourcePool`
    * `runtimeOptions`
      * `nestedHardwareVirtualization`
      * `cpuShares`
      * `cpuLimit`
      * `cpuReservation`
      * `memoryShares`
      * `memoryLimit`
      * `memoryReservation`
    * **`mainNetworkIPAddresses`** — список статических адресов (с CIDR префиксом), назначаемых (по-очереди) master нодам в основной сети (параметр `mainNetwork`).
      * Формат — список строк.
      * Опциональный параметр. По-умолчанию, включается DHCP клиент.
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
  * `name` — имя NG, будет использоваться для генерации имени нод.
  * `replicas` — сколько узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [VsphereInstanceClass](/modules/030-cloud-provider-vsphere/#vsphereinstanceclass-custom-resource). Обязательными параметрами являются `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.  Параметры, обозначенные **жирным** шрифтом уникальны для `VsphereClusterConfiguration`. Допустимые параметры:
    * `numCPUs`
    * `memory`
    * `template`
    * `mainNetwork`
    * `additionalNetworks`
    * `datastore`
    * `rootDiskSize`
    * `resourcePool`
    * `runtimeOptions`
      * `nestedHardwareVirtualization`
      * `cpuShares`
      * `cpuLimit`
      * `cpuReservation`
      * `memoryShares`
      * `memoryLimit`
      * `memoryReservation`
    * **`mainNetworkIPAddresses`** — список статических адресов (с CIDR префиксом), назначаемых (по-очереди) master нодам в основной сети (параметр `mainNetwork`).
      * Формат — список строк.
      * Опциональный параметр. По-умолчанию, включается DHCP клиент.
* `internalNetworkCIDR` — подсеть для master нод во внутренней сети. Адреса выделяются с десятого адреса. Например, для подсети `192.168.199.0/24` будут использованы адреса начиная с `192.168.199.10`. Будет использоваться при использовании `additionalNetworks` в `masterInstanceClass`.
* `vmFolderPath` — путь до VirtualMachine Folder, в котором будут создаваться склонированные виртуальные машины.
  * Пример — `dev/test`
* `regionTagCategory`— имя **категории** тэгов, использующихся для идентификации региона (vSphere Datacenter).
  * Формат — string.
  * Опциональный параметр. По-умолчанию `k8s-region`.
* `zoneTagCategory` — имя **категории** тэгов, использующихся для идентификации зоны (vSphere Cluster).
  * Формат — string.
  * Опциональный параметр. По-умолчанию `k8s-zone`.
* `defaultDatastore` — имя vSphere Datastore, который будет использоваться в качестве default StorageClass.
  * Формат — string.
  * Опциональный параметр. По-умолчанию будет использован лексикографически первый Datastore.
* `disableTimesync` — отключить ли синхронизацию времени со стороны vSphere. **Внимание!** это не отключит NTP демоны в гостевой ОС, а лишь отключит "подруливание" временем со стороны ESXi.
  * Формат — bool.
  * Опциональный параметр. По-умолчанию `true`.
* `region` — тэг, прикреплённый к vSphere Datacenter, в котором будут происходить все операции: заказ VirtualMachines, размещение их дисков на datastore, подключение к network.
* `sshPublicKey` — публичный ключ для доступа на ноды.
* `externalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и используемые vsphere-cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

    ```yaml
    externalNetworkNames:
    - MAIN-1
    - public
    ```

    * Опциональный параметр.
* `internalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и используемые vsphere-cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

    ```yaml
    internalNetworkNames:
    - KUBE-3
    - devops-internal
    ```

  * Опциональный параметр.

### Standard

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQolOJQw4clYDug78Mr7rvX7wYPsb2uVhab5cDZrzBKq76Ox6dZhgoBXuq-ta8DRC2grjNUcfEq_AR8/pub?w=667&h=516)
<!--- Исходник: https://docs.google.com/drawings/d/1QOgPkq_xfBWMMI3SEU4Q9lyZM5mIWWbF_MwVsd06diE/edit --->

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: test.local.org
  username: test@dvdv.org
  password: testtest
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
defaultDatastore: lun_2_dev
region: X1
internalNetworkCIDR: 192.168.199.0/24
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
    mainNetwork: k8s-msk/test_187
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
    mainNetwork: k8s-msk/test_187
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
```

## Ручные настройки

### Настройка govc

```shell
export GOVC_URL=n-cs-5.hq.li.corp.kavvas.com
export GOVC_USERNAME=ewwefsadfsda
export GOVC_PASSWORD=weqrfweqfeds
export GOVC_INSECURE=1
```

### Создание тэгов и категорий тэгов

В vSphere нет понятия "регион" и "зона", поэтому для разграничения зон доступности используются тэги.

```shell
govc tags.category.create -d "Kubernetes region" k8s-region
govc tags.category.create -d "Kubernetes zone" k8s-zone
govc tags.create -d "Kubernetes Region X1" -c k8s-region k8s-region-x1
govc tags.create -d "Kubernetes Region X2" -c k8s-region k8s-region-x2
govc tags.create -d "Kubernetes Zone X1-A" -c k8s-zone k8s-zone-x1-a
govc tags.create -d "Kubernetes Zone X1-B" -c k8s-zone k8s-zone-x1-b
govc tags.create -d "Kubernetes Zone X2-A" -c k8s-zone k8s-zone-x2-a
```

Созданные категории тэгов необходимо указать в `VsphereClusterConfiguration` в `.spec.provider`.

Тэги *регионов* навешиваются на Datacenter:

```shell
govc tags.attach -c k8s-region k8s-region-x1 /X1
```

Тэги *зон* навешиваются на Cluster и Datastores:

```shell
govc tags.attach -c k8s-zone k8s-zone-x1-a /X1/host/x1_cluster_prod
govc tags.attach -c k8s-zone k8s-zone-x1-a /X1/host/x1_cluster_prod
```

### Права

Необходимо создать Role с указанными правами и прикрепить её к одному или нескольким Datacenters, где нужно развернуть Kubernetes кластер.

Упущено создание пользователя, ввиду разнообразия SSO, подключаемых к vSphere.

```shell
govc role.create kubernetes Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement Global.GlobalTag Global.SystemTag InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag Network.Assign Resource.AssignVMToPool Resource.ColdMigrate Resource.HotMigrate StorageProfile.View System.Anonymous System.Read System.View VApp.ApplicationConfig VApp.Import VApp.InstanceConfig VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease VirtualMachine.Config.EditDevice VirtualMachine.Config.HostUSBDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory VirtualMachine.Config.MksControl VirtualMachine.Config.QueryFTCompatibility VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement VirtualMachine.Config.ToggleForkParent VirtualMachine.Config.UpgradeVirtualHardware VirtualMachine.GuestOperations.Execute VirtualMachine.GuestOperations.Modify VirtualMachine.GuestOperations.ModifyAliases VirtualMachine.GuestOperations.Query VirtualMachine.GuestOperations.QueryAliases VirtualMachine.Hbr.ConfigureReplication VirtualMachine.Hbr.MonitorReplication VirtualMachine.Hbr.ReplicaManagement VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.Backup VirtualMachine.Interact.ConsoleInteract VirtualMachine.Interact.CreateScreenshot VirtualMachine.Interact.CreateSecondary VirtualMachine.Interact.DefragmentAllDisks VirtualMachine.Interact.DeviceConnection VirtualMachine.Interact.DisableSecondary VirtualMachine.Interact.DnD VirtualMachine.Interact.EnableSecondary VirtualMachine.Interact.GuestControl VirtualMachine.Interact.MakePrimary VirtualMachine.Interact.Pause VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn VirtualMachine.Interact.PutUsbScanCodes VirtualMachine.Interact.Record VirtualMachine.Interact.Replay VirtualMachine.Interact.Reset VirtualMachine.Interact.SESparseMaintenance VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.SetFloppyMedia VirtualMachine.Interact.Suspend VirtualMachine.Interact.TerminateFaultTolerantVM VirtualMachine.Interact.ToolsInstall VirtualMachine.Interact.TurnOffFaultTolerance VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete VirtualMachine.Inventory.Move VirtualMachine.Inventory.Register VirtualMachine.Inventory.Unregister VirtualMachine.Namespace.Event VirtualMachine.Namespace.EventNotify VirtualMachine.Namespace.Management VirtualMachine.Namespace.ModifyContent VirtualMachine.Namespace.Query VirtualMachine.Namespace.ReadContent VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.CloneTemplate VirtualMachine.Provisioning.CreateTemplateFromVM VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate VirtualMachine.Provisioning.DiskRandomAccess VirtualMachine.Provisioning.DiskRandomRead VirtualMachine.Provisioning.FileRandomAccess VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.MarkAsTemplate VirtualMachine.Provisioning.MarkAsVM VirtualMachine.Provisioning.ModifyCustSpecs VirtualMachine.Provisioning.PromoteDisks VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot VirtualMachine.State.RevertToSnapshot

govc permissions.set  -principal имя_пользователя -role kubernetes /datacenter
```
