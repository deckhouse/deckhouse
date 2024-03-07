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

## List of privileges for using the module

> Read [the documentation](environment.html#creating-and-assigning-a-role) on how to create and assign a role to a user.

A detailed list of privileges required for Deckhouse Kubernetes Platform to work in vSphere:

| Required privileges in vSphere API | Description |
|------------------------------------|-------------|
| `Cns.Searchable`<br>`StorageProfile.View`<br>`Datastore.AllocateSpace`<br>`Datastore.Browse`<br>`Datastore.FileManagement` | To provision disks when creating virtual machines and ordering `PersistentVolumes` in a cluster. |
| `Global.GlobalTag`<br>`Global.SystemTag`<br>`InventoryService.Tagging.AttachTag`<br>`InventoryService.Tagging.CreateCategory`<br>`InventoryService.Tagging.CreateTag`<br>`InventoryService.Tagging.DeleteCategory`<br>`InventoryService.Tagging.DeleteTag`<br>`InventoryService.Tagging.EditCategory`<br>`InventoryService.Tagging.EditTag`<br>`InventoryService.Tagging.ModifyUsedByForCategory`<br>`InventoryService.Tagging.ModifyUsedByForTag`<br>`InventoryService.Tagging.ObjectAttachable` | Deckhouse Kubernetes Platform uses tags to identify the `Datacenter`, `Cluster` and `Datastore` objects available to it, as well as, to identify the virtual machines under its control. |
| `Folder.Create`<br>`Folder.Delete`<br>`Folder.Move`<br>`Folder.Rename` | To group a Deckhouse Kubernetes Platform cluster in a single `Folder` in vSphere Inventory. |
| `Network.Assign`<br>`Resource.ApplyRecommendation`<br>`Resource.AssignVAppToPool`<br>`Resource.AssignVMToPool`<br>`Resource.ColdMigrate`<br>`Resource.CreatePool`<br>`Resource.DeletePool`<br>`Resource.EditPool`<br>`Resource.HotMigrate`<br>`Resource.MovePool`<br>`Resource.QueryVMotion`<br>`Resource.RenamePool`<br>`VirtualMachine.Config.AddExistingDisk`<br>`VirtualMachine.Config.AddNewDisk`<br>`VirtualMachine.Config.AddRemoveDevice`<br>`VirtualMachine.Config.AdvancedConfig`<br>`VirtualMachine.Config.Annotation`<br>`VirtualMachine.Config.ChangeTracking`<br>`VirtualMachine.Config.CPUCount`<br>`VirtualMachine.Config.DiskExtend`<br>`VirtualMachine.Config.DiskLease`<br>`VirtualMachine.Config.EditDevice`<br>`VirtualMachine.Config.HostUSBDevice`<br>`VirtualMachine.Config.ManagedBy`<br>`VirtualMachine.Config.Memory`<br>`VirtualMachine.Config.MksControl`<br>`VirtualMachine.Config.QueryFTCompatibility`<br>`VirtualMachine.Config.QueryUnownedFiles`<br>`VirtualMachine.Config.RawDevice`<br>`VirtualMachine.Config.ReloadFromPath`<br>`VirtualMachine.Config.RemoveDisk`<br>`VirtualMachine.Config.Rename`<br>`VirtualMachine.Config.ResetGuestInfo`<br>`VirtualMachine.Config.Resource`<br>`VirtualMachine.Config.Settings`<br>`VirtualMachine.Config.SwapPlacement`<br>`VirtualMachine.Config.ToggleForkParent`<br>`VirtualMachine.Config.UpgradeVirtualHardware`<br>`VirtualMachine.GuestOperations.Execute`<br>`VirtualMachine.GuestOperations.Modify`<br>`VirtualMachine.GuestOperations.ModifyAliases`<br>`VirtualMachine.GuestOperations.Query`<br>`VirtualMachine.GuestOperations.QueryAliases`<br>`VirtualMachine.Hbr.ConfigureReplication`<br>`VirtualMachine.Hbr.MonitorReplication`<br>`VirtualMachine.Hbr.ReplicaManagement`<br>`VirtualMachine.Interact.AnswerQuestion`<br>`VirtualMachine.Interact.Backup`<br>`VirtualMachine.Interact.ConsoleInteract`<br>`VirtualMachine.Interact.CreateScreenshot`<br>`VirtualMachine.Interact.CreateSecondary`<br>`VirtualMachine.Interact.DefragmentAllDisks`<br>`VirtualMachine.Interact.DeviceConnection`<br>`VirtualMachine.Interact.DisableSecondary`<br>`VirtualMachine.Interact.DnD`<br>`VirtualMachine.Interact.EnableSecondary`<br>`VirtualMachine.Interact.GuestControl`<br>`VirtualMachine.Interact.MakePrimary`<br>`VirtualMachine.Interact.Pause`<br>`VirtualMachine.Interact.PowerOff`<br>`VirtualMachine.Interact.PowerOn`<br>`VirtualMachine.Interact.PutUsbScanCodes`<br>`VirtualMachine.Interact.Record`<br>`VirtualMachine.Interact.Replay`<br>`VirtualMachine.Interact.Reset`<br>`VirtualMachine.Interact.SESparseMaintenance`<br>`VirtualMachine.Interact.SetCDMedia`<br>`VirtualMachine.Interact.SetFloppyMedia`<br>`VirtualMachine.Interact.Suspend`<br>`VirtualMachine.Interact.SuspendToMemory`<br>`VirtualMachine.Interact.TerminateFaultTolerantVM`<br>`VirtualMachine.Interact.ToolsInstall`<br>`VirtualMachine.Interact.TurnOffFaultTolerance`<br>`VirtualMachine.Inventory.Create`<br>`VirtualMachine.Inventory.CreateFromExisting`<br>`VirtualMachine.Inventory.Delete`<br>`VirtualMachine.Inventory.Move`<br>`VirtualMachine.Inventory.Register`<br>`VirtualMachine.Inventory.Unregister`<br>`VirtualMachine.Namespace.Event`<br>`VirtualMachine.Namespace.EventNotify`<br>`VirtualMachine.Namespace.Management`<br>`VirtualMachine.Namespace.ModifyContent`<br>`VirtualMachine.Namespace.Query`<br>`VirtualMachine.Namespace.ReadContent`<br>`VirtualMachine.Provisioning.Clone`<br>`VirtualMachine.Provisioning.CloneTemplate`<br>`VirtualMachine.Provisioning.CreateTemplateFromVM`<br>`VirtualMachine.Provisioning.Customize`<br>`VirtualMachine.Provisioning.DeployTemplate`<br>`VirtualMachine.Provisioning.DiskRandomAccess`<br>`VirtualMachine.Provisioning.DiskRandomRead`<br>`VirtualMachine.Provisioning.FileRandomAccess`<br>`VirtualMachine.Provisioning.GetVmFiles`<br>`VirtualMachine.Provisioning.MarkAsTemplate`<br>`VirtualMachine.Provisioning.MarkAsVM`<br>`VirtualMachine.Provisioning.ModifyCustSpecs`<br>`VirtualMachine.Provisioning.PromoteDisks`<br>`VirtualMachine.Provisioning.PutVmFiles`<br>`VirtualMachine.Provisioning.ReadCustSpecs`<br>`VirtualMachine.State.CreateSnapshot`<br>`VirtualMachine.State.RemoveSnapshot`<br>`VirtualMachine.State.RenameSnapshot`<br>`VirtualMachine.State.RevertToSnapshot` | To manage the virtual machines lifecycle in a Deckhouse Kubernetes Platform cluster. |
