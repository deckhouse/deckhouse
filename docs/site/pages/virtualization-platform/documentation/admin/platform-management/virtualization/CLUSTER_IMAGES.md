---
title: "Cluster images"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/cluster-images.html
---

## Images

The [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource is designed for uploading virtual machine images and subsequently using them to create virtual machine disks.

{% alert level="warning" %}
Please note that [VirtualImage](/modules/virtualization/cr.html#virtualimage) is a project resource, which means it is only available within the project or namespace where it was created. To use images at the cluster level, a separate resource is provided — [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).
{% endalert %}

When connected to a virtual machine, the image is accessed in read-only mode.

The image creation process includes the following steps:

- The user creates a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource.
- After creation, the image is automatically downloaded from the source specified in the specification to DVCR or PVC storage, depending on the type.
- Once the download is complete, the resource becomes available for disk creation.

There are different types of images:

- **ISO image**: an installation image used for the initial installation of an operating system. Such images are released by OS vendors and are used for installation on physical and virtual servers.
- **Preinstalled disk image**: contains an already installed and configured operating system ready for use after the virtual machine is created. Ready images can be obtained from the distribution developers' resources or created by yourself.

Examples of resources for obtaining virtual machine images:

<a id="image-resources-table"></a>

| Distribution                                                                      | Default user.             |
| --------------------------------------------------------------------------------- | ------------------------- |
| [AlmaLinux](https://almalinux.org/get-almalinux/#Cloud_Images)                    | `almalinux`               |
| [AlpineLinux](https://alpinelinux.org/cloud/)                                     | `alpine`                  |
| [CentOS](https://cloud.centos.org/centos/)                                        | `cloud-user`              |
| [Debian](https://cdimage.debian.org/images/cloud/)                                | `debian`                  |
| [Rocky](https://rockylinux.org/download/)                                         | `rocky`                   |
| [Ubuntu](https://cloud-images.ubuntu.com/)                                        | `ubuntu`                  |

The following preinstalled image formats are supported:

- qcow2
- raw
- vmdk
- vdi

Image files can also be compressed with one of the following compression algorithms: gz, xz.

Once a share is created, the image type and size are automatically determined, and this information is reflected in the share status.

The image status shows two sizes:

- `STOREDSIZE` (storage size): The amount of space the image actually occupies in storage (DVCR or PVC). For images uploaded in a compressed format (for example, `.gz` or `.xz`), this value is smaller than the unpacked size.
- `UNPACKEDSIZE` (unpacked size): The image size after unpacking. It is used when creating a disk from the image and defines the minimum disk size that can be created.

{% alert level="info" %}
When creating a disk from an image, set the disk size to `UNPACKEDSIZE` or larger .  
If the size is not specified, the disk will be created with a size equal to `UNPACKEDSIZE`.
{% endalert %}

Images can be downloaded from various sources, such as HTTP servers where image files are located or container registries. It is also possible to download images directly from the command line using the curl utility.

Images can be created from other images and virtual machine disks.

Project image two storage options are supported:

- `ContainerRegistry`: The default type in which the image is stored in `DVCR`.
- `PersistentVolumeClaim`: The type that uses `PVC` as the storage for the image. This option is preferred if you are using storage that supports `PVC` fast cloning, which allows you to create disks from images faster.

{% alert level="warning" %}
Using an image with the `storage: PersistentVolumeClaim` parameter is only supported for creating disks in the same storage class (StorageClass).
{% endalert %}

## Increasing the size of DVCR

To increase the disk size for DVCR, you need to set a larger size in the virtualization module configuration than the current size.

1. Check the current DVCR size:

    ```shell
    d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
    ```

    Example output:

    ```text
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
    ```

1. Set the new size:

    ```shell
    d8 k patch mc virtualization \
      --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
    ```

   Example output:

    ```text
   moduleconfig.deckhouse.io/virtualization patched
    ```

1. Verify the size change:

    ```shell
    d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
    ```

   Example output:

    ```text
    {"size":"59G","storageClass":"linstor-thick-data-r1"}
   ```

1. Check the current status of the DVCR:

    ```shell
    d8 k get pvc dvcr -n d8-virtualization
    ```

   Example output:

    ```console
    NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
    dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
    ```

## Creating a golden image for Linux

A golden image is a pre-configured virtual machine image that can be used to quickly create new VMs with pre-installed software and settings.

1. Create a virtual machine, install the required software on it, and perform all necessary configurations.

1. Install and configure qemu-guest-agent (recommended):

   - For RHEL/CentOS:

   ```bash
   yum install -y qemu-guest-agent
   ```

   - For Debian/Ubuntu:

   ```bash
   apt-get update
   apt-get install -y qemu-guest-agent
   ```

1. Enable and start the service:

   ```bash
   systemctl enable qemu-guest-agent
   systemctl start qemu-guest-agent
   ```

1. Set the VM run policy to [`runPolicy: AlwaysOnUnlessStoppedManually`](/modules/virtualization/stable/cr.html#virtualmachine-v1alpha2-spec-runpolicy). This is required to be able to shut down the VM.

1. Prepare the image. Clean unused filesystem blocks:

   ```bash
   fstrim -v /
   fstrim -v /boot
   ```

1. Clean network settings:

   - For RHEL:

   ```bash
   nmcli con delete $(nmcli -t -f NAME,DEVICE con show | grep -v ^lo: | cut -d: -f1)
   rm -f /etc/sysconfig/network-scripts/ifcfg-eth*
   ```

   - For Debian/Ubuntu:

   ```bash
   rm -f /etc/network/interfaces.d/*
   ```

1. Clean system identifiers:

   ```bash
   echo -n > /etc/machine-id
   rm -f /var/lib/dbus/machine-id
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Remove SSH host keys:

   ```bash
   rm -f /etc/ssh/ssh_host_*
   ```

1. Clean systemd journal:

   ```bash
   journalctl --vacuum-size=100M --vacuum-time=7d
   ```

1. Clean package manager cache:

   - For RHEL:

   ```bash
   yum clean all
   ```

   - For Debian/Ubuntu:

   ```bash
   apt-get clean
   ```

1. Clean temporary files:

   ```bash
   rm -rf /tmp/*
   rm -rf /var/tmp/*
   ```

1. Clean logs:

   ```bash
   find /var/log -name "*.log" -type f -exec truncate -s 0 {} \;
   ```

1. Clean command history:

   ```bash
   history -c
   ```

   For RHEL: reset and restore SELinux contexts (choose one of the following):

   - Option 1: Check and restore contexts immediately:

     ```bash
     restorecon -R /
     ```

   - Option 2: Schedule relabel on next boot:

     ```bash
     touch /.autorelabel
     ```

1. Verify that `/etc/fstab` uses UUID or LABEL instead of device names (e.g., `/dev/sdX`). To check, run:

   ```bash
   blkid
   cat /etc/fstab
   ```

1. Clean cloud-init state, logs, and seed (recommended method):

   ```bash
   cloud-init clean --logs --seed
   ```

1. Perform final synchronization and buffer cleanup:

   ```bash
   sync
   echo 3 > /proc/sys/vm/drop_caches
   ```

1. Shut down the virtual machine:

   ```bash
   poweroff
   ```

1. Create a `VirtualImage` resource from the prepared VM disk:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: <image-name>
     namespace: <namespace>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <source-disk-name>
   EOF
   ```

   Alternatively, create a `ClusterVirtualImage` to make the image available at the cluster level for all projects:

    ```bash
    d8 k apply -f -<<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: <image-name>
    spec:
      dataSource:
        type: ObjectRef
        objectRef:
          kind: VirtualDisk
          name: <source-disk-name>
          namespace: <namespace>
    EOF
    ```

1. Create a VM disk from the created image:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: <vm-disk-name>
     namespace: <namespace>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: <image-name>
   EOF
   ```

After completing these steps, you will have a golden image that can be used to quickly create new virtual machines with pre-installed software and configurations.

### Creating an image from an HTTP server

Let's explore how to create a cluster image.

1. Run the following command to create a `ClusterVirtualImage`:

    ```yaml
    d8 k apply -f - <<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: ubuntu-24-04
    spec:
      # Source for creating the image.
      dataSource:
        type: HTTP
        http:
          url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
    EOF
    ```

1. Check the result of creating the `ClusterVirtualImage` with the following command:

    ```shell
    d8 k get clustervirtualimage ubuntu-24-04
    ```

    A shorter version of the command:

   ```shell
    d8 k get cvi ubuntu-24-04
    ```

    In the output, you should see information about the `ClusterVirtualImage` resource:

    ```console
    NAME           PHASE   CDROM   PROGRESS   AGE
    ubuntu-24-04   Ready   false   100%       23h
    ```

After creation, the `ClusterVirtualImage` resource may have the following states (phases):

- `Pending`: Waiting for all dependent resources required for image creation to become ready.
- `WaitForUserUpload`: Waiting for the user to upload the image (this phase exists only for `type=Upload`).
- `Provisioning`: The image creation process is in progress.
- `Ready`: The image has been created and is ready for use.
- `Failed`: An error occurred during the image creation process.
- `Terminating`: The image is being deleted. The image may "hang" in this state if it is still attached to a virtual machine.
- `ImageLost`: The image is missing in DVCR. The resource cannot be used.
- `PVCLost`: The child PVC of the resource is missing. The resource cannot be used.

Until the image transitions to the `Ready` phase, the contents of the `.spec` block can be modified. If any changes are made, the image creation process will be reinitiated.

Once the image reaches the `Ready` phase, modifications to the `.spec` block are not allowed, as the image is considered fully created and ready for use. Making changes to this block after the image has reached the `Ready` state may compromise its integrity or affect its proper usage.

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block.

You can trace the image creation process by adding the `-w` key to the command used for verification of the created resource:

```shell
d8 k get cvi ubuntu-24-04 -w
```

In the output, you should see information about the image creation progress:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-24-04   Provisioning   false              4s
ubuntu-24-04   Provisioning   false   0.0%       4s
ubuntu-24-04   Provisioning   false   28.2%      6s
ubuntu-24-04   Provisioning   false   66.5%      8s
ubuntu-24-04   Provisioning   false   100.0%     10s
ubuntu-24-04   Provisioning   false   100.0%     16s
ubuntu-24-04   Ready          false   100%       18s
```

Additional information about the downloaded image can be retrieved by describing the `ClusterVirtualImage` resource:

```shell
d8 k describe cvi ubuntu-24-04
```

How to create an image from an HTTP server in the web interface:

- Go to the "System" tab, then to the "Virtualization" → "Cluster Images" section.
- Click "Create Image", then select "Load data from link (HTTP)" from the drop-down menu.
- Enter the image name in the "Image Name" field.
- Specify the link to the image in the "URL" field.
- Click "Create".
- Wait until the image status changes to `Ready`.

### Creating an image from a container registry

An image stored in a container registry has a specific format. Let’s consider an example of this format:

1. Download the image locally:

    ```shell
    curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
    ```

1. Create a `Dockerfile` with the following content:

    ```shell
    FROM scratch
    COPY ubuntu2204.img /disk/ubuntu2204.img
    ```

1. Build the image and push it to a container registry. In this example, [docker.io](https://www.docker.com/) is used. To perform these steps, you need an account on the service and a properly configured environment:

    ```shell
    docker build -t docker.io/<username>/ubuntu2204:latest
    ```

    where `username` is the username you specified during registration on docker.io.

1. Push the created image to the container registry:

    ```shell
    docker push docker.io/<username>/ubuntu2204:latest
    ```

1. To use this image, create the following resource:

    ```yaml
    d8 k apply -f - <<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: ubuntu-2204
    spec:
      dataSource:
        type: ContainerImage
        containerImage:
          image: docker.io/<username>/ubuntu2204:latest
    EOF
    ```

How to create an image from the container registry in the web interface:

- Go to the "System" tab, then to the "Virtualization" → "Cluster Images" section.
- Click "Create Image", then select "Load data from container image" from the drop-down list.
- Enter the image name in the "Image Name" field.
- Specify the link to the image in the "Image in Container Registry" field.
- Click "Create".
- Wait until the image changes to the `Ready` status.

### Uploading an image from the command line

To upload an image from the command line, first create the resource as shown in the example `ClusterVirtualImage`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: ClusterVirtualImage
metadata:
  name: some-image
spec:
  dataSource:
    type: Upload
EOF
```

After creating this resource, it will transition to the `WaitForUserUpload` phase, indicating that it is ready for image upload.

There are two options available for uploading: from a cluster node and from an arbitrary node outside the cluster:

```shell
d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq
```

Example output:

```text
{
  "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
  "inCluster":"http://10.222.165.239/upload"
}
```

Where:

- `inCluster`: A URL used to download the image from one of the cluster nodes.
- `external`: A URL used in all other cases.

As an example, download the Cirros image:

```shell
curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
```

Then upload the image using the following command:

```shell
curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
```

After the upload is complete, the image should be created and transition to the `Ready` phase. To check the image's phase, run the command:

```shell
d8 k get cvi some-image
```

In the output, you should see information about the image's phase:

```console
NAME         PHASE   CDROM   PROGRESS   AGE
some-image   Ready   false   100%       1m
```

How to perform the operation in the web interface:

- Go to the "System" tab, then to the "Virtualization" → "Cluster Images" section.
- Click "Create Image", then select "Upload from Computer" from the drop-down menu.
- Enter the image name in the "Image Name" field.
- In the "Upload File" field, click the "Select a file on your computer" link.
- Select the file in the file manager that opens.
- Click the "Create" button.
- Wait until the image changes to `Ready` status.

### Cleaning up image storage

{% alert level="info" %}
Available in [version 1.2.0](/products/virtualization-platform/documentation/release-notes.html#v120) and later.
{% endalert %}

Over time, the creation and deletion of ClusterVirtualImage, VirtualImage, and VirtualDisk resources leads to the accumulation
of outdated images in the intra-cluster storage. Scheduled garbage collection is implemented to keep the storage up to
date, but this feature is disabled by default.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  # ...
  settings:
    dvcr:
      gc:
        schedule: "0 20 * * *"
  # ...
```

While garbage collection is running, the storage is switched to read-only mode, and all resources created during this time will wait for the cleanup to finish.

To check for outdated images in the storage, you can run the following command:

```bash
d8 k -n d8-virtualization exec deploy/dvcr -- dvcr-cleaner gc check
```

It prints information about the storage status and a list of outdated images that can be deleted.

```console
Found 2 cvi, 5 vi, 1 vd manifests in registry
Found 1 cvi, 5 vi, 11 vd resources in cluster
  Total     Used    Avail     Use%
36.3GiB  13.1GiB  22.4GiB      39%
Images eligible for cleanup:
KIND                   NAMESPACE            NAME
ClusterVirtualImage                         debian-12
VirtualDisk            default              debian-10-root
VirtualImage           default              ubuntu-2204
```
