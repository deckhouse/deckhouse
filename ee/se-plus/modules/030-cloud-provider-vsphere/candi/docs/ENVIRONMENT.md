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
     - This parameter is optional.
     - By default, the root virtual machine folder is used.
  1. Role:
     - The role must include the required [set of privileges](/modules/cloud-provider-vsphere/environment.html#list-of-required-privileges).
  1. User:
     - The user must be assigned the role specified in the previous item.
- A tag from the category specified in the [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) parameter must be assigned to the created Datacenter (default: `k8s-region`). This tag defines the region.

## List of required vSphere resources

* **User** with required [set of privileges](#list-of-required-privileges).
* **Network** with DHCP server and access to the Internet.
* **Datacenter** with a tag in [`k8s-region`](#creating-tags-and-tag-categories) category.
* **Cluster** with a tag in [`k8s-zone`](#creating-tags-and-tag-categories) category.
* **Datastore** with required [tags](#datastore-configuration).
* **Template** — [prepared](#preparing-a-virtual-machine-image) VM image.

## List of required privileges

> Read the [Configuration via vSphere Client](#configuration-via-vsphere-client) and [Configuration via govc](#configuration-via-govc) sections for details on how to create and assign a role to a user.

A detailed list of privileges required for Deckhouse Kubernetes Platform to work in vSphere:

<table>
  <thead>
    <tr>
      <th>Privilege category in UI</th>
      <th>Privileges in UI</th>
      <th>Privileges in API</th>
      <th>Purpose in Deckhouse</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>—</td>
      <td>— (assigned by default when creating a role)</td>
      <td>
        <code>System.Anonymous</code><br/>
        <code>System.Read</code><br/>
        <code>System.View</code>
      </td>
      <td>Basic access to vSphere Inventory objects required for all Deckhouse vSphere integration components.</td>
    </tr>
    <tr>
      <td>Cns</td>
      <td>Searchable</td>
      <td><code>Cns.Searchable</code></td>
      <td>Search and mapping of Container Native Storage objects when the CSI driver works with Kubernetes volumes.</td>
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
      <td>Disk provisioning when creating virtual machines and ordering <code>PersistentVolumes</code> in the cluster.</td>
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
      <td>Grouping a Deckhouse Kubernetes Platform cluster in a single <code>Folder</code> in vSphere Inventory.</td>
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
      <td>Access to global and system tags used by Deckhouse Kubernetes Platform when working with vSphere objects.</td>
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
      <td>Deckhouse Kubernetes Platform uses tags to identify the <code>Datacenter</code>, <code>Cluster</code>, and <code>Datastore</code> objects available to it, as well as to identify the virtual machines under its control.</td>
    </tr>
    <tr>
      <td>Network</td>
      <td>Assign network</td>
      <td><code>Network.Assign</code></td>
      <td>Connecting networks and port groups to Deckhouse Kubernetes Platform cluster virtual machines.</td>
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
      <td>Placement of Deckhouse Kubernetes Platform cluster virtual machines into the target resource pool and management of this pool.</td>
    </tr>
    <tr>
      <td>VM Storage Policies (<em>Profile-driven Storage Privileges</em> in vSphere 7)</td>
      <td>View VM storage policies (<em>Profile-driven storage view</em> in vSphere 7)</td>
      <td><code>StorageProfile.View</code></td>
      <td>Viewing storage policies used when creating virtual machines and dynamically provisioning volumes in the cluster.</td>
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
      <td>Managing operations related to deployment and configuration of vApp and OVF templates used when creating virtual machines.</td>
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
      <td>Managing the lifecycle of Deckhouse Kubernetes Platform cluster virtual machines.</td>
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
      <td>Creating, deleting, and moving Deckhouse Kubernetes Platform cluster virtual machines in vSphere Inventory.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Guest Operations</td>
      <td>Guest Operation Queries</td>
      <td><code>VirtualMachine.GuestOperations.Query</code></td>
      <td>Retrieving information from the guest operating system of virtual machines.</td>
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
      <td>Managing virtual machine power state, device connections, and interaction with the guest operating system.</td>
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
      <td>Cloning virtual machine templates, customizing them, and deploying them when creating Deckhouse Kubernetes Platform cluster nodes.</td>
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
      <td>Managing snapshots of virtual machines and volumes in scenarios where this functionality is used by platform components.</td>
    </tr>
  </tbody>
</table>

## vSphere configuration

### Configuration in vSphere Client

#### Creating tags and tag categories in vSphere Client

VMware vSphere does not have built-in concepts of a "region" or a "zone". In vSphere, a Datacenter acts as a region, and a Cluster acts as a zone. Tags are used to establish this mapping.

1. Open vSphere Client and go to "Menu" → "Tags & Custom Attributes" → "Tags".

   ![Creating tags and tag categories, step 1](images/tags-categories-setup/Screenshot-1.png)

1. Open the "Categories" tab and click "NEW". Create a category for regions (for example, `k8s-region`): set "Tags Per Object" to "One tag" and specify the applicable object types, including Datacenter.

   ![Creating tags and tag categories, step 2](images/tags-categories-setup/Screenshot-2.png)

1. Create a second category for zones (for example, `k8s-zone`) with the object types Host, Cluster, and Datastore.

   ![Creating tags and tag categories, step 3](images/tags-categories-setup/Screenshot-3.png)

1. Go to the "Tags" tab and create at least one tag in the region category and one tag in the zone category (for example, `test-region`, `test-zone-1`).

   ![Creating tags and tag categories, step 4](images/tags-categories-setup/Screenshot-4.png)

1. In the "Inventory" tab, select the target Datacenter, open the "Summary" panel, then choose "Actions" → "Tags & Custom Attributes" → "Assign Tag" and assign the region tag.
   Repeat this step for each Cluster that will host nodes, assigning the appropriate zone tags.

   ![Creating tags and tag categories, step 5.1](images/tags-categories-setup/Screenshot-5-1.png)
   ![Creating tags and tag categories, step 5.2](images/tags-categories-setup/Screenshot-5-2.png)

#### Configuring Datastore in vSphere Client

{% alert level="warning" %}
For dynamic provisioning of PersistentVolume, the Datastore must be available on **every** ESXi host in the zone (shared datastore).
{% endalert %}

In the "Inventory" tab, select the Datastore, open the "Summary" panel, then choose "Actions" → "Tags & Custom Attributes" → "Assign Tag". Assign the Datastore the same region tag as the corresponding Datacenter, and the same zone tag as the corresponding Cluster.

![Creating tags and tag categories, step 6](images/tags-categories-setup/Screenshot-6.png)

#### Creating and assigning a role in vSphere Client

1. Go to "Menu" → "Administration" → "Access Control" → "Roles".

   ![Creating and assigning a role, step 1](images/role-setup/Screenshot-1.png)

1. Click "NEW", enter a role name (for example, `deckhouse`), and add the privileges from the [list](#list-of-required-privileges).

   ![Creating and assigning a role, step 2](images/role-setup/Screenshot-2.png)

1. Assign the role to the Deckhouse service account: go to "Menu" → "Administration" → "Access Control" → "Global Permissions", click "ADD", and select the user and the `deckhouse` role.

   ![Creating and assigning a role, step 3](images/role-setup/Screenshot-3.png)

### Configuration via govc

#### Installing govc

To continue configuring vSphere, install the [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) CLI utility.

After installation, set the environment variables required to connect to `vCenter`.

{% alert level="warning" %}
Make sure to specify the username together with the domain, for example: `username@domain.local`.
{% endalert %}

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
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
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Attach "zone" tags to the Cluster objects:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

#### Datastore configuration with govc

{% alert level="warning" %}
For dynamic PersistentVolume provisioning, a Datastore must be available on **each** ESXi host (shared datastore).
{% endalert %}

Assign the "region" and "zone" tags to the Datastore objects to automatically create a StorageClass in the Kubernetes cluster:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
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
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
For more detailed permission configuration, refer to [the official documentation](https://pkg.go.dev/github.com/vmware/govmomi).
{% endalert %}

### VM image requirements

To create a VM template (`Template`), it is recommended to use a ready-made cloud image/OVA file provided by the OS vendor:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (section *Generic Cloud / OpenStack*)

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

#### Preparing the virtual machine image

{% alert level="warning" %}
Disable VMware Guest OS Customization (and any vApp/OS customization mechanisms, if applicable in your setup) for the template and the cluster virtual machines. DKP performs the initial node configuration via `cloud-init` (VMware GuestInfo datasource). Enabled customization can conflict with `cloud-init` and lead to incorrect node initialization.
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

## Infrastructure

### Networking

A VLAN with DHCP and Internet access is required for the running cluster:

* If the VLAN is public (public addresses), then you have to create a second network to deploy cluster nodes (DHCP is not needed in this network).
* If the VLAN is private (private addresses), then this network can be used for cluster nodes.

### Inbound traffic

* You can use an internal load balancer (if present) and direct traffic directly to the front nodes of the cluster.
* If there is no load balancer, you can use MetalLB in BGP mode to organize fault-tolerant load balancers (recommended). In this case, front nodes of the cluster will have two interfaces. For this, you will need:
  * A dedicated VLAN for traffic exchange between BGP routers and MetalLB. This VLAN must have DHCP and Internet access.
  * IP addresses of BGP routers.
  * ASN — the AS number on the BGP router.
  * ASN — the AS number in the cluster.
  * A range to announce addresses from.

### Using the datastore

Various types of storage can be used in the cluster; for the minimum configuration, you will need:
* Datastore for provisioning PersistentVolumes to the Kubernetes cluster.
* Datastore for provisioning root disks for the VMs (it can be the same Datastore as for PersistentVolume).
