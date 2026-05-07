---
title: Migrating VMs from VMware to DVP
permalink: en/virtualization-platform/guides/vmware-to-dvp.html
description: A short guide for migrating virtual machines from VMware (OVA/VMDK) to Deckhouse Virtualization Platform.
lang: en
layout: sidebar-guides
---

This guide describes how to migrate an existing virtual machine from VMware to Deckhouse Virtualization Platform (DVP). Sources are commonly an `OVA` export or standalone `VMDK` files.

The procedure can be broken down into three steps:

1. On a separate workstation, prepare a disk image in `qcow2` format.
1. Upload the disk image to the cluster as a `VirtualDisk` resource.
1. Create a `VirtualMachine` that boots from that disk.

Linux guests typically only need packages from the distribution repositories. Windows guests also require a `virtio-win` ISO so that drivers match KVM virtio devices after migration.

Before you start, make sure you have:

- access to the DVP cluster with the Deckhouse CLI (`d8`) and permissions to create virtualization resources in the target namespace;
- a Linux machine (or equivalent) where `virt-v2v` and `libguestfs` can be installed, with enough disk space to unpack an `OVA` and store the converted image;
- the VMware export files (`OVA` or `VMDK`).

For more information about disks and ways to upload images, see [Disks](/products/virtualization-platform/documentation/user/resource-management/disks.html).

## Install tools

This step prepares a conversion workstation. You do not need to run it on a DVP cluster node; any Linux host with internet access or a local package repository is sufficient.

Use the commands that match your Linux distribution.

Ubuntu/Debian:

```bash
sudo apt update
sudo apt install -y virt-v2v libguestfs-tools
```

RHEL/AlmaLinux:

```bash
sudo dnf install -y virt-v2v libguestfs-tools-c virtio-win
```

Windows guests need VirtIO drivers from `virtio-win`. On RHEL/AlmaLinux the `virtio-win` package supplies them; on `Debian/Ubuntu` you usually download the ISO separately (for example from the [Fedora virtio-win stable builds](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/)) and export:

```bash
export VIRTIO_WIN=/path/to/virtio-win.iso
```

{% alert level="warning" %}
Without a valid `virtio-win` ISO, Windows conversion may fail, or the guest may boot without functional disks or networking on DVP.
{% endalert %}

Continue with extracting and converting the disk.

## Convert the disk

Here you turn VMware disk data into one or more `qcow2` files that DVP can use as virtual machine disk volumes. If you already have a `VMDK` path, skip ahead to the `virt-v2v` subsection; if you only have an `OVA`, unpack it first.

### Extract an OVA

An `OVA` file is a tar archive that bundles an OVF descriptor, a manifest, and one or more `VMDK` images. Unpacking exposes the disk files `virt-v2v` expects.

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
Virtual machines with multiple disks contain several `*.vmdk` files. Convert each disk with `virt-v2v`, create a matching `VirtualDisk` in DVP, then reference them from `VirtualMachine` in the desired boot order.
{% endalert %}

### Convert VMDK to qcow2 with virt-v2v

With `-i disk`, `virt-v2v` processes a local `VMDK` and saves the result into the directory you specify. For Windows guests, drivers from `virtio-win` are added to the image when `VIRTIO_WIN` points at the ISO.

Linux guest conversion (no extra ISO required when the guest is not Windows):

```bash
virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

Windows guests:

```bash
VIRTIO_WIN=/path/to/virtio-win.iso virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

You should see a file such as `./out/machine.qcow2` under `./out` (the basename often matches the original VM name). That file is what you upload next.

The following section explains how to transfer the `qcow2` image into the cluster through the Kubernetes API.

## Upload the disk image

At this stage the `qcow2` image becomes a persistent volume in the cluster. In DVP this is done with a `VirtualDisk` whose data source is `Upload`.

### Choose a StorageClass

StorageClass in Kubernetes defines where and how the volume is provisioned; in VMware terms this is closest to a `datastore`. Performance, replication type, and volume expansion policy depend on the class.

List classes available in your cluster:

```bash
d8 k get storageclass
```

Example:

```console
NAME                 PROVISIONER                             VOLUMEBINDINGMODE   AGE
rv-thin-r1 (default) replicated.csi.storage.deckhouse.io    Immediate           48d
rv-thin-r2           replicated.csi.storage.deckhouse.io    Immediate           48d
```

Select the class name that fits your storage requirements for VM disks.

### Create a VirtualDisk for upload

Create the disk resource, specifying StorageClass and volume size. `spec.persistentVolumeClaim.size` must be at least the actual size of the `qcow2` you upload (if unsure, leave margin—you can recreate the resource with a larger PVC).

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

The object should move to `WaitForUserUpload`, meaning the volume is allocated and you can start transferring the file.

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

Treat the URL as sensitive—it embeds a secret path segment.

### Upload the image

Send the `qcow2` image with an HTTP `PUT` request to the chosen URL. Replace the host and path using the values from the `VirtualDisk` status.

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

If the phase stays on `WaitForUserUpload` for a long time or the resource enters `Failed`, check messages with `kubectl describe vd uploaded-disk` and events in the namespace.

Continue when the disk reaches `Ready`.

## Create the virtual machine

The final step is to describe the VM you want to run: CPU and memory, networks, and which disk is bootable. VMware configuration (`OVF`/`VMX`) is not imported automatically; parameters are carried over manually using the mapping table below and the YAML example.

### VMware vs DVP terminology

If you know vSphere, the following table maps familiar VMware objects to Kubernetes and DVP virtualization resources.

| VMware                  | DVP                                          | Description                      |
|-------------------------|----------------------------------------------|----------------------------------|
| Datastore               | StorageClass                                 | Disk backing storage             |
| VM hardware version     | VirtualMachineClass                          | VM class (CPU, memory, policies) |
| VMX                     | VirtualMachine.spec                          | VM specification                 |
| Virtual disk (VMDK)     | VirtualDisk                                  | VM disk                          |
| ISO image               | VirtualImage (`cdrom: true`)                 | Installation or driver ISO       |
| Template                | VirtualImage                                 | Template for provisioning disks  |
| Port group / VLAN       | VirtualMachine (`networks`)                  | Networking                       |
| Resource pool           | Project and quotas                           | Resource limits per project      |
| Snapshot                | VirtualDiskSnapshot / VirtualMachineSnapshot | Disk and VM snapshots            |
| Folder                  | Namespace                                    | Namespace                        |
| Cluster / resource pool | Project                                      | Namespace grouping               |
| ESXi host               | Node                                         | Physical server                  |
| vCenter                 | Kubernetes API                               | Cluster management               |

More networking detail: [Virtual machine networks](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html).

### VirtualMachine example

The `VirtualMachine` references the uploaded disk through `blockDeviceRefs`. Order in `blockDeviceRefs` determines boot order: the boot disk must be listed first.

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

If additional networks and the SDN module are configured in the cluster, you can add interfaces alongside the primary network:

```yaml
  networks:
    - type: Main
    - type: Network
      name: user-net
```

Additional capabilities (cloud-init, multiple disks, VM classes for production environments) are described in [Virtual machines](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

### Common spec fields

The following table lists fields you most often need to verify after migrating from VMware.

| Field                         | Description                                               |
|-------------------------------|-----------------------------------------------------------|
| `virtualMachineClassName`     | VM class, e.g. `generic`, `serverful`, `high-performance` |
| `osType`                      | `Generic` (Linux and others) or `Windows`                 |
| `cpu.cores`                   | Number of vCPUs                                           |
| `memory.size`                 | RAM                                                       |
| `blockDeviceRefs`             | Disks and images; list order is boot order              |
| `provisioning.type: UserData` | cloud-init user data for first boot                       |

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

If the phase is `Pending` or startup fails, use `d8 k describe vm my-vm` and review virtualization component logs in the cluster.

### Connect to the VM

Choose an access method depending on whether the guest OS accepts SSH, you need a graphical console, or serial console is enough for boot troubleshooting.

| Method         | Purpose                                | Command                            |
|----------------|----------------------------------------|------------------------------------|
| Serial console | Bootloader and kernel output           | `d8 v console my-vm`               |
| VNC            | Graphical console without SSH          | `d8 v vnc my-vm`                   |
| SSH            | Remote shell access to the guest       | `d8 v ssh cloud@my-vm --local-ssh` |

The SSH username is defined in the guest OS; examples in the DVP documentation often create a `cloud` user via cloud-init.
