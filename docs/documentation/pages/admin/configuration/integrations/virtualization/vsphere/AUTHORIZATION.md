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
     - You do not need to create the folder in advance. The installer creates it at the path from the [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) parameter.
     - If the folder already exists, set [`vmFolderExists: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists), otherwise the installation fails with the `The name '<FOLDER>' already exists` error.
     - More than one cluster cannot be placed in the same folder.
  1. Role:
     - The role must include the required [set of privileges](layout.html#list-of-required-privileges).
  1. User:
     - The user must be assigned the role specified in the previous item.
- A tag from the category specified in the [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) parameter must be assigned to the created Datacenter (default: `k8s-region`). This tag defines the region.

### VM image requirements

To create a VM template (`Template`), it is recommended to use a ready-made cloud image/OVA file provided by the OS vendor:

- [**Ubuntu**](https://cloud-images.ubuntu.com/)
- [**Debian**](https://cloud.debian.org/images/cloud/)
- [**CentOS**](https://cloud.centos.org/)
- [**Rocky Linux**](https://rockylinux.org/alternative-images/) (section *Generic Cloud / OpenStack*)

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

### Preparing the template in vSphere Client

Once the image is ready, do the following in vSphere Client:

1. Import the operating system image. Select a Datacenter or a Cluster, open "ACTIONS" → "Deploy OVF Template..." and specify the OVA file provided by the OS vendor.

   ![Preparing the template, step 1](../../../../images/cloud-provider-vsphere/vm-template-setup/deploy-ovf-template.png)

1. Check the parameters of the resulting virtual machine. On the "Summary" tab, the "Compatibility" row of the "VM Hardware" panel shows the hardware version, and the device list shows the attached disks. Hardware version 15 or later and a single disk are required.

   ![Preparing the template, step 2](../../../../images/cloud-provider-vsphere/vm-template-setup/vm-hardware-compatibility.png)

1. If the hardware version is lower than 15, upgrade it. Power off the virtual machine and open "ACTIONS" → "Compatibility" → "Upgrade VM Compatibility...". The item is unavailable for a running machine. To schedule the upgrade for the next power off, use "Schedule VM Compatibility Upgrade...".

   ![Preparing the template, step 3](../../../../images/cloud-provider-vsphere/vm-template-setup/upgrade-vm-compatibility.png)

1. Convert the virtual machine to a template. Open "ACTIONS" → "Template" → "Convert to Template". The `template` parameter accepts both a template and a powered off virtual machine.

   ![Preparing the template, step 4](../../../../images/cloud-provider-vsphere/vm-template-setup/convert-to-template.png)

## vCenter TLS certificate verification

DKP connects to vCenter over TLS and verifies its certificate. If the vCenter certificate is issued by a custom or enterprise certificate authority, pass the certificate chain of that authority in the [`caBundle`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle) parameter. Certificate verification stays enabled in this case.

Specify the chain in PEM format. It is the same setting, but its path depends on where the vCenter connection is described:

- When installing a cluster, the connection is described in the [`provider`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider) section of the [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) resource next to the [`provider.server`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-server) parameter, so the chain is set in [`provider.caBundle`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle).
- In a running cluster, the connection is described at the top level of the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module settings next to the [`host`](/modules/cloud-provider-vsphere/configuration.html#parameters-host) parameter, so the chain is set in [`caBundle`](/modules/cloud-provider-vsphere/configuration.html#parameters-cabundle).

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

The [`insecure: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-insecure) parameter disables vCenter certificate verification completely. In the settings of a running cluster, the same parameter is named [`insecure`](/modules/cloud-provider-vsphere/configuration.html#parameters-insecure). Set either `caBundle` or `insecure: true`. If both parameters are set, DKP rejects the configuration.

For the NSX-T connection, the certificate chain is set by the separate [`nsxt.caBundle`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nsxt-cabundle) parameter. In the settings of a running cluster, these are the [`nsxt.caBundle`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt-cabundle) and [`nsxt.insecureFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt-insecureflag) parameters. Set either `nsxt.caBundle` or [`nsxt.insecureFlag: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nsxt-insecureflag), otherwise DKP rejects the configuration.

{% alert level="warning" %}
The [`csi-vsphere`](/modules/csi-vsphere/) module does not support the `caBundle` parameter and connects to vCenter either with certificate verification against the system certificate authorities or with the [`insecure`](/modules/csi-vsphere/configuration.html#parameters-insecure) parameter.
{% endalert %}

## vSphere configuration

### Configuration in vSphere Client

#### Creating tags and tag categories in vSphere Client

VMware vSphere does not have built-in concepts of a "region" or a "zone". In vSphere, a Datacenter acts as a region, and a Cluster acts as a zone. Tags are used to establish this mapping.

1. Open vSphere Client and go to "Menu" → "Tags & Custom Attributes" → "Tags".

   ![Creating tags and tag categories, step 1](../../../../images/cloud-provider-vsphere/tags-categories-setup/menu-tags-and-custom-attributes.png)

1. Open the "Categories" tab and click "NEW". Create a category for regions (for example, `k8s-region`): set "Tags Per Object" to "One tag" and specify the applicable object types, including Datacenter.

   ![Creating tags and tag categories, step 2](../../../../images/cloud-provider-vsphere/tags-categories-setup/create-category-region.png)

1. Create a second category for zones (for example, `k8s-zone`) with the object types Host, Cluster, and Datastore.

   ![Creating tags and tag categories, step 3](../../../../images/cloud-provider-vsphere/tags-categories-setup/create-category-zone.png)

1. Go to the "Tags" tab and create at least one tag in the region category and one tag in the zone category (for example, `test-region`, `test-zone-1`).

   ![Creating tags and tag categories, step 4](../../../../images/cloud-provider-vsphere/tags-categories-setup/tags-list.png)

1. In the "Inventory" tab, select the target Datacenter, open the "Summary" panel, then choose "Actions" → "Tags & Custom Attributes" → "Assign Tag" and assign the region tag.
   Repeat this step for each Cluster that will host nodes, assigning the appropriate zone tags.

   ![Creating tags and tag categories, step 5.1](../../../../images/cloud-provider-vsphere/tags-categories-setup/datacenter-actions-assign-tag.png)
   ![Creating tags and tag categories, step 5.2](../../../../images/cloud-provider-vsphere/tags-categories-setup/assign-tag-to-datacenter.png)

#### Configuring Datastore in vSphere Client

{% alert level="warning" %}
For dynamic provisioning of PersistentVolume, the Datastore must be available on **every** ESXi host in the zone (shared datastore).
{% endalert %}

In the "Inventory" tab, select the Datastore, open the "Summary" panel, then choose "Actions" → "Tags & Custom Attributes" → "Assign Tag". Assign the Datastore the same region tag as the corresponding Datacenter, and the same zone tag as the corresponding Cluster.

![Creating tags and tag categories, step 6](../../../../images/cloud-provider-vsphere/tags-categories-setup/assign-tags-to-datastore.png)

To make sure the Datastore is connected to every ESXi host in the zone, open "Menu" → "Inventory" → "Storage", select the Datastore, and go to the "Hosts" tab. The list shows the hosts that have access to the Datastore.

![Checking Datastore availability](../../../../images/cloud-provider-vsphere/datastore-setup/datastore-hosts.png)

#### Creating a virtual machine folder in vSphere Client

The installer creates the virtual machine folder at the path from the [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath) parameter. If the folder already exists, set [`vmFolderExists: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists).

To create the folder in advance, open "Menu" → "Inventory" → "Hosts and Clusters", select a Datacenter object in the list, then open "ACTIONS" → "New Folder" → "New VM and Template Folder..." and enter the name.

![Creating a virtual machine folder](../../../../images/cloud-provider-vsphere/vm-folder-setup/new-vm-and-template-folder.png)

#### Creating a resource pool in vSphere Client

In every zone, the installer creates a resource pool named after the [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of the ClusterConfiguration resource. If the [`baseResourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-baseresourcepool) parameter is set, the pool is created inside the pool it points to. The parent pool must exist, the installer does not create it. The same applies to the pool from the [`resourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-resourcepool) parameter of a node group.

To create a pool, open "Menu" → "Inventory" → "Hosts and Clusters", select a Cluster object in the list, then open "ACTIONS" → "New Resource Pool..." and enter the name.

![Creating a resource pool](../../../../images/cloud-provider-vsphere/resource-pool-setup/new-resource-pool.png)

With [`useNestedResourcePool: false`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool), the installer does not create a nested pool. Virtual machines are placed in the pool from the `resourcePool` parameter. If that parameter is not set, the machines are placed in the root pool of the Cluster.

#### Creating and assigning a role in vSphere Client

1. Open "Menu" → "Administration". A separate vCenter management page opens, where the "Roles" item is in the left pane under "Access Control".

   ![Creating and assigning a role, step 1](../../../../images/cloud-provider-vsphere/role-setup/access-control-roles.png)

1. Click "NEW", enter a role name (for example, `deckhouse`), and add the privileges from the [list](layout.html#list-of-required-privileges).

   ![Creating and assigning a role, step 2](../../../../images/cloud-provider-vsphere/role-setup/new-role-privileges.png)

1. Assign the role to the Deckhouse service account: go to "Menu" → "Administration" → "Access Control" → "Global Permissions", click "ADD", and select the user and the `deckhouse` role.

   ![Creating and assigning a role, step 3](../../../../images/cloud-provider-vsphere/role-setup/add-global-permission.png)

### Configuration with govc

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

The role described below includes the privileges from [the list of required privileges](layout.html#list-of-required-privileges) section. If you need a more granular Role, please contact your Deckhouse support.
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

### Role assignment scope

The role is assigned on the vCenter root object rather than on the folder with the cluster virtual machines. The platform components access objects outside that folder:

- The CSI driver determines volume topology by the ESXi hosts attached to a Datastore, so it accesses Cluster and Host objects.
- The resource discovery component searches for CNS disks across the whole vCenter, and it needs the `Cns.Searchable` privilege from the [Storage](layout.html#storage) group for that.
- The installer creates a resource pool in the Cluster object and a folder in the Datacenter, which needs the privileges from the [Virtual machine placement](layout.html#virtual-machine-placement) group.

If you limit the role to the virtual machine folder, these operations fail.

### Checking privileges

The CSI driver checks the account privileges on each Datastore and excludes those where the privileges are insufficient. Such a Datastore does not appear in the list of available ones, and ordering a PersistentVolume through the corresponding StorageClass fails.

If a tagged Datastore does not produce a working StorageClass, check the account privileges on that object:

```shell
govc permissions.ls /<DATACENTER_NAME>/datastore/<DATASTORE_NAME>
```

Account privileges on an object can also be viewed in vSphere Client. Select the object, open the "Permissions" tab, and find the account in the list. The "Defined In" column shows the object the permission is defined on. The "Global Permission" value means the permission is assigned globally.

![Viewing permissions on an object](../../../../images/cloud-provider-vsphere/permissions-check/datastore-permissions.png)
