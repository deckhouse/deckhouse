---
title: "Cloud provider — VMware vSphere: Preparing environment"
description: "Configuring VMware vSphere for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## Environment requirements

The following prerequisites must be met for Deckhouse Kubernetes Platform to work correctly with VMware vSphere:

- Access to vCenter;
- A user account with the required set of privileges;
- Tags and tag categories created in vSphere;
- Networks with DHCP and Internet access;
- Shared Datastore resources available on all ESXi hosts in use;
- vSphere version `7.x` or `8.x` with support for [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion);
- vCenter accessible from inside the cluster from the master nodes;
- A configured Datacenter that includes the following objects:
  1. VirtualMachine template:
     - The virtual machine image must use `Virtual machines with hardware version 15 or later` — this is required for online resize support.
     - The image must include the `open-vm-tools`, `cloud-init`, and [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) packages if the `cloud-init` version is lower than `21.3`.
  1. Network:
     - The network must be available on all ESXi hosts where virtual machines are planned to be created.
  1. Datastore (one or more):
     - The Datastore must be connected to all ESXi hosts where virtual machines are planned to be created.
     - A tag from the category specified in the [`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) parameter must be assigned to the Datastore (default: `k8s-zone`). This tag defines the zone.
     - All Cluster objects within the same zone must have access to all Datastore objects in that zone.
  1. Cluster:
     - All ESXi hosts in use must be added to the Cluster.
     - A tag from the category specified in the [`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) parameter must be assigned to the Cluster (default: `k8s-zone`). This tag defines the zone.
  1. Folder for the virtual machines being created:
     - You do not need to create the folder in advance. The installer creates it at the path from the [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) parameter.
     - If the folder already exists, set [`vmFolderExists: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists), otherwise the installation fails with the `The name '<FOLDER>' already exists` error.
     - More than one cluster cannot be placed in the same folder.
  1. Role:
     - The role must include the required [set of privileges](/modules/cloud-provider-vsphere/environment.html#list-of-required-privileges).
  1. User:
     - The user must be assigned the role specified in the previous item.
- A tag from the category specified in the [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) parameter must be assigned to the created Datacenter (default: `k8s-region`). This tag defines the region.

## List of required privileges

The role for the platform account includes the privileges listed below. They are grouped by the operations that the platform performs in vSphere.

To create the role and assign it to a user, refer to [Creating and assigning a role in vSphere Client](#creating-and-assigning-a-role-in-vsphere-client) and [Creating and assigning a role with govc](#creating-and-assigning-a-role-with-govc).

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
| Create folder | `Folder.Create` | Creating the folder at the path from the [`vmFolderPath`](cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) parameter |
| Delete folder | `Folder.Delete` | Deleting that folder together with the cluster |
| Move folder | `Folder.Move` | Moving the folder when the path changes |
| Rename folder | `Folder.Rename` | Renaming the folder when the path changes |
| Assign virtual machine to resource pool | `Resource.AssignVMToPool` | Placing virtual machines in a resource pool |
| Create resource pool | `Resource.CreatePool` | Creating a nested resource pool in every zone |
| Modify resource pool | `Resource.EditPool` | Changing the settings of that pool |
| Remove resource pool | `Resource.DeletePool` | Deleting the pool together with the cluster |
| Rename resource pool | `Resource.RenamePool` | Renaming the pool |
| Assign network | `Network.Assign` | Connecting virtual machines to the networks from the [`mainNetwork`](cr.html#vsphereinstanceclass-v1-spec-mainnetwork) and [`additionalNetworks`](cr.html#vsphereinstanceclass-v1-spec-additionalnetworks) parameters |

### Creating virtual machines

Virtual machines are created by cloning a prepared template and are registered in the vSphere inventory.

| Privilege in UI | Privilege in API | Purpose in DKP |
| --- | --- | --- |
| Clone virtual machine | `VirtualMachine.Provisioning.Clone` | Cloning the template from the [`template`](cr.html#vsphereinstanceclass-v1-spec-template) parameter |
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
| Extend virtual disk | `VirtualMachine.Config.DiskExtend` | Growing the disk to the size from the [`rootDiskSize`](cr.html#vsphereinstanceclass-v1-spec-rootdisksize) parameter and expanding volumes |
| Acquire disk lease | `VirtualMachine.Config.DiskLease` | Acquiring a disk lease for the duration of operations with it |
| Toggle disk change tracking | `VirtualMachine.Config.ChangeTracking` | Managing changed block tracking for a disk |
| Configure Raw device | `VirtualMachine.Config.RawDevice` | Configuring raw device mappings (RDM) |
| Change CPU count | `VirtualMachine.Config.CPUCount` | Setting the number of vCPUs from the [`numCPUs`](cr.html#vsphereinstanceclass-v1-spec-numcpus) parameter |
| Change Memory | `VirtualMachine.Config.Memory` | Setting the memory size from the [`memory`](cr.html#vsphereinstanceclass-v1-spec-memory) parameter |
| Change resource | `VirtualMachine.Config.Resource` | Reserving memory from the [`memoryReservation`](cr.html#vsphereinstanceclass-v1-spec-runtimeoptions-memoryreservation) parameter and limiting resources |
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

## vSphere configuration

### Configuration in vSphere Client

#### Creating tags and tag categories in vSphere Client

VMware vSphere does not have built-in concepts of a "region" or a "zone". In vSphere, a Datacenter acts as a region, and a Cluster acts as a zone. Tags are used to establish this mapping.

1. Open vSphere Client and go to "Menu" → "Tags & Custom Attributes" → "Tags".

   ![Creating tags and tag categories, step 1](images/tags-categories-setup/menu-tags-and-custom-attributes.png)

1. Open the "Categories" tab and click "NEW". Create a category for regions (for example, `k8s-region`): set "Tags Per Object" to "One tag" and specify the applicable object types, including Datacenter.

   ![Creating tags and tag categories, step 2](images/tags-categories-setup/create-category-region.png)

1. Create a second category for zones (for example, `k8s-zone`) with the object types Host, Cluster, and Datastore.

   ![Creating tags and tag categories, step 3](images/tags-categories-setup/create-category-zone.png)

1. Go to the "Tags" tab and create at least one tag in the region category and one tag in the zone category (for example, `test-region`, `test-zone-1`).

   ![Creating tags and tag categories, step 4](images/tags-categories-setup/tags-list.png)

1. In the "Inventory" tab, select the target Datacenter, open the "Summary" panel, then choose "Actions" → "Tags & Custom Attributes" → "Assign Tag" and assign the region tag.
   Repeat this step for each Cluster that will host nodes, assigning the appropriate zone tags.

   ![Creating tags and tag categories, step 5.1](images/tags-categories-setup/datacenter-actions-assign-tag.png)
   ![Creating tags and tag categories, step 5.2](images/tags-categories-setup/assign-tag-to-datacenter.png)

#### Configuring Datastore in vSphere Client

{% alert level="warning" %}
For dynamic provisioning of PersistentVolume, the Datastore must be available on **every** ESXi host in the zone (shared datastore).
{% endalert %}

In the "Inventory" tab, select the Datastore, open the "Summary" panel, then choose "Actions" → "Tags & Custom Attributes" → "Assign Tag". Assign the Datastore the same region tag as the corresponding Datacenter, and the same zone tag as the corresponding Cluster.

![Creating tags and tag categories, step 6](images/tags-categories-setup/assign-tags-to-datastore.png)

To make sure the Datastore is connected to every ESXi host in the zone, open "Menu" → "Inventory" → "Storage", select the Datastore, and go to the "Hosts" tab. The list shows the hosts that have access to the Datastore.

![Checking Datastore availability](images/datastore-setup/datastore-hosts.png)

#### Creating a virtual machine folder in vSphere Client

The installer creates the virtual machine folder at the path from the [`vmFolderPath`](cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) parameter. If the folder already exists, set [`vmFolderExists: true`](cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists).

To create the folder in advance, open "Menu" → "Inventory" → "Hosts and Clusters", select a Datacenter object in the list, then open "ACTIONS" → "New Folder" → "New VM and Template Folder..." and enter the name.

![Creating a virtual machine folder](images/vm-folder-setup/new-vm-and-template-folder.png)

#### Creating a resource pool in vSphere Client

In every zone, the installer creates a resource pool named after the [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of the ClusterConfiguration resource. If the [`baseResourcePool`](cluster_configuration.html#vsphereclusterconfiguration-baseresourcepool) parameter is set, the pool is created inside the pool it points to. The parent pool must exist, the installer does not create it. The same applies to the pool from the [`resourcePool`](cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-resourcepool) parameter of a node group.

To create a pool, open "Menu" → "Inventory" → "Hosts and Clusters", select a Cluster object in the list, then open "ACTIONS" → "New Resource Pool..." and enter the name.

![Creating a resource pool](images/resource-pool-setup/new-resource-pool.png)

With [`useNestedResourcePool: false`](cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool), the installer does not create a nested pool. Virtual machines are placed in the pool from the `resourcePool` parameter. If that parameter is not set, the machines are placed in the root pool of the Cluster.

#### Creating and assigning a role in vSphere Client

1. Open "Menu" → "Administration". A separate vCenter management page opens, where the "Roles" item is in the left pane under "Access Control".

   ![Creating and assigning a role, step 1](images/role-setup/access-control-roles.png)

1. Click "NEW", enter a role name (for example, `deckhouse`), and add the privileges from the [list](#list-of-required-privileges).

   ![Creating and assigning a role, step 2](images/role-setup/new-role-privileges.png)

1. Assign the role to the Deckhouse service account: go to "Menu" → "Administration" → "Access Control" → "Global Permissions", click "ADD", and select the user and the `deckhouse` role.

   ![Creating and assigning a role, step 3](images/role-setup/add-global-permission.png)

### Configuration via govc

#### Installing govc

To continue configuring vSphere, install the [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) CLI utility.

After installation, set the environment variables required to connect to `vCenter`.

{% alert level="warning" %}
Make sure to specify the username together with the domain, for example: `username@domain.local`.
{% endalert %}

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<USERNAME>@vsphere.local
export GOVC_PASSWORD=<PASSWORD>
export GOVC_INSECURE=1
```

#### Creating tags and tag categories with govc

VMware vSphere does not have built-in concepts of a "region" or a "zone". In vSphere, a Datacenter acts as a region, and a Cluster acts as a zone. Tags are used to establish this mapping.

Create tag categories with the following commands:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Create tags in each category. If you intend to use multiple "zones" (Cluster), create a tag for each one of them:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Attach the "region" tag to Datacenter:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>
```

Attach "zone" tags to the Cluster objects:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/host/<CLUSTER_NAME_1>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/host/<CLUSTER_NAME_2>
```

#### Datastore configuration with govc

{% alert level="warning" %}
For dynamic PersistentVolume provisioning, a Datastore must be available on **each** ESXi host (shared datastore).
{% endalert %}

Assign the "region" and "zone" tags to the Datastore objects to automatically create a StorageClass in the Kubernetes cluster:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>

govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
```

#### Creating and assigning a role with govc

{% alert %}
We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

The role described below includes the privileges from [the list of required privileges](#list-of-required-privileges) section. If you need a more granular Role, please contact your Deckhouse support.
{% endalert %}

Create a role with the required privileges:

```shell
govc role.create deckhouse \
    Cns.Searchable \
    Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
    Folder.Create Folder.Delete Folder.Move Folder.Rename \
    Global.GlobalTag Global.SystemTag \
    InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory \
    InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory \
    InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory \
    InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory \
    InventoryService.Tagging.ModifyUsedByForTag InventoryService.Tagging.ObjectAttachable \
    Network.Assign \
    Resource.AssignVMToPool Resource.CreatePool Resource.DeletePool Resource.EditPool Resource.RenamePool \
    StorageProfile.View \
    System.Anonymous System.Read System.View \
    VApp.ApplicationConfig VApp.AssignResourcePool VApp.AssignVM VApp.Create VApp.Delete \
    VApp.ExtractOvfEnvironment VApp.Import VApp.InstanceConfig VApp.PowerOff VApp.PowerOn VApp.ResourceConfig \
    VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice \
    VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount \
    VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease \
    VirtualMachine.Config.EditDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory \
    VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath \
    VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo \
    VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement \
    VirtualMachine.Config.UpgradeVirtualHardware \
    VirtualMachine.GuestOperations.Query \
    VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.DeviceConnection \
    VirtualMachine.Interact.GuestControl VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn \
    VirtualMachine.Interact.Reset VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.ToolsInstall \
    VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete \
    VirtualMachine.Inventory.Move \
    VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate \
    VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs \
    VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot
```

Assign the role to a user on the vCenter object.

{% alert level="warning" %}
Make sure to specify the username together with the domain, for example: `username@domain.local`.
{% endalert %}

```shell
govc permissions.set -principal <USERNAME>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
For a description of vSphere privileges, refer to the [VMware documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/defined-privileges.html).
{% endalert %}

#### Role assignment scope

The role is assigned on the vCenter root object rather than on the directory with the cluster virtual machines. The platform components access objects outside that directory:

- The CSI driver determines volume topology by the ESXi hosts attached to a Datastore, so it accesses Cluster and Host objects.
- The resource discovery component searches for CNS disks across the whole vCenter, and it needs the `Cns.Searchable` privilege from the [Storage](#storage) group for that.
- The installer creates a resource pool in the Cluster object and a directory in the Datacenter, which needs the privileges from the [Virtual machine placement](#virtual-machine-placement) group.

If you limit the role to the virtual machine directory, these operations fail.

#### Checking privileges

The CSI driver checks the account privileges on each Datastore and excludes those where the privileges are insufficient. Such a Datastore does not appear in the list of available ones, and ordering a PersistentVolume through the corresponding StorageClass fails.

If a tagged Datastore does not produce a working StorageClass, check the account privileges on that object:

```shell
govc permissions.ls /<DATACENTER_NAME>/datastore/<DATASTORE_NAME>
```

Account privileges on an object can also be viewed in vSphere Client. Select the object, open the "Permissions" tab, and find the account in the list. The "Defined In" column shows the object the permission is defined on. The "Global Permission" value means the permission is assigned globally.

![Viewing permissions on an object](images/permissions-check/datastore-permissions.png)

### vCenter TLS certificate verification

DKP connects to vCenter over TLS and verifies its certificate. If the vCenter certificate is issued by a custom or enterprise certificate authority, pass the certificate chain of that authority in the [`caBundle`](cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle) parameter. Certificate verification stays enabled in this case.

Specify the chain in PEM format. It is the same setting, but its path depends on where the vCenter connection is described:

- When installing a cluster, the connection is described in the [`provider`](cluster_configuration.html#vsphereclusterconfiguration-provider) section of the [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) resource next to the [`provider.server`](cluster_configuration.html#vsphereclusterconfiguration-provider-server) parameter, so the chain is set in [`provider.caBundle`](cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle).
- In a running cluster, the connection is described at the top level of the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module settings next to the [`host`](configuration.html#parameters-host) parameter, so the chain is set in [`caBundle`](configuration.html#parameters-cabundle).

Example for a cluster being installed:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  caBundle: |
    -----BEGIN CERTIFICATE-----
    <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
    -----END CERTIFICATE-----
```

Example for a running cluster:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    host: "<VCENTER_FQDN>"
    username: "<USERNAME@DOMAIN.LOCAL>"
    password: "<PASSWORD>"
    caBundle: |
      -----BEGIN CERTIFICATE-----
      <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
      -----END CERTIFICATE-----
```

The [`insecure: true`](cluster_configuration.html#vsphereclusterconfiguration-provider-insecure) parameter disables vCenter certificate verification completely. In the settings of a running cluster, the same parameter is named [`insecure`](configuration.html#parameters-insecure). Set either `caBundle` or `insecure: true`. If both parameters are set, DKP rejects the configuration.

For the NSX-T connection, the certificate chain is set by the separate [`nsxt.caBundle`](cluster_configuration.html#vsphereclusterconfiguration-nsxt-cabundle) parameter. In the settings of a running cluster, these are the [`nsxt.caBundle`](configuration.html#parameters-nsxt-cabundle) and [`nsxt.insecureFlag`](configuration.html#parameters-nsxt-insecureflag) parameters. Set either `nsxt.caBundle` or [`nsxt.insecureFlag: true`](cluster_configuration.html#vsphereclusterconfiguration-nsxt-insecureflag), otherwise DKP rejects the configuration.

{% alert level="warning" %}
The [`csi-vsphere`](/modules/csi-vsphere/) module does not support the `caBundle` parameter and connects to vCenter either with certificate verification against the system certificate authorities or with the [`insecure`](/modules/csi-vsphere/configuration.html#parameters-insecure) parameter.
{% endalert %}

### VM image requirements

To create a VM template (`Template`), it is recommended to use a ready-made cloud image/OVA file provided by the OS vendor:

- [**Ubuntu**](https://cloud-images.ubuntu.com/)
- [**Debian**](https://cloud.debian.org/images/cloud/)
- [**CentOS**](https://cloud.centos.org/)
- [**Rocky Linux**](https://rockylinux.org/alternative-images/) (section *Generic Cloud / OpenStack*)

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

#### Preparing the virtual machine image

{% alert level="warning" %}
Disable VMware Guest OS Customization (and any vApp/OS customization mechanisms, if applicable in your setup) for the template and the cluster virtual machines in vSphere. DKP performs the initial node configuration via `cloud-init` (VMware GuestInfo datasource). Enabled customization can conflict with `cloud-init` and lead to incorrect node initialization.
{% endalert %}

1. Install the required packages:

   If you use `cloud-init` version lower than 21.3 (VMware GuestInfo support is required):

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init cloud-init-vmware-guestinfo
   ```

   If you use `cloud-init` version 21.3 or higher:

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init
   ```

1. Verify that the `disable_vmware_customization: false` parameter is set in `/etc/cloud/cloud.cfg`.

1. Make sure the `default_user` parameter is specified in `/etc/cloud/cloud.cfg`. It is required to add an SSH key when the VM starts.

1. Add the VMware GuestInfo datasource — create `/etc/cloud/cloud.cfg.d/99-DataSourceVMwareGuestInfo.cfg`:

   ```yaml
   datasource:
     VMware:
       vmware_cust_file_max_wait: 10
   ```

1. Before creating the VM template, reset the `cloud-init` identifiers and state using the following commands:

   ```shell
   truncate -s 0 /etc/machine-id &&
   rm /var/lib/dbus/machine-id &&
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Clear `cloud-init` event logs:

   ```shell
   cloud-init clean --logs --seed
   ```

{% alert level="warning" %}

After the virtual machine starts, the following services related to the packages installed during `cloud-init` preparation must be running:

- `cloud-config.service`,
- `cloud-final.service`,
- `cloud-init.service`.

To ensure that the services are enabled, use the command:

```shell
systemctl is-enabled cloud-config.service cloud-init.service cloud-final.service
```

Example output for enabled services:

```console
enabled
enabled
enabled
```

{% endalert %}

{% alert %}
DKP creates VM disks of type `eagerZeroedThick`, but the type of disks of created VMs may be changed without notification according to the `VM Storage Policy` settings in vSphere.
For more details, see the [documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-single-host-management-vmware-host-client-8-0/virtual-machine-management-with-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/configuring-virtual-machines-in-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/virtual-disk-configuration-vSphereSingleHostManagementVMwareHostClient/about-virtual-disk-provisioning-policies-vSphereSingleHostManagementVMwareHostClient.html).
{% endalert %}

{% alert %}
DKP uses the `ens192` interface as the default interface for VMs in vSphere. Therefore, when using static IP addresses in [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork), you must create an interface named `ens192` in the OS image as the default interface.
{% endalert %}

#### Preparing the template in vSphere Client

Once the image is ready, do the following in vSphere Client:

1. Import the operating system image. Select a Datacenter or a Cluster, open "ACTIONS" → "Deploy OVF Template..." and specify the OVA file provided by the OS vendor.

   ![Preparing the template, step 1](images/vm-template-setup/deploy-ovf-template.png)

1. Check the parameters of the resulting virtual machine. On the "Summary" tab, the "Compatibility" row of the "VM Hardware" panel shows the hardware version, and the device list shows the attached disks. Hardware version 15 or later and a single disk are required.

   ![Preparing the template, step 2](images/vm-template-setup/vm-hardware-compatibility.png)

1. If the hardware version is lower than 15, upgrade it. Power off the virtual machine and open "ACTIONS" → "Compatibility" → "Upgrade VM Compatibility...". The item is unavailable for a running machine. To schedule the upgrade for the next power off, use "Schedule VM Compatibility Upgrade...".

   ![Preparing the template, step 3](images/vm-template-setup/upgrade-vm-compatibility.png)

1. Convert the virtual machine to a template. Open "ACTIONS" → "Template" → "Convert to Template". The `template` parameter accepts both a template and a powered off virtual machine.

   ![Preparing the template, step 4](images/vm-template-setup/convert-to-template.png)

## Infrastructure

### Connecting to vCenter

Where the connection settings come from depends on where the cluster control plane runs. If the control plane runs in the cloud, the vCenter address and credentials are set in the [`provider`](cluster_configuration.html#vsphereclusterconfiguration-provider) section of the VsphereClusterConfiguration resource. If the control plane runs on virtual machines or bare metal, the same data is set in the [`host`](configuration.html#parameters-host), [`username`](configuration.html#parameters-username), and [`password`](configuration.html#parameters-password) parameters of the module.

The platform connects to vCenter over TLS and verifies the certificate. Passing the certificate authority chain is covered in the [vCenter TLS certificate verification](#vcenter-tls-certificate-verification) section.

### Networking

Cluster nodes connect to the network whose path is set in the [`mainNetwork`](cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetwork) parameter. Through this network, nodes reach vCenter, the container image registry, and each other.

The network must meet the following requirements:

- It is available on every ESXi host where virtual machines are created.
- It provides DHCP unless node addresses are set statically.
- vCenter and the container image registry are reachable from it.

#### Node addressing

By default, nodes get their addresses over DHCP. For nodes created by the installer, addresses are set statically in the [`mainNetworkIPAddresses`](cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetworkipaddresses) parameter. Static addressing is not supported for the nodes ordered through the [VsphereInstanceClass](cr.html#vsphereinstanceclass) resource, such nodes get their addresses over DHCP.

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

#### Multiple networks

Secondary interfaces of virtual machines connect to the networks listed in the [`additionalNetworks`](cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-additionalnetworks) parameter. For example, an additional interface is used to connect the network for BGP traffic.

When there are several networks, specify which of them the platform treats as internal and which as external. Based on the network names from the [`internalNetworkNames`](cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) and [`externalNetworkNames`](cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) parameters, the `vsphere-cloud-controller-manager` component sets the InternalIP and ExternalIP addresses in the Node object. These parameters take the name of the network, not its path.

The [`internalNetworkCIDR`](cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) parameter sets the subnet that master nodes get their addresses from in the internal network. Addresses are allocated starting with the tenth one. The parameter is required if the configuration contains the [`nodeGroups`](cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section.

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

### Inbound traffic

Inbound traffic is balanced in one of three ways.

1. **Through an external load balancer.** A load balancer that already exists in the infrastructure directs traffic to the cluster frontend nodes. All that is required from vSphere is a network in which the frontend nodes are reachable by the load balancer. In the cluster, the traffic is accepted by the Ingress controller of the [`ingress-nginx`](/modules/ingress-nginx/) module.

1. **Through MetalLB.** This way suits an environment without an external load balancer. MetalLB assigns addresses to services of the LoadBalancer type and works in the L2 and BGP modes. The network requirements, the editions the modes are available in, and the configuration are covered in the [`metallb` module documentation](/modules/metallb/). On the vSphere side, the BGP mode requires the following:

   - A dedicated network in which the frontend nodes exchange traffic with the BGP routers.
   - A second network interface on the frontend nodes, connected to that network by the [`additionalNetworks`](cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-additionalnetworks) parameter.
   - DHCP in that network. The platform sets an address on the additional interface only for master nodes, from the [`internalNetworkCIDR`](cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) subnet starting with the tenth address. For the nodes of a node group, the platform does not assign an address in that network.
   - IP addresses of the BGP routers, the autonomous system number (ASN) of the routers and the ASN of the cluster, and the range of addresses that the cluster announces.

1. **Through NSX-T.** If NSX-T is deployed in the infrastructure, `cloud-controller-manager` orders a load balancer in it for every service of the LoadBalancer type. The address pool name, the Tier-1 gateway path, and the credentials are set in the [`nsxt`](configuration.html#parameters-nsxt) section.

### Using the datastore

The cluster uses a Datastore for two purposes:

- Placing the root disks of virtual machines.
- Placing PersistentVolume volumes.

A single Datastore can serve both purposes. When planning the free space, take both the node disks and the volumes into account.

A Datastore must meet the following requirements:

- It is connected to every ESXi host of the zone.
- It is tagged with the region tag and the zone tag.
- It is available to the platform account. The CSI driver checks the privileges on every Datastore and excludes those where the privileges are insufficient.

#### Node disks

The Datastore for root disks is set in the [`datastore`](cr.html#vsphereinstanceclass-v1-spec-datastore) parameter of a node group, with the path relative to the Datacenter. The disk size is set in the [`rootDiskSize`](cr.html#vsphereinstanceclass-v1-spec-rootdisksize) parameter. For nodes created by the installer, the default size is 50 GiB. For nodes ordered through the [VsphereInstanceClass](cr.html#vsphereinstanceclass) resource, it is 20 GiB. A value smaller than the template disk size makes cloning fail.

An example of a node group with a separate Datastore and a larger root disk:

```yaml
nodeGroups:
- name: worker
  replicas: 2
  zones:
  - test-zone-1
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    mainNetwork: k8s-msk-178
    datastore: lun_1
    rootDiskSize: 50
```

The SPBM storage policy for node disks is set in the [`storagePolicyID`](cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) parameter, which takes the policy ID.

The ID can be viewed in vSphere Client. Open "Menu" → "Policies and Profiles" → "VM Storage Policies" and select the policy by name. In the browser address bar, find the `lvSelectedItemId` parameter. The ID is placed between `PbmRequirementStorageProfile:` and the first `%` character.

![List of storage policies](images/storage-policy-setup/vm-storage-policies.png)

The `govc` command also prints the policy ID:

```shell
govc storage.policy.ls "<POLICY_NAME>"
```

Replace `<POLICY_NAME>` with the policy name as shown in the "VM Storage Policies" list, for example `vSAN Default Storage Policy`.

#### PersistentVolume volumes

For every Datastore tagged with a zone tag, the platform creates a StorageClass. If SPBM storage policies are configured in vSphere, a StorageClass is additionally created for each combination of a Datastore and a policy. For a DatastoreCluster, StorageClasses are created only in the legacy mode, which is enabled by the [`compatibilityFlag`](configuration.html#parameters-storageclass-compatibilityflag) parameter.

The name of a StorageClass with a policy combines the Datastore name and the policy name. The platform converts it to lowercase, replaces spaces with hyphens, and removes the remaining characters except hyphens and dots. For example, the `lun_1` Datastore and the `Gold Policy` policy produce the `lun1-gold-policy` StorageClass.

For the platform to discover the policies, the vSphere account needs the `StorageProfile.View` privilege from the [list of required privileges](#list-of-required-privileges). The StorageClass set is generated automatically, so a policy is not set for an individual StorageClass manually.

To keep StorageClasses from being created for some of the Datastores, list them in the [`exclude`](configuration.html#parameters-storageclass-exclude) parameter:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    storageClass:
      exclude:
      - ".*-lun101-.*"
      - slow-lun103
```

The parameter takes names and regular expressions, each of which must match the Datastore name in full. A partial match is not taken into account. For example, for the `vsanDatastore` Datastore, the `vsan` expression excludes no StorageClass, while `vsan.*` excludes all of them. An exclusion removes both the base StorageClass and the StorageClasses with policies for that Datastore. The parameter does not exclude an individual StorageClass with a policy by its own name.

To set the default StorageClass, use the global [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) parameter.

#### CSI operation mode

By default, the storage subsystem uses CNS disks that support resizing without detaching the volume from the node (online resize). The legacy mode with FCD disks is also supported, in which resizing without detaching the volume is unavailable. The mode is selected by the [`compatibilityFlag`](configuration.html#parameters-storageclass-compatibilityflag) parameter.

#### Expanding a PersistentVolumeClaim

To expand a volume, change the requested size in the PersistentVolumeClaim:

```shell
d8 k -n <NAMESPACE> patch pvc <PVC_NAME> -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
```

No further action is required. The platform expands the volume in vSphere, then kubelet expands the file system on the node the volume is attached to. The workload is not restarted.

While the expansion is in progress, the PersistentVolumeClaim status contains the `Resizing` and `FileSystemResizePending` conditions. Once kubelet has expanded the file system, both conditions are removed and the `status.capacity` field contains the new size. The command below prints the current volume size and the list of conditions:

```shell
d8 k -n <NAMESPACE> get pvc <PVC_NAME> \
  -o jsonpath='{.status.capacity.storage}{"\n"}{range .status.conditions[*]}{.type}={.status} {end}'
```

An example of the output for a volume whose expansion has finished. The first line is the size, and the second line is empty because no conditions are left in the status:

```console
2Gi

```

If the `Resizing` condition remains in the status, expanding without detaching the volume is unavailable in this configuration, for example in the legacy mode with FCD volumes. The limitation is described in an [external-resizer issue](https://github.com/kubernetes-csi/external-resizer/issues/44). To expand such a volume, detach it from the node:

1. Prevent new workloads from being scheduled on the node the volume is attached to:

   ```shell
   d8 k cordon <NODE_NAME>
   ```

   Replace `<NODE_NAME>` with the name of the node that runs the workload using the PersistentVolumeClaim.

1. Delete the workload that uses the PersistentVolumeClaim so that the volume is detached from the node.

1. Wait until the `Resizing` condition is removed from the PersistentVolumeClaim status.

1. Allow scheduling on the node again:

   ```shell
   d8 k uncordon <NODE_NAME>
   ```

#### Viewing volumes in vSphere Client

Volumes provisioned through CSI are shown in vSphere Client. Open "Menu" → "Inventory" → "Hosts and Clusters", select a Cluster object, go to the "Monitor" tab, and choose "Container Volumes" under "Cloud Native Storage". For every volume, the list shows the name, labels, Datastore, storage policy compliance ("Compliance Status"), availability ("Health Status"), and size.

![List of CNS volumes](images/cns-volumes/container-volumes.png)

The volume name in vSphere matches the PersistentVolume name in the cluster.

The icon to the left of the volume name opens the details panel. The "Kubernetes objects" tab shows the namespace, the PersistentVolumeClaim name and labels, and the workload that uses the volume.

![CNS volume details](images/cns-volumes/container-volume-details.png)
