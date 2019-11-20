# Модуль cloud-provider-vsphere

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами Vsphere из Kubernetes.
    1. Синхронизирует метаданные vSphere VirtualMachines и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в vSphere.
2. flannel — DaemonSet. Настраивает PodNetwork между нодами.
3. CSI storage — для заказа дисков на datastore через механизм First-Class Disk.
4. Регистрация в модуле [cloud-instance-manager](modules/040-cloud-instance-manager), чтобы [VsphereInstanceClass'ы](#VsphereInstanceClass-custom-resource) можно было использовать в [CloudInstanceClass'ах](modules/040-cloud-instance-manager/README.md#CloudInstanceGroup-custom-resource).

## Конфигурация

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения:

1. Корректно [настроить](#Требования-к-окружениям) окружение.
2. Установить deckhouse с помощью `install.sh`, добавив ему параметр — `--extra-config-map-data base64_encoding_of_custom_config`.
3. Настроить параметры модуля.

### Параметры

* `host` — домен vCenter сервера.
* `username` — логин.
* `password` — пароль.
* `vmFolderPath` — путь до VirtualMachine Folder, в котором будут создаваться склонированные виртуальные машины.
    * Пример — `dev/test`
* `insecure` — можно выставить в `true`, если vCenter имеет самоподписанный сертификат.
    * Формат — bool.
    * Опциональный параметр. По-умолчанию `false`.
* `regionTagCategory`— имя **категории** тэгов, использующихся для идентификации региона (vSphere Datacenter).
    * Формат — string.
    * Опциональный параметр. По-умолчанию `k8s-region`.
* `zoneTagCategory`: имя **категории** тэгов, использующихся для идентификации зоны (vSphere Cluster).
    * Формат — string.
    * Опциональный параметр. По-умолчанию `k8s-zone`.
* `defaultDatastore`: имя vSphere Datastore, который будет использоваться в качестве default StorageClass.
    * Формат — string.
    * Опциональный параметр. По-умолчанию будет использован лексикографически первый Datastore.
* `region` — тэг, прикреплённый к vSphere Datacenter, в котором будут происходить все операции: заказ VirtualMachines, размещение их дисков на datastore, подключение к network.
* `sshKeys` — список public SSH ключей в plain-text формате.
    * Формат — массив строк.
    * Опциональный параметр. По-умолчанию разрешённых ключей для пользователя по-умолчанию не будет.
* `internalSubnet` — subnet CIDR, использующийся для внутренней межнодовой сети. Используется для настройки параметра `--iface-regex` во flannel.
    * Формат — string. Например, `10.201.0.0/16`.
    * Опциональный параметр.
* `externalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и использумые vsphere-cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
    * Формат — массив строк. Например,

        ```yaml
        externalNetworkNames:
        - MAIN-1
        - public
        ```

    * Опциональный параметр.
* `internalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и использумые vsphere-cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
    * Формат — массив строк. Например,

        ```yaml
        internalNetworkNames:
        - KUBE-3
        - devops-internal
        ```

    * Опциональный параметр.

#### Пример конфигурации

```yaml
cloudProviderVsphereEnabled: "true"
cloudProviderVsphere: |
  host: vc-3.internal
  username: user
  password: password
  vmFolderPath: dev/test
  insecure: true
  region: moscow-x001
  sshKeys:
  - "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD5sAcceTHeT6ZnU+PUF1rhkIHG8/B36VWy/j7iwqqimC9CxgFTEi8MPPGNjf+vwZIepJU8cWGB/By1z1wLZW3H0HMRBhv83FhtRzOaXVVHw38ysYdQvYxPC0jrQlcsJmLi7Vm44KwA+LxdFbkj+oa9eT08nQaQD6n3Ll4+/8eipthZCDFmFgcL/IWy6DjumN0r4B+NKHVEdLVJ2uAlTtmiqJwN38OMWVGa4QbvY1qgwcyeCmEzZdNCT6s4NJJpzVsucjJ0ZqbFqC7luv41tNuTS3Moe7d8TwIrHCEU54+W4PIQ5Z4njrOzze9/NlM935IzpHYw+we+YR+Nz6xHJwwj i@my-PC"
  internalSubnet: "10.0.201.0/16"
  externalNetworkNames:
  - KUBE-3
  - devops-internal
  internalNetworkNames:
  - KUBE-3
  - devops-internal
```

### VsphereInstanceClass custom resource

Ресурс описывает параметры группы vSphere VirtualMachines, которые будет использовать machine-controller-manager из модуля [cloud-instance-manager](modules/040-cloud-instance-manager). На `VsphereInstanceClass` ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `numCPUs` — количество виртуальных процессорных ядер, выделяемых VirtualMachine.
    * Формат — integer
* `memory` — количество памяти, выделенных VirtualMachine.
    * Формат — integer. В мебибайтах.
* `rootDiskSize` — размер корневого диска в VirtualMachine. Если в template диск меньше, автоматически произойдёт его расширение.
    * Формат — integer. В гибибайтах.
* `template` — путь до VirtualMachine Template, который будет склонирован для создания новой VirtualMachine.
    * Пример — `dev/golden_image`
* `mainNetwork` — путь до network, которая будет подключена к виртуальной машине.
    * Пример — `k8s-msk-178`
* `additionalNetworks` — список путей до networks, которые будут подключены к виртуальной машине.
    * Формат — массив строк.
    * Пример:

        ```yaml
        - DEVOPS_49
        - DEVOPS_50
        ```

* `datastore` — путь до Datastore, на котором будет созданы склонированные виртуальные машины.
    * Пример — `lun-1201`
* `cloudInitSteps` — параметры bootstrap фазы.
    * `version` — версия. По сути, имя директории [здесь](modules/040-cloud-instance-manager/cloud-init-steps).
        * По-умолчанию `ubuntu-18.04-1.0`.
        * **WIP!** Precooked версия требует специально подготовленного образа.
    * `options` — ассоциативный массив параметров. Уникальный для каждой `version` и описано в [`README.md`](modules/040-cloud-instance-manager/cloud-init-steps) соответствующих версий. Пример для [ubuntu-18.04-1.0](modules/040-cloud-instance-manager/cloud-init-steps/ubuntu-18.04-1.0):

        ```yaml
        options:
          kubernetesVersion: "1.15.3"
        ```

#### Пример VsphereInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VsphereInstanceClass
metadata:
  name: test
spec:
  numCPUs: 2
  memory: 2048
  rootDiskSize: 20
  template: dev/golden_image
  network: k8s-msk-178
  datastore: lun-1201
```

### Storage

StorageClass будет создан автоматически для каждого Datastore из зон(-ы). Для указания default StorageClass, необходимо в конфигурацию модуля добавить параметр `defaultDataStore`.

#### Важная информация об увеличении размера PVC

Работает только с Kubernetes 1.15. Необходимо добавить Feature Gate для kubelet `kubectl -n kube-system edit cm kubelet-config-1.15`:

```yaml
apiVersion: v1
data:
  kubelet: |
    featureGates:
      ExpandCSIVolumes: true
```

Опция в kubelet не подгружается динамически, поэтому нужно на каждой ноде будет сделать `kubeadm upgrade node && systemctl restart kubelet`.

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и vSphere API, перед увеличением PVC нужно заскейлить Deployment или StatefulSet в 0, после этого увеличить PVC, убедившись в успешном увеличении через `kubectl get pvc -o yaml` (в Status размер должен быть равен Spec, и не должно быть никаких conditions).

## Требования к окружениям

1. Требования к версии vSphere: `v6.7U2`.
2. vCenter, до которого есть доступ изнутри кластера с master нод.
3. Создать Datacenter, а в нём:

    1. VirtualMachine template со [специальным](https://github.com/vmware/cloud-init-vmware-guestinfo) cloud-init datasource внутри.
        * Подготовить образ Ubuntu 18.04, например, можно с помощью [скрипта](install-kubernetes/vsphere/prepare-template).
    2. Network, доступная на всех ESXi, на которых будут создаваться VirtualMachines.
    3. Datastore (или несколько), подключённый ко всем ESXi, на которых будут создаваться VirtualMachines.
        * На CluDatastore-ы **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по-умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**. Все Cluster'а из конкретной зоны должны иметь доступ ко всем Datastore'ам, с идентичной зоной.
    4. Cluster, в который добавить необходимые используемые ESXi.
        * На Cluster **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по-умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**.
    5. Folder для создаваемых VirtualMachines.
        * Опциональный. По-умолчанию будет использоваться root vm папка.
    6. Создать роль с необходимым [набором](#Список-привилегий-для-использования-модуля) прав.
    7. Создать пользователя, привязав к нему роль из пункта #6.

4. На созданный Datacenter **необходимо** "повесить" тэг из категории тэгов, указанный в `regionTagCategory` (по-умолчанию, `k8s-region`). Этот тэг будет обозначать **регион**.
5. Настроенная(-ые) Kubernetes master ноды. [Пример](install-kubernetes/common/ansible/kubernetes/tasks/master.yml) настройки ОС для master'а через kubeadm. Для созданных vSphere VirtualMachine прописать extraConfig согласно [инструкции](modules/030-cloud-provider-vsphere/docs/csi/disk_uuid.md).

## Как мне поднять кластер

1. Настройте инфраструктурное окружение в соответствии с [требованиями](#требования-к-окружениям) к окружению.
2. [Установите](#включение-модуля) deckhouse с помощью `install.sh`, передав флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами](#параметры) модуля.
3. [Создайте](#VsphereInstanceClass-custom-resource) один или несколько `VsphereInstanceClass`
4. Управляйте количеством и процессом заказа машин в облаке с помощью модуля [cloud-instance-manager](modules/040-cloud-instance-manager).

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
