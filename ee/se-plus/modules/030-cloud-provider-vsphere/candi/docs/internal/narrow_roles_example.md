# RBAC config

## Role-bindings

### vSphere

`Read-only` role without inheritance.

### Datacenter

`Read-only` role without inheritance.

### Hosts and Clusters

`Full` on `Cluster` with inheritance.

> There seems to be a potential for narrower Role.

### Networking

Tested only on Distributed vSwitch.

* Distributed switch — `Read-only` without inheritance.
* Distributed port group — `Full` with inheritance.

### Storage

`Full` role on folder with Volumes with inheritance.

### VMs and Templates

`Full` role on `vmFolderPath` with inheritance.

## Roles

### Read-only

```text
govc role.create kubernetes-ro \
  InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory InventoryService.Tagging.CreateScope \
  InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory InventoryService.Tagging.DeleteScope \
  InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory InventoryService.Tagging.EditTag \
  InventoryService.Tagging.ModifyUsedByForCategory InventoryService.Tagging.ModifyUsedByForTag Resource.AssignVMToPool \
  StorageProfile.View StorageViews.View System.Anonymous System.Read System.View
```

### Full

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
```
