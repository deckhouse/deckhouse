{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

To install Deckhouse Kubernetes Platform on VMware vSphere, you need vSphere version `7.x` or `8.x` with support for the [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) mechanism.

## List of required vSphere resources

{% alert %}
Deckhouse uses the `ens192` interface as the default interface for virtual machines in vSphere. Therefore, when using static IP addresses in `mainNetwork`, you must create an interface named `ens192` in the OS image as the default interface.
{% endalert %}

* **User** with required [set of privileges](#creating-and-assigning-a-role).
* **Network** with DHCP server and access to the Internet
* **Datacenter** with a tag in [`k8s-region`](#creating-tags-and-tag-categories) category.
* **Cluster** with a tag in [`k8s-zone`](#creating-tags-and-tag-categories) category.
* **Datastore** with required [tags](#datastore-configuration).
* **Template** — the [prepared](#preparing-a-virtual-machine-image) VM image.

## vSphere configuration

{% alert level="info" %}
To configure tags, datastore tagging, and the Deckhouse service role through the **VMware vSphere Client** UI, follow [Configuration via vSphere Client](/modules/cloud-provider-vsphere/environment.html#configuration-in-vsphere-client) in the module documentation. The steps below use **`govc` only**.
{% endalert %}

### Installing govc

You'll need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) — to proceed with the rest of the guide.

After the installation is complete, set the environment variables required to work with vCenter.

{% alert level="warning" %}
Make sure to specify the username together with the domain, for example: `username@domain.local`.
{% endalert %}

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

Attach "zone" tags to `Cluster` objects:

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

The role that you are asked to create next includes the privileges from the section [List of required privileges](/modules/cloud-provider-vsphere/environment.html#list-of-required-privileges). If you need a more granular Role, please contact your Deckhouse support.
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

Assign the role to a user on the `vCenter` object.

{% alert level="warning" %}
Make sure to specify the username together with the domain, for example: `username@domain.local`.
{% endalert %}

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

### Preparing a virtual machine image

It is recommended to use a pre-built cloud image/OVA file provided by the OS vendor to create a `Template`:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (*Generic Cloud / OpenStack* section)

If you need to use your own image, please refer to the [documentation](/modules/cloud-provider-vsphere/environment.html#virtual-machine-image-requirements).
