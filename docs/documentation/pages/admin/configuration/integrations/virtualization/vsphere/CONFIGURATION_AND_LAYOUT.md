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
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
internalNetworkCIDR: 192.168.199.0/24
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

- `region`: Tag assigned to the Datacenter object.
- `zoneTagCategory` and `regionTagCategory`: Tag categories used to identify regions and zones.
- `internalNetworkCIDR`: Subnet for assigning internal IP addresses.
- `vmFolderPath`: Path to the folder where cluster virtual machines will be placed.
- `sshPublicKey`: Public SSH key used to access the nodes.
- `zones`: List of zones available for node placement.

{% alert level="info" %}
All nodes placed in different zones must have access to shared datastores with matching zone tags.
{% endalert %}

## List of required privileges

{% alert level="info" %}
To create a role and assign it to a user, refer to [Configuration in vSphere Client](authorization.html#configuration-in-vsphere-client) and [Configuration with govc](authorization.html#configuration-with-govc) sections.
{% endalert %}

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
