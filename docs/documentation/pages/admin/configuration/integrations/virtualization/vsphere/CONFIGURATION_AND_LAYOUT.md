---
title: Layouts and configuration in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/layout.html
---

## Standard

The Standard layout is intended for deploying a cluster within the vSphere infrastructure
with full control over resources, networking, and storage.

Key features:

- Uses a vSphere Datacenter as a [`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region).
- Uses a vSphere Cluster as a [`zone`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones).
- Supports multiple zones and node placements across zones.
- Supports using different datastores for disks and volumes.
- Supports network connectivity including additional network isolation (for example, MetalLB + BGP).

![Standard layout in vSphere](../../../../images/cloud-provider-vsphere/vsphere-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11345&t=Qb5yyWumzPiTBtfL-0 --->

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
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

Required parameters for the [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) resource:

- `layout`: Placement layout name. Only `Standard` is supported.
- `provider`: vCenter connection parameters.
- `region`: Tag assigned to the Datacenter object.
- `zoneTagCategory` and `regionTagCategory`: Tag categories used to identify regions and zones.
- `zones`: List of zones available for node placement.
- `masterNodeGroup`: Parameters of the master node group.
- `vmFolderPath`: Path to the folder where cluster virtual machines will be placed.
- `sshPublicKey`: Public SSH key used to access the nodes.

The `internalNetworkCIDR` parameter is optional. DKP applies it only when `masterInstanceClass` defines `additionalNetworks`, and assigns master node addresses from this subnet starting with the tenth address. If the master node group uses a single network, omit the parameter.

{% alert level="info" %}
All nodes placed in different zones must have access to shared datastores with matching zone tags.
{% endalert %}

## List of required privileges

The role for the platform account includes the privileges listed below. The privileges are grouped by the tasks the platform performs in vSphere.

To create the role and assign it to a user, refer to [Creating and assigning a role in vSphere Client](authorization.html#creating-and-assigning-a-role-in-vsphere-client) and [Creating and assigning a role with govc](authorization.html#creating-and-assigning-a-role-with-govc).

### Basic access

vSphere assigns these privileges automatically when any role is created. They give the platform components read access to vSphere Inventory objects.

| Privilege in UI | Privilege in API |
| --- | --- |
| — | `System.Anonymous` |
| — | `System.Read` |
| — | `System.View` |

### Region and zone tags

The platform uses tags to identify the Datacenter, Cluster, and Datastore objects available to it, and to mark the virtual machines it manages.

| Privilege in UI | Privilege in API |
| --- | --- |
| Global tag | `Global.GlobalTag` |
| System tag | `Global.SystemTag` |
| Assign or Unassign vSphere Tag | `InventoryService.Tagging.AttachTag` |
| Assign or Unassign vSphere Tag on Object | `InventoryService.Tagging.ObjectAttachable` |
| Create vSphere Tag | `InventoryService.Tagging.CreateTag` |
| Create vSphere Tag Category | `InventoryService.Tagging.CreateCategory` |
| Delete vSphere Tag | `InventoryService.Tagging.DeleteTag` |
| Delete vSphere Tag Category | `InventoryService.Tagging.DeleteCategory` |
| Edit vSphere Tag | `InventoryService.Tagging.EditTag` |
| Edit vSphere Tag Category | `InventoryService.Tagging.EditCategory` |
| Modify UsedBy Field for Category | `InventoryService.Tagging.ModifyUsedByForCategory` |
| Modify UsedBy Field for Tag | `InventoryService.Tagging.ModifyUsedByForTag` |

### Storage

These privileges are required to place virtual machine disks, provision PersistentVolumes dynamically, and read SPBM storage policies. In vSphere 7, the `StorageProfile` category is named "Profile-driven storage".

| Privilege in UI | Privilege in API |
| --- | --- |
| Searchable | `Cns.Searchable` |
| Allocate space | `Datastore.AllocateSpace` |
| Browse datastore | `Datastore.Browse` |
| Low level file operations | `Datastore.FileManagement` |
| View VM storage policies | `StorageProfile.View` |

### Virtual machine placement

The platform groups the cluster virtual machines in a dedicated directory, places them in a resource pool, and connects them to networks.

| Privilege in UI | Privilege in API |
| --- | --- |
| Create folder | `Folder.Create` |
| Delete folder | `Folder.Delete` |
| Move folder | `Folder.Move` |
| Rename folder | `Folder.Rename` |
| Assign virtual machine to resource pool | `Resource.AssignVMToPool` |
| Create resource pool | `Resource.CreatePool` |
| Modify resource pool | `Resource.EditPool` |
| Remove resource pool | `Resource.DeletePool` |
| Rename resource pool | `Resource.RenamePool` |
| Assign network | `Network.Assign` |

### Creating virtual machines

Virtual machines are created by cloning a prepared template and are registered in the vSphere inventory.

| Privilege in UI | Privilege in API |
| --- | --- |
| Clone virtual machine | `VirtualMachine.Provisioning.Clone` |
| Deploy template | `VirtualMachine.Provisioning.DeployTemplate` |
| Customize guest | `VirtualMachine.Provisioning.Customize` |
| Read customization specifications | `VirtualMachine.Provisioning.ReadCustSpecs` |
| Allow virtual machine download | `VirtualMachine.Provisioning.GetVmFiles` |
| Allow virtual machine files upload | `VirtualMachine.Provisioning.PutVmFiles` |
| Create new | `VirtualMachine.Inventory.Create` |
| Create from existing | `VirtualMachine.Inventory.CreateFromExisting` |
| Remove | `VirtualMachine.Inventory.Delete` |
| Move | `VirtualMachine.Inventory.Move` |

### Configuring virtual machines

The platform sets the virtual machine parameters at creation time and changes them when a node group or an instance class is modified.

| Privilege in UI | Privilege in API |
| --- | --- |
| Add new disk | `VirtualMachine.Config.AddNewDisk` |
| Add existing disk | `VirtualMachine.Config.AddExistingDisk` |
| Remove disk | `VirtualMachine.Config.RemoveDisk` |
| Extend virtual disk | `VirtualMachine.Config.DiskExtend` |
| Acquire disk lease | `VirtualMachine.Config.DiskLease` |
| Toggle disk change tracking | `VirtualMachine.Config.ChangeTracking` |
| Configure Raw device | `VirtualMachine.Config.RawDevice` |
| Change CPU count | `VirtualMachine.Config.CPUCount` |
| Change Memory | `VirtualMachine.Config.Memory` |
| Change resource | `VirtualMachine.Config.Resource` |
| Change Swapfile placement | `VirtualMachine.Config.SwapPlacement` |
| Add or remove device | `VirtualMachine.Config.AddRemoveDevice` |
| Modify device settings | `VirtualMachine.Config.EditDevice` |
| Change Settings | `VirtualMachine.Config.Settings` |
| Advanced configuration | `VirtualMachine.Config.AdvancedConfig` |
| Set annotation | `VirtualMachine.Config.Annotation` |
| Rename | `VirtualMachine.Config.Rename` |
| Configure managedBy | `VirtualMachine.Config.ManagedBy` |
| Reset guest information | `VirtualMachine.Config.ResetGuestInfo` |
| Query unowned files | `VirtualMachine.Config.QueryUnownedFiles` |
| Reload from path | `VirtualMachine.Config.ReloadFromPath` |
| Upgrade virtual machine compatibility | `VirtualMachine.Config.UpgradeVirtualHardware` |

### Managing virtual machine state

These privileges are required to power virtual machines on and off, connect devices, read information from the guest operating system, and work with snapshots.

| Privilege in UI | Privilege in API |
| --- | --- |
| Power On | `VirtualMachine.Interact.PowerOn` |
| Power Off | `VirtualMachine.Interact.PowerOff` |
| Reset | `VirtualMachine.Interact.Reset` |
| Answer question | `VirtualMachine.Interact.AnswerQuestion` |
| Device connection | `VirtualMachine.Interact.DeviceConnection` |
| Configure CD media | `VirtualMachine.Interact.SetCDMedia` |
| Install VMware Tools | `VirtualMachine.Interact.ToolsInstall` |
| Guest operating system management by VIX API | `VirtualMachine.Interact.GuestControl` |
| Guest Operation Queries | `VirtualMachine.GuestOperations.Query` |
| Create snapshot | `VirtualMachine.State.CreateSnapshot` |
| Remove Snapshot | `VirtualMachine.State.RemoveSnapshot` |
| Rename Snapshot | `VirtualMachine.State.RenameSnapshot` |

### vApp

Operations with vApp and OVF templates. Required if the virtual machine templates or the machines themselves belong to a vApp.

| Privilege in UI | Privilege in API |
| --- | --- |
| Create | `VApp.Create` |
| Delete | `VApp.Delete` |
| Import | `VApp.Import` |
| Add virtual machine | `VApp.AssignVM` |
| Assign resource pool | `VApp.AssignResourcePool` |
| Power On | `VApp.PowerOn` |
| Power Off | `VApp.PowerOff` |
| vApp application configuration | `VApp.ApplicationConfig` |
| vApp instance configuration | `VApp.InstanceConfig` |
| vApp resource configuration | `VApp.ResourceConfig` |
| View OVF Environment | `VApp.ExtractOvfEnvironment` |
