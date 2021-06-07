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
    * Опциональный параметр. По умолчанию `false`.
* `masterNodeGroup` — описание master NodeGroup.
  * `replicas` — сколько мастер-узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [VsphereInstanceClass]({{"/modules/030-cloud-provider-vsphere/#vsphereinstanceclass-custom-resource" | true_relative_url }} ). Обязательными параметрами являются `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.  Параметры, обозначенные **жирным** шрифтом уникальны для `VsphereClusterConfiguration`. Допустимые параметры:
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
    * **`mainNetworkIPAddresses`** — список статических адресов (с CIDR префиксом), назначаемых (по-очереди) master-узлам в основной сети (параметр `mainNetwork`).
      * Опциональный параметр. По умолчанию, включается DHCP клиент.
      * `address` — IP адрес с CIDR префиксом.
        * Пример: `10.2.2.2/24`.
      * `gateway` — IP адрес шлюза по умолчанию. Должен находится в подсети, указанной в `address`.
        * Пример: `10.2.2.254`.
      * `nameservers`
        * `addresses` — список dns-серверов.
          * Пример: `- 8.8.8.8`
        * `search` — список DNS search domains.
          * Пример: `- tech.lan`
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
  * `name` — имя NG, будет использоваться для генерации имени узлов.
  * `replicas` — сколько узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [VsphereInstanceClass]({{"/modules/030-cloud-provider-vsphere/#vsphereinstanceclass-custom-resource" | true_relative_url }} ). Обязательными параметрами являются `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.  Параметры, обозначенные **жирным** шрифтом уникальны для `VsphereClusterConfiguration`. Допустимые параметры:
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
    * **`mainNetworkIPAddresses`** — список статических адресов (с CIDR префиксом), назначаемых (по-очереди) master-узлам в основной сети (параметр `mainNetwork`).
      * Опциональный параметр. По умолчанию, включается DHCP клиент.
      * `address` — IP адрес с CIDR префиксом.
        * Пример: `10.2.2.2/24`.
      * `gateway` — IP адрес шлюза по умолчанию. Должен находится в подсети, указанной в `address`.
        * Пример: `10.2.2.254`.
      * `nameservers`
        * `addresses` — массив dns-серверов.
          * Пример: `- 8.8.8.8`
        * `search` — массив DNS search domains.
          * Пример: `- tech.lan`
  * `nodeTemplate` — настройки Node-объектов в Kubernetes, которые будут добавлены после регистрации узла.
    * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.labels`
      * Пример:
        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.annotations`
      * Пример:
        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
      * Пример:
        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `internalNetworkCIDR` — подсеть для master-узлов во внутренней сети. Адреса выделяются с десятого адреса. Например, для подсети `192.168.199.0/24` будут использованы адреса начиная с `192.168.199.10`. Будет использоваться при использовании `additionalNetworks` в `masterInstanceClass`.
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
* `baseResourcePool` — относительный (от vSphere Cluster) путь до существующего родительского `resourcePool` для всех создаваемых (в каждой зоне) `resourcePool`'ов.
* `sshPublicKey` — публичный ключ для доступа на узлы.
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
* `zones` — ограничение набора зон, в которых разрешено создавать узлы.
  * Обязательный параметр.
  * Формат — массив строк.
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
zones:
- ru-central1-a
- ru-central1-b
```

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

Необходимо создать Role с указанными правами и прикрепить её к одному или нескольким Datacenters, где нужно развернуть Kubernetes кластер.

Упущено создание пользователя, ввиду разнообразия SSO, подключаемых к vSphere.

```shell
govc role.create kubernetes Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement Global.GlobalTag Global.SystemTag InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag Network.Assign Resource.AssignVMToPool Resource.ColdMigrate Resource.HotMigrate Resource.CreatePool Resource.DeletePool Resource.RenamePool Resource.EditPool Resource.MovePool StorageProfile.View System.Anonymous System.Read System.View VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease VirtualMachine.Config.EditDevice VirtualMachine.Config.HostUSBDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory VirtualMachine.Config.MksControl VirtualMachine.Config.QueryFTCompatibility VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement VirtualMachine.Config.ToggleForkParent VirtualMachine.Config.UpgradeVirtualHardware VirtualMachine.GuestOperations.Execute VirtualMachine.GuestOperations.Modify VirtualMachine.GuestOperations.ModifyAliases VirtualMachine.GuestOperations.Query VirtualMachine.GuestOperations.QueryAliases VirtualMachine.Hbr.ConfigureReplication VirtualMachine.Hbr.MonitorReplication VirtualMachine.Hbr.ReplicaManagement VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.Backup VirtualMachine.Interact.ConsoleInteract VirtualMachine.Interact.CreateScreenshot VirtualMachine.Interact.CreateSecondary VirtualMachine.Interact.DefragmentAllDisks VirtualMachine.Interact.DeviceConnection VirtualMachine.Interact.DisableSecondary VirtualMachine.Interact.DnD VirtualMachine.Interact.EnableSecondary VirtualMachine.Interact.GuestControl VirtualMachine.Interact.MakePrimary VirtualMachine.Interact.Pause VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn VirtualMachine.Interact.PutUsbScanCodes VirtualMachine.Interact.Record VirtualMachine.Interact.Replay VirtualMachine.Interact.Reset VirtualMachine.Interact.SESparseMaintenance VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.SetFloppyMedia VirtualMachine.Interact.Suspend VirtualMachine.Interact.TerminateFaultTolerantVM VirtualMachine.Interact.ToolsInstall VirtualMachine.Interact.TurnOffFaultTolerance VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete VirtualMachine.Inventory.Move VirtualMachine.Inventory.Register VirtualMachine.Inventory.Unregister VirtualMachine.Namespace.Event VirtualMachine.Namespace.EventNotify VirtualMachine.Namespace.Management VirtualMachine.Namespace.ModifyContent VirtualMachine.Namespace.Query VirtualMachine.Namespace.ReadContent VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.CloneTemplate VirtualMachine.Provisioning.CreateTemplateFromVM VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate VirtualMachine.Provisioning.DiskRandomAccess VirtualMachine.Provisioning.DiskRandomRead VirtualMachine.Provisioning.FileRandomAccess VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.MarkAsTemplate VirtualMachine.Provisioning.MarkAsVM VirtualMachine.Provisioning.ModifyCustSpecs VirtualMachine.Provisioning.PromoteDisks VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot VirtualMachine.State.RevertToSnapshot

govc permissions.set  -principal имя_пользователя -role kubernetes /datacenter
```

## Инфраструктура

### Сети
Для работы кластера необходим VLAN с DHCP и доступом в Интернет
* Если VLAN публичный (белые адреса), то нужна вторая сеть, в которой мы будем разворачивать сеть узлов кластера (в этой сети DHCP не нужен)
* Если VLAN внутренний (серые адреса), то эта же сеть будет сетью узлов кластера

### Входящий трафик
* Если у вас имеется внутренний балансировщик запросов, то можно обойтись им и направлять трафик напрямую на front-узлы кластера.
* Если балансировщика нет, то для организации отказоустойчивых Lоadbalancer'ов рекомендуется использовать MetalLB в режиме BGP. В кластере будут созданы front-узлы с двумя интерфейсами. Для этого дополнительно потребуются:
  * Отдельный VLAN для обмена трафиком между BGP-роутерами и MetalLB. В этом VLAN'e должен быть DHCP и доступ в Интернет
  * IP адреса BGP-роутеров
  * ASN (номер автономной системы) на BGP-роутере
  * ASN (номер автономной системы) в кластере
  * Диапазон, из которого анонсировать адреса

### Использование хранилища данных
В кластере может одновременно использоваться различное количество типов хранилищ, в минимальной конфигурации потребуются:
* Datastore, в котором kubernetes кластер будет заказывать PersistentVolume
* Datastore, в котором будут заказываться рутовые диски для VM (может быть тот же Datastore, что и для PersistentVolume)
