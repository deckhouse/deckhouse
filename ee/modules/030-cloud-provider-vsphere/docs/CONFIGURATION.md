---
title: "Cloud provider — VMware vSphere: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in vSphere.

If the cluster control plane is hosted on a virtual machines or bare-metal servers, the cloud provider uses the settings from the `cloud-provider-vsphere` module in the Deckhouse configuration (see below). Otherwise, if the cluster control plane is hosted in a cloud, the cloud provider uses the [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) structure for configuration.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the `node-manager` module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` parameter of NodeGroup). In the case of the vSphere cloud provider, the instance class is the [`VsphereInstanceClass`](cr.html#vsphereinstanceclass) custom resource that stores specific parameters of the machines.

{% include module-settings.liquid %}

## Storage

The module automatically creates a StorageClass for each Datastore and DatastoreCluster in the zone (or zones).

Also, it can set the name of StorageClass that will be used in the cluster by default (the [default](#parameters-storageclass-default) parameter), and
filter out the unnecessary StorageClasses (the [exclude](#parameters-storageclass-exclude) parameter).

### CSI

By default, the storage subsystem uses CNS volumes with the ability of online-resize. FCD volumes are also supported, but only in the legacy or migration modes. You can set this via the [compatibilityFlag](#parameters-storageclass-compatibilityflag) parameter.

### Important information concerning the increase of the PVC size

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) f volume-resizer, CSI, and vSphere API, you have to do the following after increasing the PVC size:

1. On the node where the Pod is located, run the `kubectl cordon <node_name>` command.
2. Delete the Pod.
3. Make sure that the resize was successful. The PVC object must *not have* the `Resizing` condition.
   > The `FileSystemResizePending` state is OK.
4. On the node where the Pod is located, run the `kubectl uncordon <node_name>` command.

## Environment requirements

* vSphere version required: `v7.0U2` ([required](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) for the `Online volume expansion` work).
* vCenter to which master nodes can connect to from within the cluster.
* Datacenter with the following components:
  1. VirtualMachine template.
     * VM image should use `Virtual machines with hardware version 15 or later` (required for online resize to work).
     * The following packages must be installed in the VM image: `open-vm-tools`, `cloud-init` and [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if the `cloud-init` version lower than 21.3 is used).
  2. The network must be available on all ESXi where VirtualMachines will be created.
  3. One or more Datastores connected to all ESXi where VirtualMachines will be created.
     * A tag from the tag category in `zoneTagCategory` (`k8s-zone` by default) **must be added** to Datastores. This tag will indicate the **zone**.  All Clusters of a specific zone must have access to all Datastores within the same zone.
  4. The cluster with the required ESXis.
     * A tag from the tag category in `zoneTagCategory` (`k8s-zone` by default) **must be added** to the Cluster. This tag will indicate the **zone**.
  5. Folder for VirtualMachines to be created.
     * An optional parameter. By default, the root vm folder is used.
  6. Create a role with the appropriate [set](#list-of-privileges-for-using-the-module) of privileges.
  7. Create a user and assign the above role to it.
* A tag from the tag category in `regionTagCategory` (`k8s-region` by default) **must be added** to the Datacenter. This tag will indicate the region.

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
        <td><code>Network.Assign</code><br><code>Resource.ApplyRecommendation</code><br><code>Resource.AssignVAppToPool</code><br><code>Resource.AssignVMToPool</code><br><code>Resource.ColdMigrate</code><br><code>Resource.CreatePool</code><br><code>Resource.DeletePool</code><br><code>Resource.EditPool</code><br><code>Resource.HotMigrate</code><br><code>Resource.MovePool</code><br><code>Resource.QueryVMotion</code><br><code>Resource.RenamePool</code><br><code>VirtualMachine.Config.AddExistingDisk</code><br><code>VirtualMachine.Config.AddNewDisk</code><br><code>VirtualMachine.Config.AddRemoveDevice</code><br><code>VirtualMachine.Config.AdvancedConfig</code><br><code>VirtualMachine.Config.Annotation</code><br><code>VirtualMachine.Config.ChangeTracking</code><br><code>VirtualMachine.Config.CPUCount</code><br><code>VirtualMachine.Config.DiskExtend</code><br><code>VirtualMachine.Config.DiskLease</code><br><code>VirtualMachine.Config.EditDevice</code><br><code>VirtualMachine.Config.HostUSBDevice</code><br><code>VirtualMachine.Config.ManagedBy</code><br><code>VirtualMachine.Config.Memory</code><br><code>VirtualMachine.Config.MksControl</code><br><code>VirtualMachine.Config.QueryFTCompatibility</code><br><code>VirtualMachine.Config.QueryUnownedFiles</code><br><code>VirtualMachine.Config.RawDevice</code><br><code>VirtualMachine.Config.ReloadFromPath</code><br><code>VirtualMachine.Config.RemoveDisk</code><br><code>VirtualMachine.Config.Rename</code><br><code>VirtualMachine.Config.ResetGuestInfo</code><br><code>VirtualMachine.Config.Resource</code><br><code>VirtualMachine.Config.Settings</code><br><code>VirtualMachine.Config.SwapPlacement</code><br><code>VirtualMachine.Config.ToggleForkParent</code><br><code>VirtualMachine.Config.UpgradeVirtualHardware</code><br><code>VirtualMachine.GuestOperations.Execute</code><br><code>VirtualMachine.GuestOperations.Modify</code><br><code>VirtualMachine.GuestOperations.ModifyAliases</code><br><code>VirtualMachine.GuestOperations.Query</code><br><code>VirtualMachine.GuestOperations.QueryAliases</code><br><code>VirtualMachine.Hbr.ConfigureReplication</code><br><code>VirtualMachine.Hbr.MonitorReplication</code><br><code>VirtualMachine.Hbr.ReplicaManagement</code><br><code>VirtualMachine.Interact.AnswerQuestion</code><br><code>VirtualMachine.Interact.Backup</code><br><code>VirtualMachine.Interact.ConsoleInteract</code><br><code>VirtualMachine.Interact.CreateScreenshot</code><br><code>VirtualMachine.Interact.CreateSecondary</code><br><code>VirtualMachine.Interact.DefragmentAllDisks</code><br><code>VirtualMachine.Interact.DeviceConnection</code><br><code>VirtualMachine.Interact.DisableSecondary</code><br><code>VirtualMachine.Interact.DnD</code><br><code>VirtualMachine.Interact.EnableSecondary</code><br><code>VirtualMachine.Interact.GuestControl</code><br><code>VirtualMachine.Interact.MakePrimary</code><br><code>VirtualMachine.Interact.Pause</code><br><code>VirtualMachine.Interact.PowerOff</code><br><code>VirtualMachine.Interact.PowerOn</code><br><code>VirtualMachine.Interact.PutUsbScanCodes</code><br><code>VirtualMachine.Interact.Record</code><br><code>VirtualMachine.Interact.Replay</code><br><code>VirtualMachine.Interact.Reset</code><br><code>VirtualMachine.Interact.SESparseMaintenance</code><br><code>VirtualMachine.Interact.SetCDMedia</code><br><code>VirtualMachine.Interact.SetFloppyMedia</code><br><code>VirtualMachine.Interact.Suspend</code><br><code>VirtualMachine.Interact.SuspendToMemory</code><br><code>VirtualMachine.Interact.TerminateFaultTolerantVM</code><br><code>VirtualMachine.Interact.ToolsInstall</code><br><code>VirtualMachine.Interact.TurnOffFaultTolerance</code><br><code>VirtualMachine.Inventory.Create</code><br><code>VirtualMachine.Inventory.CreateFromExisting</code><br><code>VirtualMachine.Inventory.Delete</code><br><code>VirtualMachine.Inventory.Move</code><br><code>VirtualMachine.Inventory.Register</code><br><code>VirtualMachine.Inventory.Unregister</code><br><code>VirtualMachine.Namespace.Event</code><br><code>VirtualMachine.Namespace.EventNotify</code><br><code>VirtualMachine.Namespace.Management</code><br><code>VirtualMachine.Namespace.ModifyContent</code><br><code>VirtualMachine.Namespace.Query</code><br><code>VirtualMachine.Namespace.ReadContent</code><br><code>VirtualMachine.Provisioning.Clone</code><br><code>VirtualMachine.Provisioning.CloneTemplate</code><br><code>VirtualMachine.Provisioning.CreateTemplateFromVM</code><br><code>VirtualMachine.Provisioning.Customize</code><br><code>VirtualMachine.Provisioning.DeployTemplate</code><br><code>VirtualMachine.Provisioning.DiskRandomAccess</code><br><code>VirtualMachine.Provisioning.DiskRandomRead</code><br><code>VirtualMachine.Provisioning.FileRandomAccess</code><br><code>VirtualMachine.Provisioning.GetVmFiles</code><br><code>VirtualMachine.Provisioning.MarkAsTemplate</code><br><code>VirtualMachine.Provisioning.MarkAsVM</code><br><code>VirtualMachine.Provisioning.ModifyCustSpecs</code><br><code>VirtualMachine.Provisioning.PromoteDisks</code><br><code>VirtualMachine.Provisioning.PutVmFiles</code><br><code>VirtualMachine.Provisioning.ReadCustSpecs</code><br><code>VirtualMachine.State.CreateSnapshot</code><br><code>VirtualMachine.State.RemoveSnapshot</code><br><code>VirtualMachine.State.RenameSnapshot</code><br><code>VirtualMachine.State.RevertToSnapshot</code></td>
        <td>To manage the virtual machines lifecycle in a Deckhouse Kubernetes Platform cluster.</td>
    </tr>
  </tbody>
</table>
