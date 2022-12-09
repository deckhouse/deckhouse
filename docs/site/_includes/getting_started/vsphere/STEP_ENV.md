{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

## List of required vSphere resources

* **User** with required set of [permissions](#permissions).
* **Network** with DHCP server and access to the Internet
* **Datacenter** with a tag in [`k8s-region`](#creating-tags-and-tag-categories) category.
* **ComputeCluster** with a tag in [`k8s-zone`](#creating-tags-and-tag-categories).
* **Datastore** with required [tags](#datastore-tags).
* **Template** — [prepared](#building-a-vm-image) VM image.

## vSphere configuration

You'll need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) to proceed with the rest of the guide.

### govc configuration

{% snippetcut %}
```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<USER_NAME>
export GOVC_PASSWORD=<USER_PASSWORD>
export GOVC_INSECURE=1
```
{% endsnippetcut %}

### Creating tags and tag categories

VMware vSphere doesn't have "regions" and "zones". It has Datacenters and ComputeClusters.

To establish relation between these and "regions"/"zones" we'll use tags that fall into two tag categories. One for "region" tags and another for "zones tags".

For example, if you've got two Datacenters with a similarly named zone in each one:

{% snippetcut %}
```shell
govc tags.category.create -d "Kubernetes region" k8s-region
govc tags.category.create -d "Kubernetes zone" k8s-zone
govc tags.create -d "Kubernetes Region #1" -c k8s-region test_region_1
govc tags.create -d "Kubernetes Region #2" -c k8s-region test_region_2
govc tags.create -d "Kubernetes Zone Test" -c k8s-zone test_zone
```
{% endsnippetcut %}

"Region" tags are attached to Datacenters:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test_region_1 /DC1
govc tags.attach -c k8s-region test_region_2 /DC2
```
{% endsnippetcut %}

"Zone" tags are attached to ComputeClusters and Datastores:

{% snippetcut %}
```shell
govc tags.attach -c k8s-zone test_zone /DC/host/test_cluster
govc tags.attach -c k8s-zone test_zone /DC/datastore/test_lun
```
{% endsnippetcut %}

#### Datastore tags

You can dynamically provision PVs (via PVCs) if all Datastores are present on **all** ESXis in a selected `zone` (ComputeCluster).
StorageClasses will be created automatically for each Datastore that is tagged with `region` and `zone` tags.

### Permissions

> We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

You have to create a role with a following list of permissions and attach
it to **vCenter**.

{% snippetcut %}
```shell
govc role.create kubernetes \
  Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement Folder.Create Global.GlobalTag Global.SystemTag \
  InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateTag \
  InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory \
  InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag \
  InventoryService.Tagging.ObjectAttachable \
  Network.Assign Resource.AssignVMToPool Resource.ColdMigrate Resource.HotMigrate Resource.CreatePool \
  Resource.DeletePool Resource.RenamePool Resource.EditPool Resource.MovePool StorageProfile.View System.Anonymous System.Read System.View \
  VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice \
  VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount \
  VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease \
  VirtualMachine.Config.EditDevice VirtualMachine.Config.HostUSBDevice VirtualMachine.Config.ManagedBy \
  VirtualMachine.Config.Memory VirtualMachine.Config.MksControl VirtualMachine.Config.QueryFTCompatibility \
  VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath \
  VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo \
  VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement \
  VirtualMachine.Config.ToggleForkParent VirtualMachine.Config.UpgradeVirtualHardware VirtualMachine.GuestOperations.Execute \
  VirtualMachine.GuestOperations.Modify VirtualMachine.GuestOperations.ModifyAliases VirtualMachine.GuestOperations.Query \
  VirtualMachine.GuestOperations.QueryAliases VirtualMachine.Hbr.ConfigureReplication VirtualMachine.Hbr.MonitorReplication \
  VirtualMachine.Hbr.ReplicaManagement VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.Backup \
  VirtualMachine.Interact.ConsoleInteract VirtualMachine.Interact.CreateScreenshot VirtualMachine.Interact.CreateSecondary \
  VirtualMachine.Interact.DefragmentAllDisks VirtualMachine.Interact.DeviceConnection VirtualMachine.Interact.DisableSecondary \
  VirtualMachine.Interact.DnD VirtualMachine.Interact.EnableSecondary VirtualMachine.Interact.GuestControl \
  VirtualMachine.Interact.MakePrimary VirtualMachine.Interact.Pause VirtualMachine.Interact.PowerOff \
  VirtualMachine.Interact.PowerOn VirtualMachine.Interact.PutUsbScanCodes VirtualMachine.Interact.Record \
  VirtualMachine.Interact.Replay VirtualMachine.Interact.Reset VirtualMachine.Interact.SESparseMaintenance \
  VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.SetFloppyMedia VirtualMachine.Interact.Suspend \
  VirtualMachine.Interact.TerminateFaultTolerantVM VirtualMachine.Interact.ToolsInstall \
  VirtualMachine.Interact.TurnOffFaultTolerance VirtualMachine.Inventory.Create \
  VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete \
  VirtualMachine.Inventory.Move VirtualMachine.Inventory.Register VirtualMachine.Inventory.Unregister \
  VirtualMachine.Namespace.Event VirtualMachine.Namespace.EventNotify VirtualMachine.Namespace.Management \
  VirtualMachine.Namespace.ModifyContent VirtualMachine.Namespace.Query VirtualMachine.Namespace.ReadContent \
  VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.CloneTemplate VirtualMachine.Provisioning.CreateTemplateFromVM \
  VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate VirtualMachine.Provisioning.DiskRandomAccess \
  VirtualMachine.Provisioning.DiskRandomRead VirtualMachine.Provisioning.FileRandomAccess VirtualMachine.Provisioning.GetVmFiles \
  VirtualMachine.Provisioning.MarkAsTemplate VirtualMachine.Provisioning.MarkAsVM VirtualMachine.Provisioning.ModifyCustSpecs \
  VirtualMachine.Provisioning.PromoteDisks VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs \
  VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot VirtualMachine.State.RevertToSnapshot \
  Cns.Searchable StorageProfile.View

govc permissions.set  -principal username -role kubernetes /DC
```
{% endsnippetcut %}

### Building a VM image

1. [Install Packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).
1. Clone the Deckhouse repository:
   {% snippetcut %}
```bash
git clone https://github.com/deckhouse/deckhouse/
```
   {% endsnippetcut %}

1. `cd` into the `ee/modules/030-cloud-provider-vsphere/packer/` folder of the repository:
   {% snippetcut %}
```bash
cd deckhouse/ee/modules/030-cloud-provider-vsphere/packer/
```
   {% endsnippetcut %}

1. Create a file name `vsphere.auto.pkrvars.hcl` with the following contents:
   {% snippetcut %}
```hcl
vcenter_server = "<hostname or IP of a vCenter>"
vcenter_username = "<username>"
vcenter_password = "<password>"
vcenter_cluster = "<ComputeCluster name, in which template will be created>"
vcenter_datacenter = "<Datacenter name>"
vcenter_resource_pool = <"ResourcePool name">
vcenter_datastore = "<Datastore name>"
vcenter_folder = "<Folder name>"
vm_network = "<VM network in which you will build an image>"
```
   {% endsnippetcut %}
{% raw %}
1. If your PC (the one you are running Packer from) is not located in the same network as `vm_network` (if you are connected through a tunnel), change `{{ .HTTPIP }}` in the `<UbuntuVersion>.pkrvars.hcl` to your PCs VPN IP:

    ```hcl
    " url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg",
    ```
{% endraw %}

1. Build a version of Ubuntu:

   {% snippetcut %}
```shell
# Ubuntu 20.04
packer build --var-file=20.04.pkrvars.hcl .
# Ubuntu 18.04
packer build --var-file=18.04.pkrvars.hcl .
```
   {% endsnippetcut %}
