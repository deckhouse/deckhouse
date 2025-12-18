---
title: "Cloud provider — VMware vSphere: Preparing environment"
description: "Configuring VMware vSphere for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## Environment requirements

* vSphere version: `7.x` or `8.x` with support for the [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) mechanism.
* vCenter to which master nodes can connect to from within the cluster.
* Datacenter with the following components:
  1. VirtualMachine template.
     * VM image should use `Virtual machines with hardware version 15 or later` (required for online resize to work).
     * The following packages must be installed in the VM image: `open-vm-tools`, `cloud-init` and [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if the `cloud-init` version lower than 21.3 is used).
  1. The network must be available on all ESXi where VirtualMachines will be created.
  1. One or more Datastores connected to all ESXi where VirtualMachines will be created.
     * A tag from the tag category in [`zoneTagCategory`](./configuration.html#parameters-zonetagcategory) (`k8s-zone` by default) **must be added** to Datastores. This tag will indicate the **zone**.  All Clusters of a specific zone must have access to all Datastores within the same zone.
  1. The cluster with the required ESXis.
     * A tag from the tag category in [`zoneTagCategory`](./configuration.html#parameters-zonetagcategory) (`k8s-zone` by default) **must be added** to the Cluster. This tag will indicate the **zone**.
  1. Folder for VirtualMachines to be created.
     * An optional parameter. By default, the root vm folder is used.
  1. Create a role with the appropriate [set](#list-of-required-privileges) of privileges.
  1. Create a user and assign the above role to it.
* A tag from the tag category in [`regionTagCategory`](./configuration.html#parameters-regiontagcategory) (`k8s-region` by default) **must be added** to the Datacenter. This tag will indicate the region.

## List of required vSphere resources

* **User** with required set of [permissions](#creating-and-assigning-a-role).
* **Network** with DHCP server and access to the Internet
* **Datacenter** with a tag in [`k8s-region`](#creating-tags-and-tag-categories) category.
* **Cluster** with a tag in [`k8s-zone`](#creating-tags-and-tag-categories) category.
* **Datastore** with required [tags](#datastore-configuration).
* **Template** — [prepared](#preparing-a-virtual-machine-image) VM image.

## List of required privileges

> Read [the documentation](environment.html#creating-and-assigning-a-role) on how to create and assign a role to a user.

A detailed list of privileges required for Deckhouse Kubernetes Platform to work in vSphere:

<table>
  <thead>
    <tr>
        <th>List of privileges</th>
        <th>Purpose</th>
    </tr>
  </thead>
  <tbody>
    <tr>
        <td><code>Cns.Searchable</code><br><code>StorageProfile.View</code><br><code>Datastore.AllocateSpace</code><br><code>Datastore.Browse</code><br><code>Datastore.FileManagement</code></td>
        <td>To provision disks when creating virtual machines and ordering <code>PersistentVolumes</code> in a cluster.</td>
    </tr>
    <tr>
        <td><code>Global.GlobalTag</code><br><code>Global.SystemTag</code><br><code>InventoryService.Tagging.AttachTag</code><br><code>InventoryService.Tagging.CreateCategory</code><br><code>InventoryService.Tagging.CreateTag</code><br><code>InventoryService.Tagging.DeleteCategory</code><br><code>InventoryService.Tagging.DeleteTag</code><br><code>InventoryService.Tagging.EditCategory</code><br><code>InventoryService.Tagging.EditTag</code><br><code>InventoryService.Tagging.ModifyUsedByForCategory</code><br><code>InventoryService.Tagging.ModifyUsedByForTag</code><br><code>InventoryService.Tagging.ObjectAttachable</code></td>
        <td>Deckhouse Kubernetes Platform uses tags to identify the <code>Datacenter</code>, <code>Cluster</code> and <code>Datastore</code> objects available to it, as well as, to identify the virtual machines under its control.</td>
    </tr>
    <tr>
        <td><code>Folder.Create</code><br><code>Folder.Delete</code><br><code>Folder.Move</code><br><code>Folder.Rename</code></td>
        <td>To group a Deckhouse Kubernetes Platform cluster in a single <code>Folder</code> in vSphere Inventory.</td>
    </tr>
<tr>
        <td><code>Host.Cim.CimInteraction</code></td>
        <td>Interaction with the Common Information Model (CIM) system for monitoring the status of server equipment through special interfaces without installing agents.</td>
    </tr>
    <tr>
        <td><code>Host.Config.AdvancedConfig</code><br><code>Host.Config.AuthenticationStore</code><br><code>Host.Config.PciPassthru</code><br><code>Host.Config.Snmp</code><br><code>Host.Config.DateTime</code><br><code>Host.Config.Settings</code><br><code>Host.Config.Connection</code><br><code>Host.Config.Firmware</code><br><code>Host.Config.GuestStore</code><br><code>Host.Config.HyperThreading</code><br><code>Host.Config.Image</code><br><code>Host.Config.Maintenance</code><br><code>Host.Config.Memory</code><br><code>Host.Config.Nvdimm</code><br><code>Host.Config.Network</code><br><code>Host.Config.Power</code><br><code>Host.Config.ProductLocker</code><br><code>Host.Config.Quarantine</code><br><code>Host.Config.Patch</code><br><code>Host.Config.NetService</code><br><code>Host.Config.Storage</code><br><code>Host.Config.SystemManagement</code><br><code>Host.Config.Resources</code><br><code>Host.Config.AutoStart</code></td>
        <td>Administration of ESXi host infrastructure for initial configuration, network topology changes, storage additions, and hypervisor fine-tuning.</td>
    </tr>
    <tr>
        <td><code>Infraprofile.Read</code></td>
        <td>Viewing vCenter server configurations.</td>
    </tr>
    <tr>
        <td><code>Profile.Clear</code><br><code>Profile.Create</code><br><code>Profile.Delete</code><br><code>Profile.Edit</code><br><code>Profile.Export</code><br><code>Profile.View</code></td>
        <td>Managing operations related to the creation, modification, and application of host configuration templates (Host Profiles).</td>
    </tr>
    <tr>
        <td><code>Network.Assign</code><br><code>Resource.ApplyRecommendation</code><br><code>Resource.AssignVAppToPool</code><br><code>Resource.AssignVMToPool</code><br><code>Resource.ColdMigrate</code><br><code>Resource.CreatePool</code><br><code>Resource.DeletePool</code><br><code>Resource.EditPool</code><br><code>Resource.HotMigrate</code><br><code>Resource.MovePool</code><br><code>Resource.QueryVMotion</code><br><code>Resource.RenamePool</code><br><code>VirtualMachine.Config.AddExistingDisk</code><br><code>VirtualMachine.Config.AddNewDisk</code><br><code>VirtualMachine.Config.AddRemoveDevice</code><br><code>VirtualMachine.Config.AdvancedConfig</code><br><code>VirtualMachine.Config.Annotation</code><br><code>VirtualMachine.Config.ChangeTracking</code><br><code>VirtualMachine.Config.CPUCount</code><br><code>VirtualMachine.Config.DiskExtend</code><br><code>VirtualMachine.Config.DiskLease</code><br><code>VirtualMachine.Config.EditDevice</code><br><code>VirtualMachine.Config.HostUSBDevice</code><br><code>VirtualMachine.Config.ManagedBy</code><br><code>VirtualMachine.Config.Memory</code><br><code>VirtualMachine.Config.MksControl</code><br><code>VirtualMachine.Config.QueryFTCompatibility</code><br><code>VirtualMachine.Config.QueryUnownedFiles</code><br><code>VirtualMachine.Config.RawDevice</code><br><code>VirtualMachine.Config.ReloadFromPath</code><br><code>VirtualMachine.Config.RemoveDisk</code><br><code>VirtualMachine.Config.Rename</code><br><code>VirtualMachine.Config.ResetGuestInfo</code><br><code>VirtualMachine.Config.Resource</code><br><code>VirtualMachine.Config.Settings</code><br><code>VirtualMachine.Config.SwapPlacement</code><br><code>VirtualMachine.Config.ToggleForkParent</code><br><code>VirtualMachine.Config.UpgradeVirtualHardware</code><br><code>VirtualMachine.GuestOperations.Execute</code><br><code>VirtualMachine.GuestOperations.Modify</code><br><code>VirtualMachine.GuestOperations.ModifyAliases</code><br><code>VirtualMachine.GuestOperations.Query</code><br><code>VirtualMachine.GuestOperations.QueryAliases</code><br><code>VirtualMachine.Hbr.ConfigureReplication</code><br><code>VirtualMachine.Hbr.MonitorReplication</code><br><code>VirtualMachine.Hbr.ReplicaManagement</code><br><code>VirtualMachine.Interact.AnswerQuestion</code><br><code>VirtualMachine.Interact.Backup</code><br><code>VirtualMachine.Interact.ConsoleInteract</code><br><code>VirtualMachine.Interact.CreateScreenshot</code><br><code>VirtualMachine.Interact.CreateSecondary</code><br><code>VirtualMachine.Interact.DefragmentAllDisks</code><br><code>VirtualMachine.Interact.DeviceConnection</code><br><code>VirtualMachine.Interact.DisableSecondary</code><br><code>VirtualMachine.Interact.DnD</code><br><code>VirtualMachine.Interact.EnableSecondary</code><br><code>VirtualMachine.Interact.GuestControl</code><br><code>VirtualMachine.Interact.MakePrimary</code><br><code>VirtualMachine.Interact.Pause</code><br><code>VirtualMachine.Interact.PowerOff</code><br><code>VirtualMachine.Interact.PowerOn</code><br><code>VirtualMachine.Interact.PutUsbScanCodes</code><br><code>VirtualMachine.Interact.Record</code><br><code>VirtualMachine.Interact.Replay</code><br><code>VirtualMachine.Interact.Reset</code><br><code>VirtualMachine.Interact.SESparseMaintenance</code><br><code>VirtualMachine.Interact.SetCDMedia</code><br><code>VirtualMachine.Interact.SetFloppyMedia</code><br><code>VirtualMachine.Interact.Suspend</code><br><code>VirtualMachine.Interact.SuspendToMemory</code><br><code>VirtualMachine.Interact.TerminateFaultTolerantVM</code><br><code>VirtualMachine.Interact.ToolsInstall</code><br><code>VirtualMachine.Interact.TurnOffFaultTolerance</code><br><code>VirtualMachine.Inventory.Create</code><br><code>VirtualMachine.Inventory.CreateFromExisting</code><br><code>VirtualMachine.Inventory.Delete</code><br><code>VirtualMachine.Inventory.Move</code><br><code>VirtualMachine.Inventory.Register</code><br><code>VirtualMachine.Inventory.Unregister</code><br><code>VirtualMachine.Namespace.Event</code><br><code>VirtualMachine.Namespace.EventNotify</code><br><code>VirtualMachine.Namespace.Management</code><br><code>VirtualMachine.Namespace.ModifyContent</code><br><code>VirtualMachine.Namespace.Query</code><br><code>VirtualMachine.Namespace.ReadContent</code><br><code>VirtualMachine.Provisioning.Clone</code><br><code>VirtualMachine.Provisioning.CloneTemplate</code><br><code>VirtualMachine.Provisioning.CreateTemplateFromVM</code><br><code>VirtualMachine.Provisioning.Customize</code><br><code>VirtualMachine.Provisioning.DeployTemplate</code><br><code>VirtualMachine.Provisioning.DiskRandomAccess</code><br><code>VirtualMachine.Provisioning.DiskRandomRead</code><br><code>VirtualMachine.Provisioning.FileRandomAccess</code><br><code>VirtualMachine.Provisioning.GetVmFiles</code><br><code>VirtualMachine.Provisioning.MarkAsTemplate</code><br><code>VirtualMachine.Provisioning.MarkAsVM</code><br><code>VirtualMachine.Provisioning.ModifyCustSpecs</code><br><code>VirtualMachine.Provisioning.PromoteDisks</code><br><code>VirtualMachine.Provisioning.PutVmFiles</code><br><code>VirtualMachine.Provisioning.ReadCustSpecs</code><br><code>VirtualMachine.State.CreateSnapshot</code><br><code>VirtualMachine.State.RemoveSnapshot</code><br><code>VirtualMachine.State.RenameSnapshot</code><br><code>VirtualMachine.State.RevertToSnapshot</code></td>
        <td>To manage the virtual machines lifecycle in a Deckhouse Kubernetes Platform cluster.</td>
    </tr>
    <tr>
        <td><code>VApp.AssignVM</code><br><code>VApp.AssignResourcePool</code><br><code>VApp.AssignVApp</code><br><code>VApp.Clone</code><br><code>VApp.Create</code><br><code>VApp.Delete</code><br><code>VApp.Export</code><br><code>VApp.Import</code><br><code>VApp.Move</code><br><code>VApp.PowerOff</code><br><code>VApp.PowerOn</code><br><code>VApp.PullFromURL</code><br><code>VApp.Rename</code><br><code>VApp.Suspend</code><br><code>VApp.Unregister</code><br><code>VApp.ExtractOvfEnvironment</code><br><code>VApp.ApplicationConfig</code><br><code>VApp.InstanceConfig</code><br><code>VApp.ManagedByConfig</code><br><code>VApp.ResourceConfig</code></td>
        <td>Manage operations related to vApp deployment and configuration.</td>
    </tr>
    <tr>
        <td><code>VcIdentityProviders.Read</code></td>
        <td>Read access to the VcIdentityProviders API (vCenter Server identity providers).</td>
    </tr>
  </tbody>
</table>

## vSphere configuration

### Installing govc

You'll need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) — to proceed with the rest of the guide.

After the installation is complete, set the environment variables required to work with vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

### Creating tags and tag categories

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

#### Datastore configuration

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

### Creating and assigning a role

{% alert %}
We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

This all-encompassing Role should be enough for all Deckhouse components. A detailed list of privileges is described in the section ["List of required privileges"](#list-of-required-privileges). If you need a more granular Role, please contact your Deckhouse support.
{% endalert %}

Create a role with the corresponding permissions:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   VcIdentityProviders.Read \
   Infraprofile.Read\
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.' -e 'Host.Cim.' -e 'Host.Config.' -e 'Profile.' -e 'VApp.')
```

Assign the role to a user on the `vCenter` object:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

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

1. Before creating the VM template, reset identifiers and the `cloud-init` state:

   ```shell
   truncate -s 0 /etc/machine-id rm /var/lib/dbus/machine-id ln -s /etc/machine-id /var/lib/dbus/machine-id
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

### Using the data store

Various types of storage can be used in the cluster; for the minimum configuration, you will need:
* Datastore for provisioning PersistentVolumes to the Kubernetes cluster.
* Datastore for provisioning root disks for the VMs (it can be the same Datastore as for PersistentVolume).
