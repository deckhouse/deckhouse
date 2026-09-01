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

## List of required privileges

The role for the platform account includes the privileges listed below. The privileges are grouped by the tasks the platform performs in vSphere.

To create the role and assign it to a user, refer to [Creating and assigning a role in vSphere Client](#creating-and-assigning-a-role-in-vsphere-client) and [Creating and assigning a role with govc](#creating-and-assigning-a-role-with-govc).

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

Assign the role on the vCenter root object, as shown in the commands above. DKP components need access to objects outside the directory with the cluster virtual machines:

- The CSI driver determines volume topology by the ESXi hosts attached to a Datastore, so it accesses Cluster and Host objects.
- The resource discovery component searches for CNS disks within vCenter, which uses the `Cns.Searchable` privilege.
- The installer creates a resource pool in the Cluster object and a directory in the Datacenter.

If you limit the role to the virtual machine directory, these operations fail.

#### Diagnosing missing privileges

The CSI driver checks the account privileges on each Datastore and excludes those where the privileges are insufficient. Such a Datastore does not appear in the list of available ones, and ordering a PersistentVolume through the corresponding StorageClass fails.

If a tagged Datastore does not produce a working StorageClass, check the account privileges on that object:

```shell
govc permissions.ls /<DATACENTER_NAME>/datastore/<DATASTORE_NAME>
```

### vCenter TLS certificate verification

DKP connects to vCenter over TLS and verifies its certificate. If the vCenter certificate is issued by a custom or enterprise certificate authority, pass the certificate chain of that authority in the `caBundle` parameter. Certificate verification stays enabled in this case.

Specify the chain in PEM format. Where you set it depends on how the cluster is created:

- When installing a cluster, use the [`provider.caBundle`](cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle) parameter of the VsphereClusterConfiguration resource.
- In a running cluster, use the [`caBundle`](configuration.html#parameters-cabundle) parameter of the module settings.

The `insecure: true` parameter disables vCenter certificate verification completely. Set either `caBundle` or `insecure: true`. DKP rejects a configuration that sets a non-empty `caBundle` and `insecure: true` at the same time.

For the NSX-T connection, the certificate chain is set by the separate `nsxt.caBundle` parameter, which is likewise incompatible with `nsxt.insecureFlag: true`.

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

The cluster requires a VLAN with DHCP and Internet access. The layout depends on the type of addresses in that VLAN:

- If the VLAN uses public addresses, create a second network for the cluster nodes. DHCP is not required in it.
- If the VLAN uses private addresses, the same network serves as the cluster node network.

### Inbound traffic

Inbound traffic can be balanced in two ways:

- Direct traffic to the cluster frontend nodes through an existing internal load balancer.
- Deploy MetalLB in BGP mode if there is no internal load balancer. The cluster frontend nodes get two interfaces, and the following is also required:

  - A dedicated VLAN for traffic exchange between BGP routers and MetalLB, with DHCP and Internet access.
  - IP addresses of the BGP routers.
  - Autonomous system number (ASN) on the BGP router.
  - Autonomous system number (ASN) in the cluster.
  - A range of addresses to announce.

### Using the datastore

The cluster can use several storage types at the same time. The minimum configuration includes:

- A Datastore where the cluster provisions PersistentVolumes.
- A Datastore where the root disks of the virtual machines are provisioned. It can be the same Datastore as for PersistentVolumes.
