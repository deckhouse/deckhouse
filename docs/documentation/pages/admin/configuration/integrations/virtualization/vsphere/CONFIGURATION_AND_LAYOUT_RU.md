---
title: Схемы размещения и настройка VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/layout.html
lang: ru
---

## Standard

Схема Standard предназначена для размещения кластера внутри инфраструктуры vSphere с возможностью управления ресурсами, сетями и хранилищем.

Особенности:

- Использование vSphere Datacenter в качестве региона ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region));
- Использование vSphere Cluster в качестве зоны ([`zone`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones));
- Поддержка нескольких зон и размещения узлов по зонам;
- Использование различных datastore для дисков и volume’ов;
- Поддержка подключения сетей, включая дополнительную сетевую изоляцию (например, MetalLB + BGP).

![resources](../../../../images/cloud-provider-vsphere/vsphere-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11345&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
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
    mainNetwork: net3-k8s
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
      mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
  - ru-central1-a
  - ru-central1-b
```

Обязательные параметры [ресурса VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration):

- `region` — тег, присвоенный объекту Datacenter;
- `zoneTagCategory` и `regionTagCategory` — категории тегов, по которым распознаются регионы и зоны;
- `internalNetworkCIDR` — подсеть для назначения внутренних IP-адресов;
- `vmFolderPath` — путь до папки, в которой будут размещаться виртуальные машины кластера;
- `sshPublicKey` — публичный SSH-ключ для доступа к узлам;
- `zones` — список зон, доступных для размещения узлов.

{% alert level="info" %}
Все узлы, размещённые в разных зонах, должны иметь доступ к общим datastore с аналогичными тегами зоны.
{% endalert %}

## Список необходимых привилегий

{% alert level="info" %}
Подробнее о том, как создать и назначить роль пользователю можно ознакомиться в разделах[«Настройка через vSphere Client»](authorization.html#настройка-через-vsphere-client) и [«Настройка через govc»](authorization.html#настройка-через-govc).
{% endalert %}

**Детальный список привилегий, необходимых для работы Deckhouse Kubernetes Platform в vSphere:**

<table>
  <thead>
    <tr>
      <th>Категория привилегий в UI</th>
      <th>Список привилегий в UI</th>
      <th>Список привилегий в API</th>
      <th>Назначение привилегий в Deckhouse</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>—</td>
      <td>— (назначаются по умолчанию при создании роли)</td>
      <td>
        <code>System.Anonymous</code><br/>
        <code>System.Read</code><br/>
        <code>System.View</code>
      </td>
      <td>Базовый доступ к объектам vSphere Inventory, необходимый для работы всех компонентов интеграции Deckhouse с vSphere.</td>
    </tr>
    <tr>
      <td>Cns</td>
      <td>Searchable</td>
      <td><code>Cns.Searchable</code></td>
      <td>Поиск и сопоставление объектов Container Native Storage при работе CSI-драйвера с томами Kubernetes.</td>
    </tr>
    <tr>
      <td>Datastore</td>
      <td>
        Allocate space,<br/>
        Browse datastore,<br/>
        Low level file operations
      </td>
      <td>
        <code>Datastore.AllocateSpace</code><br/>
        <code>Datastore.Browse</code><br/>
        <code>Datastore.FileManagement</code>
      </td>
      <td>Выделение дисков при создании виртуальных машин и заказе <code>PersistentVolumes</code> в кластере.</td>
    </tr>
    <tr>
      <td>Folder</td>
      <td>
        Create folder,<br/>
        Delete folder,<br/>
        Move folder,<br/>
        Rename folder
      </td>
      <td>
        <code>Folder.Create</code><br/>
        <code>Folder.Delete</code><br/>
        <code>Folder.Move</code><br/>
        <code>Folder.Rename</code>
      </td>
      <td>Группировка кластера Deckhouse Kubernetes Platform в одном <code>Folder</code> в vSphere Inventory.</td>
    </tr>
    <tr>
      <td>Global</td>
      <td>
        Global tag,<br/>
        System tag
      </td>
      <td>
        <code>Global.GlobalTag</code><br/>
        <code>Global.SystemTag</code>
      </td>
      <td>Доступ к глобальным и системным тегам, используемым Deckhouse Kubernetes Platform при работе с объектами vSphere.</td>
    </tr>
    <tr>
      <td>vSphere Tagging</td>
      <td>
        Assign or Unassign vSphere Tag,<br/>
        Assign or Unassign vSphere Tag on Object,<br/>
        Create vSphere Tag,<br/>
        Create vSphere Tag Category,<br/>
        Delete vSphere Tag,<br/>
        Delete vSphere Tag Category,<br/>
        Edit vSphere Tag,<br/>
        Edit vSphere Tag Category,<br/>
        Modify UsedBy Field for Category,<br/>
        Modify UsedBy Field for Tag
      </td>
      <td>
        <code>InventoryService.Tagging.AttachTag</code><br/>
        <code>InventoryService.Tagging.ObjectAttachable</code><br/>
        <code>InventoryService.Tagging.CreateTag</code><br/>
        <code>InventoryService.Tagging.CreateCategory</code><br/>
        <code>InventoryService.Tagging.DeleteTag</code><br/>
        <code>InventoryService.Tagging.DeleteCategory</code><br/>
        <code>InventoryService.Tagging.EditTag</code><br/>
        <code>InventoryService.Tagging.EditCategory</code><br/>
        <code>InventoryService.Tagging.ModifyUsedByForCategory</code><br/>
        <code>InventoryService.Tagging.ModifyUsedByForTag</code>
      </td>
      <td>Deckhouse Kubernetes Platform использует теги для определения доступных ему объектов <code>Datacenter</code>, <code>Cluster</code> и <code>Datastore</code>, а также для определения виртуальных машин, находящихся под его управлением.</td>
    </tr>
    <tr>
      <td>Network</td>
      <td>Assign network</td>
      <td><code>Network.Assign</code></td>
      <td>Подключение сетей и port group к виртуальным машинам кластера Deckhouse Kubernetes Platform.</td>
    </tr>
    <tr>
      <td>Resource</td>
      <td>
        Assign virtual machine to resource pool,<br/>
        Create resource pool,<br/>
        Modify resource pool,<br/>
        Remove resource pool,<br/>
        Rename resource pool
      </td>
      <td>
        <code>Resource.AssignVMToPool</code><br/>
        <code>Resource.CreatePool</code><br/>
        <code>Resource.DeletePool</code><br/>
        <code>Resource.EditPool</code><br/>
        <code>Resource.RenamePool</code>
      </td>
      <td>Размещение виртуальных машин кластера Deckhouse Kubernetes Platform в целевом пуле ресурсов и управление этим пулом.</td>
    </tr>
    <tr>
      <td>VM Storage Policies (<em>Profile-driven Storage Privileges</em> в vSphere 7)</td>
      <td>View VM storage policies (<em>Profile-driven storage view</em> в vSphere 7)</td>
      <td><code>StorageProfile.View</code></td>
      <td>Просмотр политик хранения, используемых при создании виртуальных машин и динамическом заказе томов в кластере.</td>
    </tr>
    <tr>
      <td>vApp</td>
      <td>
        Add virtual machine,<br/>
        Assign resource pool,<br/>
        Create,<br/>
        Delete,<br/>
        Import,<br/>
        Power Off,<br/>
        Power On,<br/>
        View OVF Environment,<br/>
        vApp application configuration,<br/>
        vApp instance configuration,<br/>
        vApp resource configuration
      </td>
      <td>
        <code>VApp.ApplicationConfig</code><br/>
        <code>VApp.AssignResourcePool</code><br/>
        <code>VApp.AssignVM</code><br/>
        <code>VApp.Create</code><br/>
        <code>VApp.Delete</code><br/>
        <code>VApp.ExtractOvfEnvironment</code><br/>
        <code>VApp.Import</code><br/>
        <code>VApp.InstanceConfig</code><br/>
        <code>VApp.PowerOff</code><br/>
        <code>VApp.PowerOn</code><br/>
        <code>VApp.ResourceConfig</code>
      </td>
      <td>Управление операциями, связанными с развертыванием и конфигурацией vApp и OVF-шаблонов, используемых при создании виртуальных машин.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Change Configuration</td>
      <td>
        Add existing disk,<br/>
        Add new disk,<br/>
        Add or remove device,<br/>
        Advanced configuration,<br/>
        Set annotation,<br/>
        Change CPU count,<br/>
        Toggle disk change tracking,<br/>
        Extend virtual disk,<br/>
        Acquire disk lease,<br/>
        Modify device settings,<br/>
        Configure managedBy,<br/>
        Change Memory,<br/>
        Query unowned files,<br/>
        Configure Raw device,<br/>
        Reload from path,<br/>
        Remove disk,<br/>
        Rename,<br/>
        Reset guest information,<br/>
        Change resource,<br/>
        Change Settings,<br/>
        Change Swapfile placement,<br/>
        Upgrade virtual machine compatibility
      </td>
      <td>
        <code>VirtualMachine.Config.AddExistingDisk</code><br/>
        <code>VirtualMachine.Config.AddNewDisk</code><br/>
        <code>VirtualMachine.Config.AddRemoveDevice</code><br/>
        <code>VirtualMachine.Config.AdvancedConfig</code><br/>
        <code>VirtualMachine.Config.Annotation</code><br/>
        <code>VirtualMachine.Config.CPUCount</code><br/>
        <code>VirtualMachine.Config.ChangeTracking</code><br/>
        <code>VirtualMachine.Config.DiskExtend</code><br/>
        <code>VirtualMachine.Config.DiskLease</code><br/>
        <code>VirtualMachine.Config.EditDevice</code><br/>
        <code>VirtualMachine.Config.ManagedBy</code><br/>
        <code>VirtualMachine.Config.Memory</code><br/>
        <code>VirtualMachine.Config.QueryUnownedFiles</code><br/>
        <code>VirtualMachine.Config.RawDevice</code><br/>
        <code>VirtualMachine.Config.ReloadFromPath</code><br/>
        <code>VirtualMachine.Config.RemoveDisk</code><br/>
        <code>VirtualMachine.Config.Rename</code><br/>
        <code>VirtualMachine.Config.ResetGuestInfo</code><br/>
        <code>VirtualMachine.Config.Resource</code><br/>
        <code>VirtualMachine.Config.Settings</code><br/>
        <code>VirtualMachine.Config.SwapPlacement</code><br/>
        <code>VirtualMachine.Config.UpgradeVirtualHardware</code>
      </td>
      <td>Управление жизненным циклом виртуальных машин кластера Deckhouse Kubernetes Platform.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Edit Inventory</td>
      <td>
        Create new,<br/>
        Create from existing,<br/>
        Remove,<br/>
        Move
      </td>
      <td>
        <code>VirtualMachine.Inventory.Create</code><br/>
        <code>VirtualMachine.Inventory.CreateFromExisting</code><br/>
        <code>VirtualMachine.Inventory.Delete</code><br/>
        <code>VirtualMachine.Inventory.Move</code>
      </td>
      <td>Создание, удаление и перемещение виртуальных машин кластера Deckhouse Kubernetes Platform в инвентаре vSphere.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Guest Operations</td>
      <td>Guest Operation Queries</td>
      <td><code>VirtualMachine.GuestOperations.Query</code></td>
      <td>Получение информации из гостевой операционной системы виртуальных машин.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Interaction</td>
      <td>
        Answer question,<br/>
        Device connection,<br/>
        Guest operating system management by VIX API,<br/>
        Power Off,<br/>
        Power On,<br/>
        Reset,<br/>
        Configure CD media,<br/>
        Install VMware Tools
      </td>
      <td>
        <code>VirtualMachine.Interact.AnswerQuestion</code><br/>
        <code>VirtualMachine.Interact.DeviceConnection</code><br/>
        <code>VirtualMachine.Interact.GuestControl</code><br/>
        <code>VirtualMachine.Interact.PowerOff</code><br/>
        <code>VirtualMachine.Interact.PowerOn</code><br/>
        <code>VirtualMachine.Interact.Reset</code><br/>
        <code>VirtualMachine.Interact.SetCDMedia</code><br/>
        <code>VirtualMachine.Interact.ToolsInstall</code>
      </td>
      <td>Управление состоянием виртуальных машин, подключением устройств и взаимодействием с гостевой операционной системой.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Provisioning</td>
      <td>
        Clone virtual machine,<br/>
        Customize guest,<br/>
        Deploy template,<br/>
        Allow virtual machine download,<br/>
        Allow virtual machine files upload,<br/>
        Read customization specifications
      </td>
      <td>
        <code>VirtualMachine.Provisioning.Clone</code><br/>
        <code>VirtualMachine.Provisioning.Customize</code><br/>
        <code>VirtualMachine.Provisioning.DeployTemplate</code><br/>
        <code>VirtualMachine.Provisioning.GetVmFiles</code><br/>
        <code>VirtualMachine.Provisioning.PutVmFiles</code><br/>
        <code>VirtualMachine.Provisioning.ReadCustSpecs</code>
      </td>
      <td>Клонирование шаблонов виртуальных машин, их настройка и развертывание при создании узлов кластера Deckhouse Kubernetes Platform.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Snapshot Management</td>
      <td>
        Create snapshot,<br/>
        Remove Snapshot,<br/>
        Rename Snapshot
      </td>
      <td>
        <code>VirtualMachine.State.CreateSnapshot</code><br/>
        <code>VirtualMachine.State.RemoveSnapshot</code><br/>
        <code>VirtualMachine.State.RenameSnapshot</code>
      </td>
      <td>Управление снимками виртуальных машин и томов в сценариях, где эта функциональность используется компонентами платформы.</td>
    </tr>
  </tbody>
</table>
