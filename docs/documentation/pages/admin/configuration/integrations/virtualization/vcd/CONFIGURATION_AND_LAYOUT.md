---
title: Layouts and configuration
permalink: en/admin/integrations/virtualization/vcd/сonfiguration-and-layout-scheme.html
---

## Layouts

Deckhouse Kubernetes Platform supports one layout for deploying resources in VCD.

### Standard

![Standard layout in VCD](../../../../images/cloud-provider-vcd/vcd-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11247&t=IvETjbByf1MSQzcm-0 --->

Example layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetwork: internal
    mainNetworkIPAddresses:
    - 192.168.199.2
```

## Configuration

Integration is performed using the VCDClusterConfiguration resource.
It defines the configuration of the cloud cluster in VCD
and is used by the virtualization system when the cluster control plane is hosted in the system.
The DKP module responsible for the integration is automatically configured based on the selected layout.

To modify the configuration in a running cluster, run the following command:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

{% alert level="info" %}
After changing node-related parameters, run the `dhctl converge` command to apply the changes.
{% endalert %}

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: VCDClusterConfiguration
sshPublicKey: "<SSH_PUBLIC_KEY>"
organization: My_Org
virtualDataCenter: My_Org
virtualApplicationName: Cloud
mainNetwork: internal
layout: Standard
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    sizingPolicy: 4cpu8ram
    rootDiskSizeGb: 20
    etcdDiskSizeGb: 20
    storageProfile: nvme
nodeGroups:
  - name: worker
    replicas: 1
    instanceClass:
      template: Org/Templates/ubuntu-focal-20.04
      sizingPolicy: 16cpu32ram
      storageProfile: ssd
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
  insecure: true
```

The number of nodes to be provisioned and their parameters are defined in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource,
where you also specify the name of the instance class used for that group (the `cloudInstances.classReference` parameter).
For the VCD cloud provider, the instance class is a custom resource called [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass),
which contains the specific configuration of the VMs.

The following is the example configuration of VCDInstanceClass for ephemeral nodes of the VMware Cloud Director cloud provider.

### Example configuration of VCDInstanceClass

```yaml
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: MyOrg/Linux/ubuntu2204-cloud-ova
```

### Storage

A StorageClass is automatically created for each Datastore and DatastoreCluster in the zone (or zones).

You can set the name of StorageClass that will be used in the cluster by default (the `default` parameter),
and filter out the unnecessary StorageClasses (the `exclude` parameter).

#### CSI

By default, the storage subsystem uses CNS volumes capable of on-the-fly resizing.
FCD volumes are also supported, but only in the legacy mode.
You can set this via the `compatibilityFlag` parameter.

#### Important information concerning the PVC size increase

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) of volume-resizer CSI and vSphere API,
you have to do the following after increasing the PVC size:

1. On the node where the Pod is located, run the `kubectl cordon <node_name>` command.
1. Delete the Pod.
1. Make sure that the resize was successful. The PVC object must *not have* the `Resizing` condition.
   > The `FileSystemResizePending` state is OK.
1. On the node where the Pod is located, run the `kubectl uncordon <node_name>` command.

### Environment requirements

* vSphere version required: `v7.0U2` ([required](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) for the `Online volume expansion` to work).
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
  6. Create a role with the appropriate [set](#list-of-required-privileges) of privileges.
  7. Create a user and assign the above role to it.
* A tag from the tag category in `regionTagCategory` (`k8s-region` by default) **must be added** to the Datacenter. This tag will indicate the **region**.

### List of required privileges

{% alert level="info" %}
Read [the documentation](environment.html#creating-and-assigning-a-role) on how to create and assign a role to a user.
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
