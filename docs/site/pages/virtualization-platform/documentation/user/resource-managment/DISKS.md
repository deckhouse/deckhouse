---
title: "Disks"
permalink: en/virtualization-platform/documentation/user/resource-management/disks.html
---

Disks in virtual machines are necessary for writing and storing data, ensuring that applications and operating systems can fully function. DVP provides the storage for these disks.

Depending on the storage properties, the behavior of disks during creation of virtual machines during operation may differ:

The behavior of disks during their creation depends on the `VolumeBindingMode` parameter, which defines when exactly the disk is created and on which node:

`Immediate`: The disk is created immediately after the resource is created (the disk is assumed to be available for connection to a virtual machine on any node in the cluster).

![Immediate](/images/virtualization-platform/vd-immediate.png)

`WaitForFirstConsumer`: The disk is created only after it is connected to the virtual machine and is created on the node on which the virtual machine will be running.

![WaitForFirstConsumer](/images/virtualization-platform/vd-wffc.png)

The `AccessMode` parameter determines how the virtual machine can access the disk — whether it is used exclusively by one VM or shared among several:

- `ReadWriteMany (RWX)`: Multiple disk access. Live migration of virtual machines with such disks is possible.
- `ReadWriteOnce (RWO)`: Only one instance of the virtual machine can access the disk. Live migration of virtual machines with such disks is supported only in DVP commercial editions. Live migration is only available if all disks are connected statically via (`.spec.blockDeviceRefs`). Disks connected dynamically via `VirtualMachineBlockDeviceAttachments` must be reattached statically by specifying them in `.spec.blockDeviceRefs`.

When creating a disk, the controller will independently determine the most optimal parameters supported by the storage.

{% alert level="warning" %}
It is impossible to create disks from ISO images.
{% endalert %}

To find out the available storage options, run the following command:

```bash
d8 k get storageclass
```

Example output:

```console
NAME                                 PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
i-sds-replicated-thin-r1 (default)   replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
i-sds-replicated-thin-r2             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
i-sds-replicated-thin-r3             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
sds-replicated-thin-r1               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
sds-replicated-thin-r2               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
sds-replicated-thin-r3               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
nfs-4-1-wffc                         nfs.csi.k8s.io                        Delete          WaitForFirstConsumer   true                   30d
```

A full description of the disk configuration settings can be found at [VirtualDisk resource documentation](/modules/virtualization/cr.html#virtualdisk).

How to find out the available storage options in the DVP web interface:

- Go to the "System" tab, then to the "Storage" section → "Storage Classes".

## Create an empty disk

Empty disks are usually used to install an OS on them, or to store some data.

Create a disk:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: blank-disk
spec:
  # Disk storage parameter settings.
  persistentVolumeClaim:
    # Substitute your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
    size: 100Mi
EOF
```

After creation, the `VirtualDisk` resource can be in the following states (phases):

- `Pending`: Waiting for all dependent resources required for disk creation to be ready.
- `Provisioning`: Disk creation process is in progress.
- `Resizing`: The process of resizing the disk is in progress.
- `WaitForFirstConsumer`: The disk is waiting for the virtual machine that will use it to be created.
- `WaitForUserUpload`: The disk is waiting for the user to upload an image (type: Upload).
- `Ready`: The disk has been created and is ready for use.
- `Failed`: An error occurred during the creation process.
- `PVCLost`: System error, PVC with data has been lost.
- `Terminating`: The disk is being deleted. The disk may "hang" in this state if it is still connected to the virtual machine.

As long as the disk has not entered the `Ready` phase, the contents of the entire `.spec` block can be changed. If changes are made, the disk creation process will start over.

If the `.spec.persistentVolumeClaim.storageClassName` parameter is not specified, the default `StorageClass` at the cluster level will be used, or for images if specified in [module settings](/products/virtualization-platform/documentation/admin/platform-management/virtualization/virtual-machine-classes.html).

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block

Check the status of the disk after creation with the command:

```bash
d8 k get vd blank-disk
```

Example output:

```console
NAME       PHASE   CAPACITY   AGE
blank-disk   Ready   100Mi      1m2s
```

How to create an empty disk in the web interface (this step can be skipped and performed when creating a VM):

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "VM Disks" section.
- Click "Create Disk".
- In the form that opens, enter `blank-disk` in the "Disk Name" field.
- In the "Size" field, set the size with the measurement units `100Mi`.
- In the "StorageClass Name" field, you can select a StorageClass or leave the default selection.
- Click the "Create" button.
- The disk status is displayed at the top left, under the disk name.

## Creating a disk from an image

A disk can also be created and populated with data from previously created `ClusterVirtualImage` and `VirtualImage` images.

When creating a disk, you can specify its desired size, which must be equal to or larger than the size of the extracted image. If no size is specified, a disk will be created with the size corresponding to the original disk image.

Using the example of the previously created image `VirtualImage`, let's consider the command that allows you to determine the size of the unpacked image:

```bash
d8 k get vi ubuntu-22-04 -o wide
```

Example output:

```console
NAME           PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                                                       AGE
ubuntu-22-04   Ready   false   100%       285.9Mi      2.5Gi          dvcr.d8-virtualization.svc/cvi/ubuntu-22-04:eac95605-7e0b-4a32-bb50-cc7284fd89d0   122m
```

The size you are looking for is specified in the **UNPACKEDSIZE** column and is 2.5Gi.

Let's create a disk from this image:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Disk storage parameter settings.
  persistentVolumeClaim:
    # Specify a size larger than the value of the unpacked image.
    size: 10Gi
    # Substitute your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
  # The source from which the disk is created.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22-04
EOF
```

Now create a disk, without explicitly specifying the size:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root-2
spec:
  # Disk storage settings.
  persistentVolumeClaim:
    # Substitute your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
  # The source from which the disk is created.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22-04
EOF
```

Check the status of the disks after creation:

```bash
d8 k get vd
```

Example output:

```console
NAME           PHASE   CAPACITY   AGE
linux-vm-root    Ready   10Gi       7m52s
linux-vm-root-2  Ready   2590Mi     7m15s
```

How to create a disk from an image in the web interface (this step can be skipped and performed when creating a VM):

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "VM Disks" section.
- Click "Create Disk".
- In the form that opens, enter `linux-vm-root` in the "Disk Name" field.
- In the "Source" field, make sure that the "Project" checkbox is selected.
- Select the image you want from the drop-down list.
- In the "Size" field, you can change the size to a larger one or leave the default selection.
- In the "StorageClass Name" field, you can select a StorageClass or leave the default selection.
- Click the "Create" button.
- The disk status is displayed at the top left, under the disk name.

## Change disk size

You can increase the size of disks even if they are already attached to a running virtual machine. To do this, edit the `spec.persistentVolumeClaim.size` field:

Check the size before the change:

```bash
d8 k get vd linux-vm-root
```

Example output:

```console
NAME          PHASE   CAPACITY   AGE
linux-vm-root   Ready   10Gi       10m
```

Let's apply the changes:

```bash
d8 k patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'
```

Let's check the size after the change:

```bash
d8 k get vd linux-vm-root
```

Example output:

```console
NAME          PHASE   CAPACITY   AGE
linux-vm-root   Ready   11Gi       12m
```

How to change the disk size in the web interface:

Method #1:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "VM Disks" section.
- Select the desired disk and click on the pencil icon in the "Size" column.
- In the pop-up window, you can change the size to a larger one.
- Click on the "Apply" button.
- The disk status is displayed in the "Status" column.

Method #2:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "VM Disks" section.
- Select the desired disk and click on its name.
- In the form that opens, on the "Configuration" tab, in the "Size" field, you can change the size to a larger one.
- Click on the "Save" button that appears.
- The disk status is displayed at the top left, under its name.

## Changing the disk StorageClass

In the DVP commercial editions, it is possible to change the StorageClass for existing disks. Currently, this is only supported for running VMs (`Phase` should be `Running`).

{% alert level="warning" %}
Storage class migration is only available for disks connected statically via `.spec.blockDeviceRefs`.

To migrate the storage class of disks attached via `VirtualMachineBlockDeviceAttachments`, they must be reattached statically by specifying disks names in `.spec.blockDeviceRefs`.
{% endalert %}

Example:

```bash
d8 k patch vd disk --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
```

After the disk configuration is updated, a live migration of the VM is triggered, during which the disk is migrated to the new storage.

If a VM has multiple disks attached and you need to change the storage class for several of them, this operation must be performed sequentially:

```bash
d8 k patch vd disk1 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
d8 k patch vd disk2 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
```

If migration fails, repeated attempts are made with increasing delays (exponential backoff algorithm). The maximum delay is 300 seconds (5 minutes). Delays: 5 seconds (1st attempt), 10 seconds (2nd), then each delay doubles, reaching 300 seconds (7th and subsequent attempts). The first attempt is performed without delay.

To cancel migration, the user must return the storage class in the specification to the original one.
