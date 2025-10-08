---
title: Layouts and configuration
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
Read [the documentation](authorization.html#creating-and-assigning-a-role) on how to create and assign a role to a user.
{% endalert %}

**A detailed list of privileges required for Deckhouse Kubernetes Platform to work in vSphere:**

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
        <td><code>Network.Assign</code><br><code>Resource.ApplyRecommendation</code><br><code>Resource.AssignVAppToPool</code><br><code>Resource.AssignVMToPool</code><br><code>Resource.ColdMigrate</code><br><code>Resource.CreatePool</code><br><code>Resource.DeletePool</code><br><code>Resource.EditPool</code><br><code>Resource.HotMigrate</code><br><code>Resource.MovePool</code><br><code>Resource.QueryVMotion</code><br><code>Resource.RenamePool</code><br><code>VirtualMachine.Config.AddExistingDisk</code><br><code>VirtualMachine.Config.AddNewDisk</code><br><code>VirtualMachine.Config.AddRemoveDevice</code><br><code>VirtualMachine.Config.AdvancedConfig</code><br><code>VirtualMachine.Config.Annotation</code><br><code>VirtualMachine.Config.ChangeTracking</code><br><code>VirtualMachine.Config.CPUCount</code><br><code>VirtualMachine.Config.DiskExtend</code><br><code>VirtualMachine.Config.DiskLease</code><br><code>VirtualMachine.Config.EditDevice</code><br><code>VirtualMachine.Config.HostUSBDevice</code><br><code>VirtualMachine.Config.ManagedBy</code><br><code>VirtualMachine.Config.Memory</code><br><code>VirtualMachine.Config.MksControl</code><br><code>VirtualMachine.Config.QueryFTCompatibility</code><br><code>VirtualMachine.Config.QueryUnownedFiles</code><br><code>VirtualMachine.Config.RawDevice</code><br><code>VirtualMachine.Config.ReloadFromPath</code><br><code>VirtualMachine.Config.RemoveDisk</code><br><code>VirtualMachine.Config.Rename</code><br><code>VirtualMachine.Config.ResetGuestInfo</code><br><code>VirtualMachine.Config.Resource</code><br><code>VirtualMachine.Config.Settings</code><br><code>VirtualMachine.Config.SwapPlacement</code><br><code>VirtualMachine.Config.ToggleForkParent</code><br><code>VirtualMachine.Config.UpgradeVirtualHardware</code><br><code>VirtualMachine.GuestOperations.Execute</code><br><code>VirtualMachine.GuestOperations.Modify</code><br><code>VirtualMachine.GuestOperations.ModifyAliases</code><br><code>VirtualMachine.GuestOperations.Query</code><br><code>VirtualMachine.GuestOperations.QueryAliases</code><br><code>VirtualMachine.Hbr.ConfigureReplication</code><br><code>VirtualMachine.Hbr.MonitorReplication</code><br><code>VirtualMachine.Hbr.ReplicaManagement</code><br><code>VirtualMachine.Interact.AnswerQuestion</code><br><code>VirtualMachine.Interact.Backup</code><br><code>VirtualMachine.Interact.ConsoleInteract</code><br><code>VirtualMachine.Interact.CreateScreenshot</code><br><code>VirtualMachine.Interact.CreateSecondary</code><br><code>VirtualMachine.Interact.DefragmentAllDisks</code><br><code>VirtualMachine.Interact.DeviceConnection</code><br><code>VirtualMachine.Interact.DisableSecondary</code><br><code>VirtualMachine.Interact.DnD</code><br><code>VirtualMachine.Interact.EnableSecondary</code><br><code>VirtualMachine.Interact.GuestControl</code><br><code>VirtualMachine.Interact.MakePrimary</code><br><code>VirtualMachine.Interact.Pause</code><br><code>VirtualMachine.Interact.PowerOff</code><br><code>VirtualMachine.Interact.PowerOn</code><br><code>VirtualMachine.Interact.PutUsbScanCodes</code><br><code>VirtualMachine.Interact.Record</code><br><code>VirtualMachine.Interact.Replay</code><br><code>VirtualMachine.Interact.Reset</code><br><code>VirtualMachine.Interact.SESparseMaintenance</code><br><code>VirtualMachine.Interact.SetCDMedia</code><br><code>VirtualMachine.Interact.SetFloppyMedia</code><br><code>VirtualMachine.Interact.Suspend</code><br><code>VirtualMachine.Interact.SuspendToMemory</code><br><code>VirtualMachine.Interact.TerminateFaultTolerantVM</code><br><code>VirtualMachine.Interact.ToolsInstall</code><br><code>VirtualMachine.Interact.TurnOffFaultTolerance</code><br><code>VirtualMachine.Inventory.Create</code><br><code>VirtualMachine.Inventory.CreateFromExisting</code><br><code>VirtualMachine.Inventory.Delete</code><br><code>VirtualMachine.Inventory.Move</code><br><code>VirtualMachine.Inventory.Register</code><br><code>VirtualMachine.Inventory.Unregister</code><br><code>VirtualMachine.Namespace.Event</code><br><code>VirtualMachine.Namespace.EventNotify</code><br><code>VirtualMachine.Namespace.Management</code><br><code>VirtualMachine.Namespace.ModifyContent</code><br><code>VirtualMachine.Namespace.Query</code><br><code>VirtualMachine.Namespace.ReadContent</code><br><code>VirtualMachine.Provisioning.Clone</code><br><code>VirtualMachine.Provisioning.CloneTemplate</code><br><code>VirtualMachine.Provisioning.CreateTemplateFromVM</code><br><code>VirtualMachine.Provisioning.Customize</code><br><code>VirtualMachine.Provisioning.DeployTemplate</code><br><code>VirtualMachine.Provisioning.DiskRandomAccess</code><br><code>VirtualMachine.Provisioning.DiskRandomRead</code><br><code>VirtualMachine.Provisioning.FileRandomAccess</code><br><code>VirtualMachine.Provisioning.GetVmFiles</code><br><code>VirtualMachine.Provisioning.MarkAsTemplate</code><br><code>VirtualMachine.Provisioning.MarkAsVM</code><br><code>VirtualMachine.Provisioning.ModifyCustSpecs</code><br><code>VirtualMachine.Provisioning.PromoteDisks</code><br><code>VirtualMachine.Provisioning.PutVmFiles</code><br><code>VirtualMachine.Provisioning.ReadCustSpecs</code><br><code>VirtualMachine.State.CreateSnapshot</code><br><code>VirtualMachine.State.RemoveSnapshot</code><br><code>VirtualMachine.State.RenameSnapshot</code><br><code>VirtualMachine.State.RevertToSnapshot</code></td>
        <td>To manage the virtual machines lifecycle in a Deckhouse Kubernetes Platform cluster.</td>
    </tr>
  </tbody>
</table>
