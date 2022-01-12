---
title: "Cloud provider — VMware vSphere: configuration"
---

The module is automatically enabled for all cloud clusters deployed in vSphere.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` parameter of NodeGroup). In the case of the vSphere cloud provider, the instance class is the [`VsphereInstanceClass`](cr.html#vsphereinstanceclass) custom resource that stores specific parameters of the machines.

## Parameters

The module settings are set automatically based on the placement strategy chosen. In most cases, you do not have to configure the module manually.

If you need to configure a module because, say, you have a bare metal cluster and you need to enable  adding additional instances from vSphere, then refer to the [How to configure a Hybrid cluster in vSphere](faq.html#how-do-i-create-a-hybrid-cluster) section.

**Note** that if the parameters provided below are changed (i.e., the parameters specified in the deckhouse ConfigMap), the **existing Machines are NOT redeployed** (new machines will be created with the updated parameters). Re-deployment is only performed when `NodeGroup` and `VsphereInstanceClass` are changed. You can learn more in the [node-manager module's documentation](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

* `host` — the domain of the vCenter server;
* `username` — the login ID;
* `password` — the password;
* `insecure` — can be set to `true` if vCenter has a self-signed certificate;
  * Format — bool;
  * An optional parameter; by default `false`;
* `vmFolderPath` — the path to the VirtualMachine Folder where the cloned VMs will be created;
  * e.g., `dev/test`;
* `regionTagCategory`— the name of the tag **category** used to identify the region (vSphere Datacenter);
  * Format — string;
  * An optional parameter; by default `k8s-region`;
* `zoneTagCategory` — the name of the tag **category** used to identify the region (vSphere Cluster).
    * Format — string;
    * An optional parameter; by default `k8s-zone`;

* `disableTimesync` — disable time synchronization on the vSphere side. **Note** that this parameter will not disable the NTP daemons in the guest OS, but only disable the time correction on the part of ESXi;
  * Format — bool.
  * An optional parameter; by default `true`;
* `region` — is a tag added to the vSphere Datacenter where all actions will occur: provisioning VirtualMachines, storing virtual disks on datastores, connecting to the network.
* `sshKeys` — a list of public SSH keys in plain-text format;
  * Format — an array of strings;
  * An optional parameter; by default there are no allowed keys for the user;
* `externalNetworkNames` — names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert ExternalIP into the `.status.addresses` field in the Node API object.
  * Format — an array of strings; for example,

  Example:
  ```yaml
  externalNetworkNames:
  - MAIN-1
  - public
  ```

  * An optional parameter
* `internalNetworkNames` — names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert InternalIP into the `.status.addresses` field in the Node API object.
  * Format — an array of strings; for example,

  Example:
  ```yaml
  internalNetworkNames:
  - KUBE-3
  - devops-internal
  ```

  * An optional parameter.

## Storage

The module automatically creates a StorageClass for each Datastore and DatastoreCluster in the zone(-s). Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
  * Format — an array of strings;
  * An optional parameter;
* `default` — the name of StorageClass that will be used in the cluster by default;
  * Format — a string;
  * An optional parameter;
  * If the parameter is omitted, the default StorageClass is either:
    * an arbitrary StorageClass present in the cluster that has the default annotation;
    * the first (in lexicographic order) StorageClass of those created by the module.

Example:
```yaml
cloudProviderVsphere: |
  storageClass:
    exclude:
    - ".*-lun101-.*"
    - slow-lun103-1c280603
    default: fast-lun102-7d0bf578
```

### CSI

By default, the storage subsystem uses CNS volumes with the ability of online-resize. FCD volumes are also supported, but only in the legacy or migration modes.

* `compatibilityFlag` — a flag allowing the use of the old CSI version;
  * Format — a string;
  * Possible values:
    * `legacy` — use the old version of the driver. FCD discs only, no online-resizing;
    * `migration` — in this case, both drivers will be available in the cluster at the same time. This mode is used to migrate from an old driver.
  * An optional parameter;

```yaml
cloudProviderVsphere: |
  storageClass:
    compatibilityFlag: legacy
```

### Important information concerning the increase of the PVC size

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) f volume-resizer, CSI, and vSphere API, you have to do the following after increasing the PVC size:

1. Run the `kubectl cordon node_where_pod_is_hosted` command;
2. Delete the Pod;
3. Make sure that the resize was successful. The PVC object must *not have* the `Resizing` condition. **Note** that the `FileSystemResizePending` state is OK;
4. Run the `kubectl uncordon node_where_pod_is_hosted` command;

## Environment requirements

1. vSphere version required: `v7.0U2` ([required](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) for the `Online volume expansion` work); 
2. vCenter to which master nodes can connect to from within the cluster;
3. Datacenter with the following components:

    1. VirtualMachine template with a [specific](https://github.com/vmware/cloud-init-vmware-guestinfo) cloud-init datasource.
       * VM image should use `Virtual machines with hardware version 15 or later` (required for online resize to work).
    2. The network must be available on all ESXi where VirtualMachines will be created.
    3. One or more Datastores connected to all ESXi where VirtualMachines will be created.
        * A tag from the tag category in `zoneTagCategory` (`k8s-zone` by default) **must be added** to Datastores. This tag will indicate the **zone**.  All Clusters of a specific zone must have access to all Datastores within the same zone.
    4. The cluster with the required ESXis.
        * A tag from the tag category in `zoneTagCategory` (`k8s-zone` by default) **must be added** to the Cluster. This tag will indicate the **zone**.
    5. Folder for VirtualMachines to be created.
        * An optional parameter. By default, the root vm folder is used.
    6. Create a role with the appropriate [set](#list-of-privileges-for-using-the-module) of privileges.
    7. Create a user and assign the above role to it.

4. A tag from the tag category in `regionTagCategory` (`k8s-region` by default) **must be added** to the Datacenter. This tag will indicate the region.

## List of privileges for using the module

```none
Datastore.AllocateSpace
Datastore.FileManagement
Global.GlobalTag
Global.SystemTag
InventoryService.Tagging.AttachTag
InventoryService.Tagging.CreateCategory
InventoryService.Tagging.CreateTag
InventoryService.Tagging.DeleteCategory
InventoryService.Tagging.DeleteTag
InventoryService.Tagging.EditCategory
InventoryService.Tagging.EditTag
InventoryService.Tagging.ModifyUsedByForCategory
InventoryService.Tagging.ModifyUsedByForTag
Network.Assign
Resource.AssignVMToPool
StorageProfile.View
System.Anonymous
System.Read
System.View
VirtualMachine.Config.AddExistingDisk
VirtualMachine.Config.AddNewDisk
VirtualMachine.Config.AddRemoveDevice
VirtualMachine.Config.AdvancedConfig
VirtualMachine.Config.Annotation
VirtualMachine.Config.CPUCount
VirtualMachine.Config.ChangeTracking
VirtualMachine.Config.DiskExtend
VirtualMachine.Config.DiskLease
VirtualMachine.Config.EditDevice
VirtualMachine.Config.HostUSBDevice
VirtualMachine.Config.ManagedBy
VirtualMachine.Config.Memory
VirtualMachine.Config.MksControl
VirtualMachine.Config.QueryFTCompatibility
VirtualMachine.Config.QueryUnownedFiles
VirtualMachine.Config.RawDevice
VirtualMachine.Config.ReloadFromPath
VirtualMachine.Config.RemoveDisk
VirtualMachine.Config.Rename
VirtualMachine.Config.ResetGuestInfo
VirtualMachine.Config.Resource
VirtualMachine.Config.Settings
VirtualMachine.Config.SwapPlacement
VirtualMachine.Config.ToggleForkParent
VirtualMachine.Config.UpgradeVirtualHardware
VirtualMachine.GuestOperations.Execute
VirtualMachine.GuestOperations.Modify
VirtualMachine.GuestOperations.ModifyAliases
VirtualMachine.GuestOperations.Query
VirtualMachine.GuestOperations.QueryAliases
VirtualMachine.Hbr.ConfigureReplication
VirtualMachine.Hbr.MonitorReplication
VirtualMachine.Hbr.ReplicaManagement
VirtualMachine.Interact.AnswerQuestion
VirtualMachine.Interact.Backup
VirtualMachine.Interact.ConsoleInteract
VirtualMachine.Interact.CreateScreenshot
VirtualMachine.Interact.CreateSecondary
VirtualMachine.Interact.DefragmentAllDisks
VirtualMachine.Interact.DeviceConnection
VirtualMachine.Interact.DisableSecondary
VirtualMachine.Interact.DnD
VirtualMachine.Interact.EnableSecondary
VirtualMachine.Interact.GuestControl
VirtualMachine.Interact.MakePrimary
VirtualMachine.Interact.Pause
VirtualMachine.Interact.PowerOff
VirtualMachine.Interact.PowerOn
VirtualMachine.Interact.PutUsbScanCodes
VirtualMachine.Interact.Record
VirtualMachine.Interact.Replay
VirtualMachine.Interact.Reset
VirtualMachine.Interact.SESparseMaintenance
VirtualMachine.Interact.SetCDMedia
VirtualMachine.Interact.SetFloppyMedia
VirtualMachine.Interact.Suspend
VirtualMachine.Interact.TerminateFaultTolerantVM
VirtualMachine.Interact.ToolsInstall
VirtualMachine.Interact.TurnOffFaultTolerance
VirtualMachine.Inventory.Create
VirtualMachine.Inventory.CreateFromExisting
VirtualMachine.Inventory.Delete
VirtualMachine.Inventory.Move
VirtualMachine.Inventory.Register
VirtualMachine.Inventory.Unregister
VirtualMachine.Namespace.Event
VirtualMachine.Namespace.EventNotify
VirtualMachine.Namespace.Management
VirtualMachine.Namespace.ModifyContent
VirtualMachine.Namespace.Query
VirtualMachine.Namespace.ReadContent
VirtualMachine.Provisioning.Clone
VirtualMachine.Provisioning.CloneTemplate
VirtualMachine.Provisioning.CreateTemplateFromVM
VirtualMachine.Provisioning.Customize
VirtualMachine.Provisioning.DeployTemplate
VirtualMachine.Provisioning.DiskRandomAccess
VirtualMachine.Provisioning.DiskRandomRead
VirtualMachine.Provisioning.FileRandomAccess
VirtualMachine.Provisioning.GetVmFiles
VirtualMachine.Provisioning.MarkAsTemplate
VirtualMachine.Provisioning.MarkAsVM
VirtualMachine.Provisioning.ModifyCustSpecs
VirtualMachine.Provisioning.PromoteDisks
VirtualMachine.Provisioning.PutVmFiles
VirtualMachine.Provisioning.ReadCustSpecs
VirtualMachine.State.CreateSnapshot
VirtualMachine.State.RemoveSnapshot
VirtualMachine.State.RenameSnapshot
VirtualMachine.State.RevertToSnapshot
```
