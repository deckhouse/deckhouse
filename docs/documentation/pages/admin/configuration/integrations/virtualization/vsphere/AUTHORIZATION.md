---
title: Connection and authorization in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/authorization.html
---

## Requirements

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
     - The role must include the required [set of privileges](#list-of-required-privileges).
  1. User:
     - The user must be assigned the role specified in the previous item.
- A tag from the category specified in the [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) parameter must be assigned to the created Datacenter (default: `k8s-region`). This tag defines the region.

### VM image requirements

To create a VM template (Template), it is recommended to use a ready-made cloud image/OVA file provided by the OS vendor:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (section *Generic Cloud / OpenStack*)

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

### Preparing the virtual machine image

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

## vSphere configuration

You can prepare vSphere infrastructure for DKP via the **vSphere Client** web interface or the **govc** CLI utility. Below is the vSphere Client setup; the alternative approach is in [Configuration with govc](#configuration-with-govc).

{% alert level="info" %}
Administrator privileges on vCenter (the **Administrator** role) are required. For more on the vSphere authorization model, see the [official VMware documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/vsphere-permissions-and-user-management-tasks/understanding-authorization-in-vsphere.html).
{% endalert %}

### Configuration in vSphere Client

#### Region and zone model

VMware vSphere does not have built-in "region" and availability zone concepts like public clouds. DKP maps them to vSphere inventory objects using [tags](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vcenter-and-host-management/vsphere-tags-and-attributes-host-management/vsphere-tags-host-management.html):

| DKP concept | vSphere object | Configuration parameter | Tag category (default) |
|-------------|----------------|------------------------|------------------------|
| Region | Datacenter | [`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region) | `k8s-region` ([`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory)) |
| Zone | Cluster (Compute Cluster) | entry in [`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones) | `k8s-zone` ([`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory)) |

Tag names you create in vSphere (for example, `moscow` and `zone-a`) must match the `region` and `zones` values in the DKP cluster configuration.

#### Preparing vSphere inventory

Before creating tags and roles, ensure the target Datacenter has the following inventory objects:

1. **Datacenter** — container for all DKP cluster resources.
1. **Cluster** — one or more Compute Clusters with connected ESXi hosts. Each Cluster that will host nodes becomes a separate "zone".
1. **Networks** — distributed port groups or standard switch port groups available on all ESXi hosts in the target Clusters. The network must provide DHCP and internet access for cluster nodes.
1. **Datastore** — shared datastore connected to **all** ESXi hosts in the zone. Required for VM root disks and dynamic PVC provisioning (see [Storage](storage.html)).
1. **VM folder** — folder in **Hosts and Clusters** → **VMs and Templates** where DKP will create cluster VMs ([`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath)).
1. **VM template** — prepared OS image with `cloud-init` (see [Virtual machine image requirements](#virtual-machine-image-requirements)).

{% alert %}
Only one DKP cluster can be placed in a single folder (`vmFolderPath`). Create a separate folder for each new cluster.
{% endalert %}

##### Creating a VM folder for the cluster

1. In vSphere Client, go to **Menu** → **Hosts and Clusters**.
1. Select the Datacenter → right-click **VMs and Templates** (or an existing folder inside it).
1. Choose **New Folder** → **New Virtual Machine and Template Folder**.
1. Enter a folder name (for example, `k8s-prod`). This path is used in `vmFolderPath` (for example, `k8s-prod` or `parent/k8s-prod`).

##### Verifying Datastore availability

1. Go to **Menu** → **Storage**.
1. Select the target Datastore.
1. On the **Hosts** tab, verify the Datastore is connected to all ESXi hosts in the Cluster where nodes will be placed.
1. On the **Summary** tab, check available free space.

If a Datastore is not visible on one of the hosts in a zone, it cannot be used for dynamic PVC provisioning in that zone.

#### Creating tag categories {#creating-tags-and-tag-categories-in-vsphere-client}

A [tag category](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vcenter-and-host-management/vsphere-tags-and-attributes-host-management/vsphere-tags-host-management.html) defines which object types tags can be applied to and how many tags from the category are allowed per object.

1. Open vSphere Client and go to **Menu** → **Tags & Custom Attributes** → **Tags**.

   ![Creating tags and tag categories, step 1](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-1.png)

1. On the **Categories** tab, click **NEW**.

   ![Creating tags and tag categories, step 2](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-2.png)

1. Create a category for regions. Fill in the fields:

   | Field | Value for region category |
   |-------|---------------------------|
   | **Category Name** | `k8s-region` (or the name from `regionTagCategory`) |
   | **Description** | Kubernetes Region |
   | **Tags Per Object** | **One Tag** — only one tag from this category per object |
   | **Associable Object Types** | **Datacenter** |

   Click **Create**.

1. Click **NEW** again and create a category for zones:

   | Field | Value for zone category |
   |-------|-------------------------|
   | **Category Name** | `k8s-zone` (or the name from `zoneTagCategory`) |
   | **Description** | Kubernetes Zone |
   | **Tags Per Object** | **One Tag** |
   | **Associable Object Types** | **Cluster Compute Resource**, **Host**, **Datastore** |

   ![Creating tags and tag categories, step 3](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-3.png)

   Click **Create**.

{% alert level="info" %}
According to [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vcenter-and-host-management/vsphere-tags-and-attributes-host-management/vsphere-tags-host-management.html), after creating a category with **One Tag**, you can change it to **Many Tags**, but not the reverse. If you initially select **All Objects**, you cannot restrict object types later — set the required types when creating the category.
{% endalert %}

#### Creating tags

1. On the **Tags** tab (in the same **Tags & Custom Attributes** section), click **NEW**.
1. Create a region tag:
   - **Name** — region name to be used in the DKP `region` parameter (for example, `moscow`);
   - **Description** — optional description;
   - **Category** — `k8s-region`.
1. Create zone tags — one per Cluster where nodes will be placed (for example, `zone-a`, `zone-b`). Category — `k8s-zone`.

   ![Creating tags and tag categories, step 4](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-4.png)

Tag names must match the `region` and `zones` values in [`VsphereClusterConfiguration`](/modules/cloud-provider-vsphere/cluster_configuration.html).

#### Assigning tags to inventory objects

[Tag assignment](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/assign-or-remove-a-tag.html) is done via the object context menu in the inventory tree:

1. In **Menu** → **Hosts and Clusters**, select an object.
1. Click **Actions** → **Tags and Custom Attributes** → **Assign Tag**.
1. Select a tag from the list and click **Assign**.

Assign tags to the following objects:

| Object | Region tag | Zone tag |
|--------|------------|----------|
| Datacenter | Yes | — |
| Cluster (zone) | — | Yes |
| Datastore (in zone) | Yes | Yes (same as Cluster) |

![Creating tags and tag categories, step 5.1](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-5-1.png)
![Creating tags and tag categories, step 5.2](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-5-2.png)

Example for two zones:

- Datacenter `DC1` → tag `moscow` (category `k8s-region`);
- Cluster `Cluster-A` → tag `zone-a` (category `k8s-zone`);
- Cluster `Cluster-B` → tag `zone-b` (category `k8s-zone`);
- Datastore `lun-01` (available in `Cluster-A`) → tags `moscow` + `zone-a`;
- Datastore `lun-02` (available in `Cluster-B`) → tags `moscow` + `zone-b`.

{% alert level="warning" %}
All Clusters within the same zone must have access to all Datastores tagged with that zone. A Datastore without a zone tag will not be discovered by `cloud-data-discoverer` and will not appear as a StorageClass in the cluster.
{% endalert %}

#### Configuring Datastore {#configuring-datastore-in-vsphere-client}

{% alert level="warning" %}
For dynamic PersistentVolume provisioning, a Datastore must be available on **every** ESXi host in the zone (shared datastore).
{% endalert %}

1. Go to **Menu** → **Storage**.
1. Select the Datastore.
1. On the **Hosts** tab, verify the Datastore is connected to all hosts in the target Cluster.
1. Assign region and zone tags: **Actions** → **Tags and Custom Attributes** → **Assign Tag**.

   ![Creating tags and tag categories, step 6](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-6.png)

For cluster storage details, see [Storage and load balancing](storage.html).

#### Creating a custom role {#creating-and-assigning-a-role-in-vsphere-client}

DKP requires a [set of privileges](#list-of-required-privileges) not present in standard vSphere roles. Create a custom role following the [VMware instructions](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/vsphere-permissions-and-user-management-tasks/using-roles-to-assign-privileges.html):

1. Go to **Menu** → **Administration** → **Access Control** → **Roles**.

   ![Creating and assigning a role, step 1](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-1.png)

1. Click **NEW**.
1. Enter a role name (for example, `deckhouse`).
1. In the privilege category list, select the required privileges from the [table](#list-of-required-privileges). Use the **Show selected** / **Show all** filters in the role creation dialog for convenience.

   Main privilege categories for DKP:

   | UI category | What to enable |
   |-------------|----------------|
   | **Cns** | Searchable |
   | **Datastore** | Allocate space, Browse datastore, Low level file operations |
   | **Folder** | Create folder, Delete folder, Move folder, Rename folder |
   | **Global** | Global tag, System tag |
   | **vSphere Tagging** | All privileges listed in the table |
   | **Network** | Assign network |
   | **Resource** | All resource pool operations |
   | **VM Storage Policies** | View VM storage policies |
   | **vApp** | All privileges listed in the table |
   | **Virtual Machine** | Change Configuration, Edit Inventory, Guest Operations, Interaction, Provisioning, Snapshot Management — all listed in the table |

   ![Creating and assigning a role, step 2](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-2.png)

1. Click **Create**.

{% alert level="info" %}
Alternatively, clone an existing role (**Clone**), then edit it (**Edit**) and add missing privileges. You can view privileges for any role on the **Privileges** tab in the **Roles** section.
{% endalert %}

#### Assigning permissions to the DKP account

Permissions are assigned as a "user/group + role" pair on an inventory object ([vSphere authorization model](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/vsphere-permissions-and-user-management-tasks/understanding-authorization-in-vsphere.html)). For DKP, start with **Global Permissions** — the simplest option for initial setup.

##### Global Permissions (recommended for initial setup)

1. Go to **Menu** → **Administration** → **Access Control** → **Global Permissions**.
1. Click **ADD**.
1. In the **Add Permission** dialog:
   - **Domain** — select the SSO domain (for example, `vsphere.local`);
   - **User/Group** — find and select the DKP account (for example, `deckhouse@vsphere.local`);
   - **Role** — select the created `deckhouse` role;
   - **Propagate to children** — leave enabled so permissions apply to all inventory objects.
1. Click **OK**.

   ![Creating and assigning a role, step 3](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-3.png)

{% alert level="warning" %}
Specify the username with the SSO domain, for example `deckhouse@vsphere.local`. The format depends on the configured identity source (Active Directory, LDAP, `vsphere.local`).
{% endalert %}

##### Object-level permissions (alternative)

Instead of Global Permissions, you can assign the `deckhouse` role to specific inventory objects with **Propagate to children** enabled. This reduces the privilege level of the account — see [Granular permission model](#granular-permission-model).

To assign permissions on an object:

1. In the inventory tree, select an object (Datacenter, Cluster, VM folder, Datastore).
1. Go to the **Permissions** tab.
1. Click **Add** → select the user, role, and enable **Propagate to children** if needed.

{% alert %}
Steps for creating a user in the vSphere SSO provider (Active Directory, LDAP, `vsphere.local`) depend on your infrastructure and are not covered here. The user must be created in the identity source connected to vCenter Single Sign-On before assigning permissions.
{% endalert %}

#### Verifying the configuration

After setup, verify that:

- [ ] The Datacenter has a region tag;
- [ ] each target Cluster has a zone tag;
- [ ] shared Datastores have region and zone tags;
- [ ] the VM folder for the cluster exists;
- [ ] the VM template is prepared and contains a single disk;
- [ ] the DKP account has a role with the [required privileges](#list-of-required-privileges);
- [ ] vCenter is reachable from the network where cluster master nodes will be placed.

### Configuration with govc

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

#### Verifying network permissions with govc

The [`Network.Assign`](#list-of-required-privileges) privilege is required to attach port groups to virtual machines during provisioning. With the [granular permission model](#granular-permission-model), this privilege must be assigned on **each** port group specified in [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) and [`additionalNetworks`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-additionalnetworks), or inherited from a parent object with inheritance enabled.

Verify that the service account has the required permissions on the target network:

```shell
export GOVC_URL="https://<VCENTER_FQDN>/sdk"
export GOVC_USERNAME="<USERNAME@DOMAIN.LOCAL>"
export GOVC_PASSWORD="<PASSWORD>"
export GOVC_INSECURE=true

govc permissions.ls -r "/<DatacenterName>/network/<NetworkName>"
```

Path examples:

- Port group at the root of the datacenter Networks section: `/<DatacenterName>/network/net3-k8s`;
- Port group in an inventory folder: `/<DatacenterName>/network/k8s-networks/PROD NET`;
- Port group on a Distributed Switch: `/<DatacenterName>/network/<DVSName>/<PortGroupName>`.

For network names containing spaces, use the name as-is in the path without extra escaping. In YAML configuration, quote such names (see [Network parameters](layout.html#network-parameters)).

The command output must show a role for the DKP account that includes the `Network.Assign` privilege. The `-r` (`--recursive`) flag displays permissions inherited from parent objects.

{% alert level="warning" %}
Missing `Network.Assign` on a port group often manifests as a VM creation error. For CloudEphemeral nodes (provisioned by machine-controller-manager), verify permissions on networks from [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), not only on master node networks.
{% endalert %}

#### Creating and assigning a role with govc

{% alert %}
We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

The role described below includes the privileges from the [list of required privileges](#list-of-required-privileges) section. For a more granular permission model, see [Granular permission model](#granular-permission-model).
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

## List of required privileges

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
      <td>
        Attaching networks (port groups) to virtual machines during template cloning.
        The privilege must be assigned on each port group from <code>mainNetwork</code> and <code>additionalNetworks</code>.
        With the granular permission model, see <a href="#verifying-network-permissions-with-govc">govc verification</a>.
      </td>
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

## Granular permission model

Instead of assigning the `deckhouse` role via **Global Permissions** on the entire vCenter, you can limit the scope by assigning roles on specific inventory objects. This reduces the privilege level of the DKP service account.

Recommended assignment scheme:

| vSphere object | Access level | Inheritance | Purpose |
|----------------|--------------|-------------|---------|
| vCenter (root) | Read-only | No | Inventory overview, tag operations |
| Datacenter | Read-only | No | Access to datacenter objects |
| Cluster (zone) | Full access (`deckhouse` role) | Yes | Resource pool creation, VM placement |
| VM folder ([`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath)) | Full access (`deckhouse` role) | Yes | Creating, deleting, and managing cluster VMs |
| Datastore folder (CNS volumes) | Full access (`deckhouse` role) | Yes | PersistentVolume provisioning via CSI |
| Distributed Switch | Read-only | No | Viewing network infrastructure |
| Distributed Port Group | Full access | Yes | Connecting networks to VMs |

{% alert level="warning" %}
A granular permission model requires careful configuration and testing. If you see access errors in `cloud-controller-manager`, `cloud-data-discoverer`, or CSI driver logs, verify that the account has the required privileges on all affected objects.
{% endalert %}

{% alert %}
With the granular model, the role with the full set of privileges from the [table above](#list-of-required-privileges) is assigned only to objects where DKP performs write operations (Cluster, VM folder, Datastore folder, port group). On other objects, a Read-only role with tagging privileges (`InventoryService.Tagging.*`, `StorageProfile.View`, `System.*`) is sufficient.
{% endalert %}
