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
internalNetworkCIDR: 192.168.199.0/24
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

The `internalNetworkCIDR` parameter is required if the configuration contains `nodeGroups`. The installer validates it when creating static nodes and fails without it. It is also required when `masterNodeGroup.instanceClass` defines `additionalNetworks`. In that case, DKP assigns master node addresses from this subnet starting with the tenth address. If there are no `nodeGroups` and the master nodes use a single network, omit the parameter.

{% alert level="info" %}
All nodes placed in different zones must have access to shared datastores with matching zone tags.
{% endalert %}

## Networking

Cluster nodes connect to the network whose path is set in the [`mainNetwork`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetwork) parameter. Through this network, nodes reach vCenter, the container image registry, and each other.

The network must meet the following requirements:

- It is available on every ESXi host where virtual machines are created.
- It provides DHCP unless node addresses are set statically.
- vCenter and the container image registry are reachable from it.

### Node addressing

By default, nodes get their addresses over DHCP. For nodes created by the installer, addresses are set statically in the [`mainNetworkIPAddresses`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetworkipaddresses) parameter. Static addressing is not supported for the nodes ordered through the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource, such nodes get their addresses over DHCP.

Addresses are assigned to the nodes of a group in order, so provide as many addresses as there are nodes in the group. If there are fewer addresses, the list is reused and the same address goes to several nodes.

An example of static addressing for three master nodes:

```yaml
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: k8s-msk-178
    mainNetworkIPAddresses:
    - address: 10.1.14.20/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.21/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.22/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
```

### Multiple networks

Secondary interfaces of virtual machines connect to the networks listed in the [`additionalNetworks`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-additionalnetworks) parameter. For example, an additional interface is used to connect the network for BGP traffic.

When there are several networks, specify which of them the platform treats as internal and which as external. Based on the network names from the [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) and [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) parameters, the `vsphere-cloud-controller-manager` component sets the InternalIP and ExternalIP addresses in the Node object. These parameters take the name of the network, not its path.

The [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) parameter sets the subnet that master nodes get their addresses from in the internal network. Addresses are allocated starting with the tenth one. The parameter is required if the configuration contains the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section.

An example of a configuration with two networks:

```yaml
externalNetworkNames:
- net3-k8s
internalNetworkNames:
- K8S_3
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: net3-k8s
    additionalNetworks:
    - K8S_3
```

## List of required privileges

The role for the platform account includes the privileges listed below. They are grouped by the operations that the platform performs in vSphere.

To create the role and assign it to a user, refer to [Creating and assigning a role in vSphere Client](authorization.html#creating-and-assigning-a-role-in-vsphere-client) and [Creating and assigning a role with govc](authorization.html#creating-and-assigning-a-role-with-govc).

### Basic access

vSphere assigns these privileges automatically when any role is created. They give the platform components read access to vSphere Inventory objects.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| — | `System.Anonymous` | Calling vCenter methods that require no authorization |
| — | `System.Read` | Reading the state and settings of objects |
| — | `System.View` | Viewing inventory objects |

### Region and zone tags

The platform uses tags to identify the Datacenter, Cluster, and Datastore objects available to it, and to mark the virtual machines it manages.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Global tag | `Global.GlobalTag` | Working with vCenter global tags |
| System tag | `Global.SystemTag` | Working with vCenter system tags |
| Assign or Unassign vSphere Tag | `InventoryService.Tagging.AttachTag` | Reading region and zone tags and tagging the cluster virtual machines |
| Assign or Unassign vSphere Tag on Object | `InventoryService.Tagging.ObjectAttachable` | Assigning a tag to a particular inventory object |
| Create vSphere Tag | `InventoryService.Tagging.CreateTag` | Creating the tags that mark the cluster virtual machines |
| Create vSphere Tag Category | `InventoryService.Tagging.CreateCategory` | Creating the `deckhouse-cluster-name` and `deckhouse-node-role` categories for those tags |
| Delete vSphere Tag | `InventoryService.Tagging.DeleteTag` | Deleting the tags created by the platform |
| Delete vSphere Tag Category | `InventoryService.Tagging.DeleteCategory` | Deleting the categories created by the platform |
| Edit vSphere Tag | `InventoryService.Tagging.EditTag` | Editing the tags created by the platform |
| Edit vSphere Tag Category | `InventoryService.Tagging.EditCategory` | Editing the categories created by the platform |
| Modify UsedBy Field for Category | `InventoryService.Tagging.ModifyUsedByForCategory` | Changing the internal UsedBy field of a category |
| Modify UsedBy Field for Tag | `InventoryService.Tagging.ModifyUsedByForTag` | Changing the internal UsedBy field of a tag |

### Storage

These privileges are required to place virtual machine disks, provision PersistentVolumes dynamically, and read SPBM storage policies.

{% alert level="info" %}
In vSphere 7, the `StorageProfile.View` privilege is located in the "Profile-driven storage" section of the interface.
{% endalert %}

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Searchable | `Cns.Searchable` | Searching for CNS disks across the whole vCenter during resource discovery |
| Allocate space | `Datastore.AllocateSpace` | Allocating space for node disks and PersistentVolume volumes |
| Browse datastore | `Datastore.Browse` | Browsing files on a Datastore |
| Low level file operations | `Datastore.FileManagement` | Operations with disk files on a Datastore |
| View VM storage policies | `StorageProfile.View` | Reading SPBM storage policies to create StorageClasses |

### Virtual machine placement

The platform groups the cluster virtual machines in a dedicated directory, places them in a resource pool, and connects them to networks.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Create folder | `Folder.Create` | Creating the folder at the path from the [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) parameter |
| Delete folder | `Folder.Delete` | Deleting that folder together with the cluster |
| Move folder | `Folder.Move` | Moving the folder when the path changes |
| Rename folder | `Folder.Rename` | Renaming the folder when the path changes |
| Assign virtual machine to resource pool | `Resource.AssignVMToPool` | Placing virtual machines in a resource pool |
| Create resource pool | `Resource.CreatePool` | Creating a nested resource pool in every zone |
| Modify resource pool | `Resource.EditPool` | Changing the settings of that pool |
| Remove resource pool | `Resource.DeletePool` | Deleting the pool together with the cluster |
| Rename resource pool | `Resource.RenamePool` | Renaming the pool |
| Assign network | `Network.Assign` | Connecting virtual machines to the networks from the [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) and [`additionalNetworks`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-additionalnetworks) parameters |

### Creating virtual machines

Virtual machines are created by cloning a prepared template and are registered in the vSphere inventory.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Clone virtual machine | `VirtualMachine.Provisioning.Clone` | Cloning the template from the [`template`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-template) parameter |
| Deploy template | `VirtualMachine.Provisioning.DeployTemplate` | Deploying a virtual machine from a template |
| Customize guest | `VirtualMachine.Provisioning.Customize` | Customizing the guest operating system during cloning |
| Read customization specifications | `VirtualMachine.Provisioning.ReadCustSpecs` | Reading guest operating system customization specifications |
| Allow virtual machine download | `VirtualMachine.Provisioning.GetVmFiles` | Reading virtual machine files |
| Allow virtual machine files upload | `VirtualMachine.Provisioning.PutVmFiles` | Writing virtual machine files |
| Create new | `VirtualMachine.Inventory.Create` | Creating a virtual machine in the inventory |
| Create from existing | `VirtualMachine.Inventory.CreateFromExisting` | Creating a virtual machine from an existing one |
| Remove | `VirtualMachine.Inventory.Delete` | Deleting a virtual machine when the number of nodes is reduced |
| Move | `VirtualMachine.Inventory.Move` | Moving a virtual machine to the cluster folder |

### Configuring virtual machines

The platform sets the virtual machine parameters at creation time and changes them when a node group or an instance class is modified.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Add new disk | `VirtualMachine.Config.AddNewDisk` | Creating the root disk of a virtual machine |
| Add existing disk | `VirtualMachine.Config.AddExistingDisk` | Attaching an existing disk |
| Remove disk | `VirtualMachine.Config.RemoveDisk` | Detaching a disk |
| Extend virtual disk | `VirtualMachine.Config.DiskExtend` | Growing the disk to the size from the [`rootDiskSize`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-rootdisksize) parameter and expanding volumes |
| Acquire disk lease | `VirtualMachine.Config.DiskLease` | Acquiring a disk lease for the duration of operations with it |
| Toggle disk change tracking | `VirtualMachine.Config.ChangeTracking` | Managing changed block tracking for a disk |
| Configure Raw device | `VirtualMachine.Config.RawDevice` | Configuring raw device mappings (RDM) |
| Change CPU count | `VirtualMachine.Config.CPUCount` | Setting the number of vCPUs from the [`numCPUs`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-numcpus) parameter |
| Change Memory | `VirtualMachine.Config.Memory` | Setting the memory size from the [`memory`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-memory) parameter |
| Change resource | `VirtualMachine.Config.Resource` | Reserving memory from the [`memoryReservation`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-runtimeoptions-memoryreservation) parameter and limiting resources |
| Change Swapfile placement | `VirtualMachine.Config.SwapPlacement` | Choosing the swap file location |
| Add or remove device | `VirtualMachine.Config.AddRemoveDevice` | Adding and removing devices, including network adapters |
| Modify device settings | `VirtualMachine.Config.EditDevice` | Changing device settings |
| Change Settings | `VirtualMachine.Config.Settings` | Changing general virtual machine settings |
| Advanced configuration | `VirtualMachine.Config.AdvancedConfig` | Passing the `cloud-init` configuration through `guestinfo` |
| Set annotation | `VirtualMachine.Config.Annotation` | Writing notes for a virtual machine |
| Rename | `VirtualMachine.Config.Rename` | Renaming a virtual machine |
| Configure managedBy | `VirtualMachine.Config.ManagedBy` | Marking a virtual machine as managed by the platform |
| Reset guest information | `VirtualMachine.Config.ResetGuestInfo` | Resetting the information received from the guest operating system |
| Query unowned files | `VirtualMachine.Config.QueryUnownedFiles` | Checking files that do not belong to the virtual machine |
| Reload from path | `VirtualMachine.Config.ReloadFromPath` | Reloading the virtual machine configuration from a file |
| Upgrade virtual machine compatibility | `VirtualMachine.Config.UpgradeVirtualHardware` | Upgrading the virtual machine hardware version |

### Managing virtual machine state

These privileges are required to power virtual machines on and off, connect devices, read information from the guest operating system, and work with snapshots. Snapshots are ordered if the [`snapshot-controller`](/modules/snapshot-controller/) module is enabled in the cluster.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Power On | `VirtualMachine.Interact.PowerOn` | Powering on a virtual machine |
| Power Off | `VirtualMachine.Interact.PowerOff` | Powering off a virtual machine |
| Reset | `VirtualMachine.Interact.Reset` | Resetting a virtual machine |
| Answer question | `VirtualMachine.Interact.AnswerQuestion` | Answering vSphere questions that block the machine |
| Device connection | `VirtualMachine.Interact.DeviceConnection` | Connecting and disconnecting devices of a running machine |
| Configure CD media | `VirtualMachine.Interact.SetCDMedia` | Attaching an image to the CD/DVD drive |
| Install VMware Tools | `VirtualMachine.Interact.ToolsInstall` | Installing VMware Tools |
| Guest operating system management by VIX API | `VirtualMachine.Interact.GuestControl` | Managing the guest operating system through the VIX API |
| Guest Operation Queries | `VirtualMachine.GuestOperations.Query` | Reading the state of the guest operating system |
| Create snapshot | `VirtualMachine.State.CreateSnapshot` | Creating a snapshot |
| Remove Snapshot | `VirtualMachine.State.RemoveSnapshot` | Removing a snapshot |
| Rename Snapshot | `VirtualMachine.State.RenameSnapshot` | Renaming a snapshot |

### vApp

Operations with vApp and OVF templates. Required if the virtual machine templates or the machines themselves belong to a vApp.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Create | `VApp.Create` | Creating a vApp |
| Delete | `VApp.Delete` | Deleting a vApp |
| Import | `VApp.Import` | Importing an OVF or OVA into a vApp |
| Add virtual machine | `VApp.AssignVM` | Adding a virtual machine to a vApp |
| Assign resource pool | `VApp.AssignResourcePool` | Assigning a resource pool to a vApp |
| Power On | `VApp.PowerOn` | Powering on a vApp |
| Power Off | `VApp.PowerOff` | Powering off a vApp |
| vApp application configuration | `VApp.ApplicationConfig` | Changing vApp application settings |
| vApp instance configuration | `VApp.InstanceConfig` | Changing vApp instance settings |
| vApp resource configuration | `VApp.ResourceConfig` | Changing vApp resource settings |
| View OVF Environment | `VApp.ExtractOvfEnvironment` | Reading the OVF environment of a virtual machine |
