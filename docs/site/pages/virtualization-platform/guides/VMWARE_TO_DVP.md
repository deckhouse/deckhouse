---
title: Migrating VMs from VMware to DVP
permalink: en/virtualization-platform/guides/migrating-vms-from-vmware-to-dvp.html
description: A short guide for migrating virtual machines from VMware (OVA/VMDK) to Deckhouse Virtualization Platform.
lang: en
layout: sidebar-guides
---

This guide describes how to migrate an existing virtual machine from VMware to Deckhouse Virtualization Platform (DVP).

The migration source can be:

- a virtual machine distribution (an `OVA` file, a tar archive with disk files in `vmdk` format, VM metadata in `ovf` format, and checksums in `mf` format);
- standalone `VMDK` disk files.

## Migration approaches

### Direct VMDK import

DVP supports importing disks in `vmdk` format.
You can upload a `VMDK` file from an OVA or vSphere export to a [`VirtualImage`](/modules/virtualization/cr.html#virtualimage) or [`ClusterVirtualImage`](/modules/virtualization/cr.html#clustervirtualimage), create a disk from the image, and then a virtual machine.
See [Images](/products/virtualization-platform/documentation/user/resource-management/images.html#load-an-image-from-the-command-line) for the upload procedure.

{% alert level="warning" %}
The platform imports the disk file as-is and does not adapt the guest OS for KVM.
For disks from VMware this often causes problems: the VM fails to boot, cannot see the disk, or has no network (especially on Windows).
Direct import is suitable only when the `VMDK` is already prepared for QEMU/KVM.
{% endalert %}

### Recommended path

For typical VMs from VMware, use the `virt-v2v` utility: it converts `VMDK` to `qcow2` and adapts the guest OS for KVM (virtio drivers, bootloader, replacement of VMware devices).
Upload the prepared disk directly to a [`VirtualDisk`](/modules/virtualization/cr.html#virtualdisk) with `type: Upload`.
The volume is provisioned in the chosen StorageClass, bypassing DVCR.

The step-by-step instructions below cover this scenario.

## Migration stages

Migrating a VM from VMware to DVP includes the following stages:

1. [Install the required tools](#install-tools).
1. [Convert the disk](#convert-the-disk).
1. [Upload the disk to the cluster](#upload-the-disk-to-the-cluster) as a [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) resource.
1. [Create a virtual machine](#create-the-virtual-machine) ([VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource) that boots from this disk.

## What you need for migration

Before you start the migration, make sure you have:

- access to the DVP cluster with the Deckhouse CLI utility (`d8`) installed and permissions to create virtualization resources in the target namespace;
- a Linux host with `virt-v2v` and `libguestfs` installed and enough disk space to unpack the `OVA` (or standalone `VMDK` files) and store the conversion output;
- the VMware export files (`OVA` or `VMDK`).

For more details on uploading disks to the cluster, see [Disks](/products/virtualization-platform/documentation/user/resource-management/disks.html).

## Install tools

On this step you prepare a conversion workstation.
It does not have to be a DVP cluster node: any Linux host with internet access or a local package repository is sufficient.

The packages you need depend on the guest OS in the VM you are migrating:

- for Linux, `virt-v2v` and `libguestfs` are sufficient;
- for Windows, you will additionally need a `virtio-win` ISO so the guest OS works correctly with virtual devices in KVM after migration.

Install the tools:

{% tabs os %}
{% tab "Ubuntu/Debian workstation" %}

Run the following command:

```bash
sudo apt update
sudo apt install -y virt-v2v libguestfs-tools
```

If the VM being migrated runs Windows:

1. [Download the VirtIO drivers](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/) from the `virtio-win` distribution.

1. Specify the path to the VirtIO drivers via an environment variable:

   ```bash
   export VIRTIO_WIN=/path/to/virtio-win.iso
   ```

{% alert level="warning" %}
Without a valid `virtio-win` ISO for a Windows guest, conversion may fail, or the guest OS may not see disks or networking after the VM starts on DVP.
{% endalert %}

{% endtab %}
{% tab "RHEL/AlmaLinux workstation" %}
Run the following command:

```bash
sudo dnf install -y virt-v2v libguestfs-tools-c virtio-win
```

If the VM being migrated runs Windows, set the path to the ISO via an environment variable:

```bash
export VIRTIO_WIN=/path/to/virtio-win.iso
```

{% endtab %}
{% endtabs %}

Then proceed to converting the disk.

## Convert the disk

On this step you convert VMware data into one or more `qcow2` files that DVP can use as virtual machine volumes.
If you already have a ready `VMDK`, go straight to [Convert VMDK to qcow2 via virt-v2v](#convert-vmdk-to-qcow2-via-virt-v2v).
If you are using a virtual machine `OVA` distribution, [unpack it first](#extract-an-ova).

### Extract an OVA

An `OVA` file is a tar archive with a manifest, an `OVF` descriptor, and one or more `VMDK` images.
Unpacking gives you the disk file path for `virt-v2v`.
Keep the `OVF` file: you will use it later for CPU, memory, and bootloader settings in the VirtualMachine resource.

Extract everything if you want to verify checksums or inspect the OVF:

```bash
tar -xvf machine.ova
```

Typical contents:

```text
machine.ova
├── machine.mf          # checksums (SHA256)
├── machine.ovf         # VM metadata (CPU, RAM, disks, networks)
└── machine-disk1.vmdk  # disk image
```

If the archive is large, you can extract only the required `VMDK` listed in the `OVF`:

```bash
tar -xvf machine.ova machine-disk1.vmdk
```

{% alert level="info" %}
Virtual machines with multiple disks contain several `*.vmdk` files. Convert each disk with the `virt-v2v` utility, create a matching `VirtualDisk` in DVP, then reference them from `VirtualMachine` in the desired boot order.
{% endalert %}

### Convert VMDK to qcow2 via virt-v2v

With `-i disk`, `virt-v2v` processes a local `VMDK` and saves the result into the directory you specify.
To perform the conversion, run:

{% tabs os_convert %}
{% tab "For a Linux guest OS" %}

```bash
virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

{% endtab %}
{% tab "For a Windows guest OS" %}

To convert a `VMDK` for a Windows guest, specify the path to `virtio-win.iso` in the command:

```bash
VIRTIO_WIN=/path/to/virtio-win.iso virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

{% endtab %}
{% endtabs %}

After the conversion, a file such as `./out/machine.qcow2` will appear under `./out` (the basename often matches the original VM name from the metadata). That file is what you upload to the cluster next.

## Upload the disk to the cluster

This section describes how to transfer the prepared `qcow2` image to DVP through the Kubernetes API.
At this stage the file becomes a persistent volume in the cluster: create a VirtualDisk with `type: Upload` and transfer the `qcow2` over HTTP.
The disk is provisioned in the chosen StorageClass, bypassing DVCR.

Uploading the disk image to the cluster includes the following steps:

1. Choose a StorageClass.
1. Create a VirtualDisk for upload.
1. Get upload URLs.
1. Upload the image.
1. Check the status of the uploaded image.

### Choose a StorageClass

StorageClass in Kubernetes defines where and how the volume is provisioned; in VMware terms this is closest to a `datastore`. Performance, replication type, and volume expansion policy depend on the class.

List classes available in your cluster:

```bash
d8 k get storageclass
```

Example:

```console
NAME                 PROVISIONER                             VOLUMEBINDINGMODE   AGE
rv-thin-r1 (default) replicated.csi.storage.deckhouse.io     Immediate           48d
rv-thin-r2           replicated.csi.storage.deckhouse.io     Immediate           48d
```

Select the class name that fits your storage requirements for VM disks.

### Create a VirtualDisk for upload

Create the disk resource, specifying StorageClass and volume size.
`spec.persistentVolumeClaim.size` must be at least the actual size of the `qcow2` you upload.
If unsure, leave margin: if the size is insufficient, recreate the resource with a larger PVC.

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: uploaded-disk
spec:
  persistentVolumeClaim:
    storageClassName: rv-thin-r1
    size: 10Gi
  dataSource:
    type: Upload
EOF
```

After creation the resource moves to `WaitForUserUpload`: the volume is allocated and you can start transferring the file.

### Get upload URLs

The platform provides two URLs: internal (`imageUploadURLs.inCluster`) and external (`imageUploadURLs.external`). Use the address reachable from your network (from inside the cluster or from an administrator workstation).

Internal URL (use when uploading from a cluster node or from a pod):

```bash
d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs.inCluster}"
```

External URL (use from an administrator workstation when access to DVP is configured):

```bash
d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs.external}"
```

Both fields together (requires `jq`):

```bash
d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs}" | jq
```

{% alert level="warning" %}
The URL string contains a secret path segment. Do not publish it in public channels.
{% endalert %}

### Upload the image

Send the `qcow2` image with an HTTP `PUT` request to the URL obtained in the previous step. The example below uses an external URL. Replace it with the address from your `VirtualDisk` status and the path to the converted file.

```bash
curl https://virtualization.example.com/upload/<secret-url> \
    --progress-bar -T ./out/machine.qcow2 | cat
```

Wait until the transfer completes successfully without HTTP errors. The controller will process the image and transition the disk to `Ready`.

### Check status

Verify that the disk resource is healthy and the volume size looks correct:

```bash
d8 k get vd uploaded-disk
```

Example:

```console
NAMESPACE   NAME             PHASE   CAPACITY   AGE
default     uploaded-disk    Ready   10Gi       1m
```

If the phase stays on `WaitForUserUpload` for a long time or the resource enters `Failed`, check messages with `d8 k describe vd uploaded-disk` and events in the corresponding namespace.

Continue when the disk reaches `Ready`.

## Create the virtual machine

The final step is to describe the VM that will boot from the migrated disk.
Specify how much CPU and memory to allocate, which network to attach, and which disk is bootable.
VMware configuration (`OVF`/`VMX`) is not imported directly: carry over parameters manually using the `OVF` file and the mapping table below.

### VMware vs DVP terminology

If you know vSphere, the following table maps familiar VMware objects to Kubernetes and DVP virtualization resources.

| VMware                  | DVP                                                                                                                                                           | Description                      |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------|
| Datastore               | StorageClass                                                                                                                                                  | Disk backing storage             |
| VMX                     | [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).spec                                                                                         | VM specification                 |
| Virtual disk (VMDK)     | [VirtualDisk](/modules/virtualization/cr.html#virtualdisk)                                                                                                    | VM disk                          |
| ISO image               | [VirtualImage](/modules/virtualization/cr.html#virtualimage) (`cdrom: true`)                                                                                  | Installation or driver ISO       |
| Template                | [VirtualImage](/modules/virtualization/cr.html#virtualimage)                                                                                                  | Template for provisioning disks  |
| Port group / VLAN       | [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) (`networks`)                                                                                 | Networking                       |
| Resource pool           | Project and quotas                                                                                                                                            | Resource limits per project      |
| Snapshot                | [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) / [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot) | Disk and VM snapshots            |
| Folder                  | Namespace                                                                                                                                                     | Namespace                        |
| Cluster / resource pool | Project                                                                                                                                                       | Namespace grouping               |
| ESXi host               | Node                                                                                                                                                          | Physical server                  |
| vCenter                 | Kubernetes API                                                                                                                                                | Cluster management               |

For more details on connecting VMs to networks, see [Virtual machine networks](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html).

### VirtualMachine example

The VirtualMachine resource references the uploaded disk through `blockDeviceRefs`. Order in `blockDeviceRefs` determines boot order: the boot disk must be listed first.

Minimal Linux example after disk migration:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: my-vm
spec:
  virtualMachineClassName: generic
  osType: Generic
  cpu:
    cores: 2
  memory:
    size: 4Gi
  networks:
    - type: Main
  blockDeviceRefs:
    - kind: VirtualDisk
      name: uploaded-disk
EOF
```

Set `osType: Windows` for Windows guests.

If the source VM in VMware used UEFI boot, add `bootloader: EFI` (see the parameter table below).

If additional networks and the SDN module are configured in the cluster, you can add interfaces alongside the primary network:

```yaml
  networks:
    - type: Main
    - type: Network
      name: user-net
```

Additional capabilities (cloud-init, multiple disks, VM classes for production environments) are described in [Virtual machines](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

#### Common spec fields

The following table lists fields you most often need to verify after migration from VMware.
CPU, memory, and bootloader values can usually be taken from the unpacked `OVF` file.

| Field                         | Description                                                                                  |
|-------------------------------|----------------------------------------------------------------------------------------------|
| `virtualMachineClassName`     | VM class, e.g. `generic`, `serverful`, `high-performance`                                    |
| `osType`                      | `Generic` (Linux and others) or `Windows`                                                    |
| `bootloader`                  | Boot firmware: `BIOS`, `EFI`, or `EFIWithSecureBoot`; for UEFI VMs in VMware, set `EFI`        |
| `cpu.cores`                   | Number of vCPUs                                                                              |
| `memory.size`                 | RAM                                                                                          |
| `blockDeviceRefs`             | Disks and images; list order is boot order                                                     |
| `provisioning.type: UserData` | cloud-init user data for first boot of the guest OS                                          |

### Check VM status

After applying the manifest, wait until the VM is running and has an address (when the primary network assigns IPs from `virtualMachineCIDRs`):

```bash
d8 k get vm my-vm
```

Example:

```console
NAME    PHASE     NODE           IPADDRESS     AGE
my-vm   Running   virtlab-pt-2   10.66.10.12   2m
```

If the phase is `Pending` or startup fails, use `d8 k describe vm my-vm`, the serial console `d8 v console my-vm` (see [Connect to the VM](#connect-to-the-vm) below), and review virtualization component logs in the cluster.

### Connect to the VM

Choose an access method depending on whether the guest OS accepts SSH, you need a graphical console, or serial console is enough for boot troubleshooting.

| Method         | Purpose                                | Command                            |
|----------------|----------------------------------------|------------------------------------|
| Serial console | Bootloader and kernel output           | `d8 v console my-vm`               |
| VNC            | Graphical console without SSH          | `d8 v vnc my-vm`                   |
| SSH            | Remote shell access to the guest       | `d8 v ssh cloud@my-vm --local-ssh` |

The SSH username is defined in the guest OS; examples in the DVP documentation often create a `cloud` user via cloud-init.
