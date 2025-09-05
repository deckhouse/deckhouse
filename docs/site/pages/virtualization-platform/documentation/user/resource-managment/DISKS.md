---
title: "Disks"
permalink: en/virtualization-platform/documentation/user/resource-management/disks.html
---

Disks in virtual machines ([VirtualDisk](../../../reference/cr/virtualdisk.html) resources) are essential for writing and storing data. They ensure the proper functioning of applications and operating systems. The structure of these disks includes storage provided by the platform.

Depending on the storage properties, disks during creation and virtual machines during operation may exhibit different behaviors.

`VolumeBindingMode` properties:

`Immediate` —  disk is created immediately after the resource is created (it is assumed that the disk will be available for attachment to a virtual machine on any cluster node).  

![Immediate](/../../../../images/virtualization-platform/vd-immediate.png)

`WaitForFirstConsumer` — disk is created only after it is attached to a virtual machine and will be created on the node where the virtual machine is launched.  

![WaitForFirstConsumer](/../../../../images/virtualization-platform/vd-wffc.ru.png)

AccessMode:

`ReadWriteOnce (RWO)` — access to the disk is granted to only one instance of a virtual machine. Live migration of virtual machines with such disks is not possible.

`ReadWriteMany (RWX)` — multiple access to the disk is allowed. Live migration of virtual machines with such disks is possible.

When a disk is created, the controller automatically determines the most optimal parameters supported by the storage.

> **Warning** Creating disks from ISO images is not allowed.

To find the available storage options on the platform, run the following command:

```bash
d8 k get storageclass
```

Example output:

```console
NAME                          PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
NAME                                 PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
i-sds-replicated-thin-r1 (default)   replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
i-sds-replicated-thin-r2             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
i-sds-replicated-thin-r3             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
sds-replicated-thin-r1               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
sds-replicated-thin-r2               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
sds-replicated-thin-r3               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
nfs-4-1-wffc                         nfs.csi.k8s.io                        Delete          WaitForFirstConsumer   true                   30d
```

The `(default)` marker next to the class name indicates that this `StorageClass` will be used by default if the user has not explicitly specified the class name in the resource being created.
If the `StorageClass` is missing by default in the cluster, the user must explicitly specify the required `StorageClass` in the resource specification.
Deckhouse Virtualization Platform also allows you to set individual settings for storing disks and images.

How to find out the available storage options on the platform in the web interface:

- Go to the "System" tab, then to the "Storage" section → "Storage Classes".

## Storage class settings for disks

The storage class settings for disks are defined in the `.spec.settings.virtualDisks` parameter of the module settings.
Example:

```yaml
spec:
...
settings:
virtualDisks:
allowedStorageClassNames:
- sc-1
- sc-2
defaultStorageClassName: sc-1
```

- `allowedStorageClassNames` — (optional) is a list of valid `StorageClass` for creating a `VirtualDisk`, which can be explicitly specified in the resource specification.
- `defaultStorageClassName` — (optional) is the `StorageClass` used by default when creating a `VirtualDisk` if the `.spec.persistentVolumeClaim.storageClassName` parameter is not specified.

## Fine-tuning storage classes for disks

When creating a disk, the controller will automatically select the most optimal parameters supported by the storage based on the data it knows.
Priorities for configuring `PersistentVolumeClaim` parameters when creating a disk by automatically detecting storage characteristics:

- RWX + Block
- RWX + FileSystem
- RWO + Block
- RWO + FileSystem.
  
If the storage is unknown and it is impossible to determine its parameters automatically, the mode is used: RWO + FileSystem

## Creating an empty disk

Empty disks are typically used for installing operating systems or storing data.

To create a disk use:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: blank-disk
spec:
  # Disk storage settings.
  persistentVolumeClaim:
    # Replace with your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
    size: 100Mi
EOF
```

After creation, the [VirtualDisk](../../../reference/cr/virtualdisk.html) resource can be in the following states:

- `Pending`: Waiting for readiness of all dependent resources required for disk creation.
- `Provisioning`: The disk creation process is ongoing.
- `Resizing`: The disk resizing process is ongoing.
- `WaitForFirstConsumer`: The disk is waiting for a virtual machine that will use it.
- `WaitForUserUpload` - the disk is waiting for the user to upload an image (type: Upload).
- `Ready`: The disk is created and ready for use.
- `Failed`: An error occurred during the creation process.
- `PVCLost` - system error, PVC with data has been lost.
- `Terminating` - the disk is being deleted. The disk may "hang" in this state if it is still connected to the virtual machine.

Until the disk reaches the `Ready` phase, the entire `.spec` block can be modified. Changing it will restart the disk creation process.

If the `.spec.persistentVolumeClaim.storageClassName` parameter is not specified, the default `StorageClass` at the cluster level will be used, or for images if specified in [virtualization settings](../../admin/install/steps/virtualization.html#parameter-description).

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block

Check the disk's status after creation:

```bash
d8 k get vd blank-disk
```

Example output:

```console
NAME         PHASE     CAPACITY   AGE
blank-disk   Ready     100Mi      1m2s
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

Disks can be created and populated with data from previously created images such as [ClusterVirtualImage](../../../reference/cr/clustervirtualimage.html) and [VirtualImage](../../../reference/cr/virtualimage.html).

When creating a disk, you can specify its desired size, which must be equal to or greater than the unpacked size of the image. If the size is not specified, the disk will be created with the same size as the source disk image.

Using a previously created project image [VirtualImage](../../../reference/cr/virtualimage.html), here’s an example command to determine the size of the unpacked image:

```bash
d8 k get vi ubuntu-22-04 -o wide
```

Example output:

```console
NAME           PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                                                       AGE
ubuntu-22-04   Ready   false   100%       285.9Mi      2.5Gi          dvcr.d8-virtualization.svc/cvi/ubuntu-22.04:eac95605-7e0b-4a32-bb50-cc7284fd89d0   122m
```

The required size is indicated in the UNPACKEDSIZE column and is 2.5Gi.

Create a disk from this image:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Disk storage parameters configuration.
  persistentVolumeClaim:
    # Specify a size greater than the unpacked image size.
    size: 10Gi
    # Substitute with your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
  # The source from which the disk is created.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22-04
EOF
```

Now, create a disk without specifying its size:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root-2
spec:
  # Disk storage parameters configuration.
  persistentVolumeClaim:
    # Substitute with your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
  # The source from which the disk is created.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22-04
EOF
```

Check the state of the disks after creation:

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

## Resizing a disk

The size of disks can be increased even if they are already attached to a running virtual machine. Changes are made to the `spec.persistentVolumeClaim`.size field:

Check the size before the change:

```bash
d8 k get vd linux-vm-root
```

Example output:

```console
NAME          PHASE   CAPACITY   AGE
linux-vm-root   Ready   10Gi       10m
```

Apply the changes:

```bash
d8 k patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'
```

Check the size after the change:

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
