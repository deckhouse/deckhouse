---
title: Connection and authorization in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/authorization.html
---

## Requirements

For the proper operation of the Deckhouse Kubernetes Platform with VMware vSphere, the following are required:

- Access to vCenter;
- A user with required set of privileges;
- Created tags and tag categories in vSphere;
- Networks with DHCP and internet access;
- Available shared datastores on all ESXi hosts.

* vSphere version: `7.x` or `8.x` with support for the [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) mechanism.
* vCenter: must be accessible from within the cluster from the master nodes.
* A created Datacenter containing:
  1. Virtual Machine template.
     * The VM image must use `Virtual machines with hardware version 15 or later` (required for online resize to work).
     * The following packages must be installed: `open-vm-tools`, `cloud-init`, and [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if using `cloud-init` version lower than 21.3).
  2. Network.
     * Must be available on all ESXi hosts where VMs will be created.
  3. Datastore (one or more).
     * Must be connected to all ESXi hosts where VMs will be created.
     * **Required**: assign a tag from the tag categories specified in [zoneTagCategory](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) (default: `k8s-zone`). This tag will designate a **zone**. All clusters in a specific zone must have access to all datastores with the same zone tag.
  4. Cluster.
     * Contains the ESXi hosts to be used.
     * **Required**: assign a tag from the tag categories specified in [zoneTagCategory](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) (default: `k8s-zone`). This tag will designate a **zone**.
  5. Folder for created VMs.
     * Optional (root VM folder is used by default).
  6. Role.
     * Must include the required [set of privileges](/modules/cloud-provider-vsphere/environment.html#list-of-required-privileges).
  7. User.
     * Assigned the role from item 6.
* The created Datacenter **must** be assigned a tag from the tag category specified in [regionTagCategory](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) (default: `k8s-region`). This tag will designate a **region**.

### VM image requirements

To create a VM template (`Template`), it is recommended to use a ready-made cloud image/OVA file provided by the OS vendor:

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

### Configuration via vSphere Client

#### Creating tags and tag categories

VMware vSphere does not have the concepts of “region” and “zone”. In vSphere, a “region” is represented by a `Datacenter`, and a “zone” by a `Cluster`. Tags are used to establish this mapping.

1. Open vSphere Client and navigate to **Menu** → **Tags & Custom Attributes** → **Tags**.

   ![Creating tags and tag categories, step 1](images/tags-categories-setup/Screenshot-1.png)

2. Open the **Categories** tab and click **NEW**. Create a category for regions (for example, `k8s-region`): set **Tags Per Object** to **One tag** and specify the associated object types, including **Datacenter**.

   ![Creating tags and tag categories, step 2](images/tags-categories-setup/Screenshot-2.png)

3. Create a second category for zones (for example, `k8s-zone`) with **Host**, **Cluster**, and **Datastore** as object types.

   ![Creating tags and tag categories, step 3](images/tags-categories-setup/Screenshot-3.png)

4. Go to the **Tags** tab and create at least one tag in the region category and one tag in the zone category (for example, `test-region`, `test-zone-1`).

   ![Creating tags and tag categories, step 4](images/tags-categories-setup/Screenshot-4.png)

5. In the **Inventory** tab, select the target **Datacenter**, open the **Summary** panel, then choose **Actions** → **Tags & Custom Attributes** → **Assign Tag** and assign the region tag.
   Repeat this for each **Cluster** where nodes will be placed, assigning the corresponding zone tags.

   ![Creating tags and tag categories, step 5.1](images/tags-categories-setup/Screenshot-5-1.png)
   ![Creating tags and tag categories, step 5.2](images/tags-categories-setup/Screenshot-5-2.png)

##### Datastore configuration

{% alert level="warning" %}
For dynamic `PersistentVolume` provisioning, a `Datastore` must be available on **each** ESXi host (shared datastore).
{% endalert %}

In the **Inventory** tab, select the **Datastore**, open the **Summary** panel, then choose **Actions** → **Tags & Custom Attributes** → **Assign Tag** and assign the same region tag as on the corresponding **Datacenter**, and the same zone tag as on the corresponding **Cluster**.

![Creating tags and tag categories, step 6](images/tags-categories-setup/Screenshot-6.png)

#### Creating and assigning a role

1. Navigate to **Menu** → **Administration** → **Access Control** → **Roles**.

   ![Creating and assigning a role, step 1](images/role-setup/Screenshot-1.png)

2. Click **NEW**, enter a role name (for example, `deckhouse`), and add the privileges from the [list](#list-of-required-privileges).

   ![Creating and assigning a role, step 2](images/role-setup/Screenshot-2.png)

3. Assign the role to the Deckhouse account: in **Menu** → **Administration** → **Access Control** → **Global Permissions**, click **ADD** and select the user and the `deckhouse` role.

   ![Creating and assigning a role, step 3](images/role-setup/Screenshot-3.png)

### Configuration via govc

#### Installing govc

You'll need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) — to proceed with the rest of the guide.

After the installation is complete, set the environment variables required to work with vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

#### Creating tags and tag categories

Instead of "regions" and "zones", VMware vSphere provides `Datacenter` and `Cluster` objects. We will use tags to match them with "regions"/"zones". These tags fall into two categories: one for "regions" tags and the other for "zones" tags.

Create a tag category using the following commands:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Create tags in each category. If you intend to use multiple "zones" (`Cluster`), create a tag for each one of them:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Attach the "region" tag to `Datacenter`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Attach "zone" tags to the `Cluster` objects:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

##### Datastore configuration

{% alert level="warning" %}
For dynamic `PersistentVolume` provisioning, a `Datastore` must be available on **each** ESXi host (shared datastore).
{% endalert %}

Assign the "region" and "zone" tags to the `Datastore` objects to automatically create a `StorageClass` in the Kubernetes cluster:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

#### Creating and assigning a role

{% alert %}
We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

The role described below includes the privileges from the [List of required privileges](#list-of-required-privileges) section. If you need a more granular Role, please contact your Deckhouse support.
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

Assign the role to a user on the `vCenter` object:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
For more detailed permission configuration, refer to the [official documentation](https://pkg.go.dev/github.com/vmware/govmomi).
{% endalert %}
