---
title: "Images"
permalink: en/virtualization-platform/documentation/user/resource-management/images.html
---

The [VirtualImage](/modules/virtualization/cr.html#virtualimage.html) resource is designed for uploading virtual machine images and subsequently using them to create virtual machine disks.

{% alert level="warning" %}
Please note that [VirtualImage](/modules/virtualization/cr.html#virtualimage) is a project resource, which means it is only available within the project or namespace where it was created. To use images at the cluster level, a separate resource is provided — [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).
{% endalert %}

When connected to a virtual machine, the image is accessed in read-only mode.

The image creation process includes the following steps:

- The user creates a `VirtualImage` resource.
- After creation, the image is automatically downloaded from the source specified in the specification to DVCR or PVC storage, depending on the type.
- Once the download is complete, the resource becomes available for disk creation.

There are different types of images:

- **ISO image**: an installation image used for the initial installation of an operating system. Such images are released by OS vendors and are used for installation on physical and virtual servers.
- **Preinstalled disk image**: contains an already installed and configured operating system ready for use after the virtual machine is created. Ready images can be obtained from the distribution developers' resources or created by yourself.

Examples of resources for obtaining virtual machine images:

<a id="image-resources-table"></a>

| Distribution                                                   | Default user. |
|----------------------------------------------------------------|---------------|
| [AlmaLinux](https://almalinux.org/get-almalinux/#Cloud_Images) | `almalinux`   |
| [AlpineLinux](https://alpinelinux.org/cloud/)                  | `alpine`      |
| [CentOS](https://cloud.centos.org/centos/)                     | `cloud-user`  |
| [Debian](https://cdimage.debian.org/images/cloud/)             | `debian`      |
| [Rocky](https://rockylinux.org/download/)                      | `rocky`       |
| [Ubuntu](https://cloud-images.ubuntu.com/)                     | `ubuntu`      |

The following preinstalled image formats are supported:

- qcow2
- raw
- vmdk
- vdi

Image files can also be compressed with one of the following compression algorithms: gz, xz.

Once a share is created, the image type and size are automatically determined. This information is reflected in the resource status.

Images can be downloaded from various sources, such as HTTP servers where image files are located or container registries. It is also possible to download images directly from the command line using the curl utility.

Images can be created from other images and virtual machine disks.

Project image two storage types are supported:

- `ContainerRegistry`: The default type in which the image is stored in `DVCR`.
- `PersistentVolumeClaim`: The type that uses `PVC` as the storage for the image. This option is preferred if you are using storage that supports `PVC` fast cloning, which allows you to create disks from images faster.

{% alert level="warning" %}
Using an image with the `storage: PersistentVolumeClaim` parameter is only supported for creating disks in the same storage class (StorageClass).
{% endalert %}

A full description of the `VirtualImage` resource configuration settings can be found at [link](/modules/virtualization/cr.html#virtualimage.html).

## Creating image from HTTP server

Consider creating an image with the option of storing it in DVCR. Execute the following command to create a `VirtualImage`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22-04
spec:
  # Save the image to DVCR
  storage: ContainerRegistry
  # The source for the image.
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
EOF
```

Check the result of the `VirtualImage` creation:

```bash
d8 k get virtualimage ubuntu-22-04
# or a shorter version
d8 k get vi ubuntu-22-04
```

Example output:

```console
NAME           PHASE   CDROM   PROGRESS   AGE
ubuntu-22-04   Ready   false   100%       23h
```

After creation the `VirtualImage` resource can be in the following states (phases):

- `Pending`: Waiting for all dependent resources required for image creation to be ready.
- `WaitForUserUpload`: Waiting for the user to upload the image (the phase is present only for `type=Upload`).
- `Provisioning`: The image creation process is in progress.
- `Ready`: The image is created and ready for use.
- `Failed`: An error occurred during the image creation process.
- `Terminating`: The image is being deleted. The image may "hang" in this state if it is still connected to the virtual machine.

As long as the image has not entered the `Ready` phase, the contents of the `.spec` block can be changed. If you change it, the disk creation process will start again. After entering the `Ready` phase, the contents of the `.spec` block cannot be changed.

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block

You can trace the image creation process by adding the `-w` key to the previous command:

```bash
d8 k get vi ubuntu-22-04 -w
```

Example output:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-22-04   Provisioning   false              4s
ubuntu-22-04   Provisioning   false   0.0%       4s
ubuntu-22-04   Provisioning   false   28.2%      6s
ubuntu-22-04   Provisioning   false   66.5%      8s
ubuntu-22-04   Provisioning   false   100.0%     10s
ubuntu-22-04   Provisioning   false   100.0%     16s
ubuntu-22-04   Ready          false   100%       18s
```

The `VirtualImage` resource description provides additional information about the downloaded image:

```bash
d8 k describe vi ubuntu-22-04
```

How to create an image from an HTTP server in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "Disk Images" section.
- Click "Create Image".
- Select "Load data from link (HTTP)" from the list.
- In the form that opens, enter the image name in the "Image name" field.
- Select `ContainerRegistry` in the "Storage" field.
- Specify the link to the image in the "URL" field.
- Click the "Create" button.
- The image status is displayed at the top left, under the image name.

Now let's look at an example of creating an image and storing it in PVC:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22-04-pvc
spec:
  storage: PersistentVolumeClaim
  persistentVolumeClaim:
    # Substitute your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
  # Source for image creation.
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
EOF
```

Check the result of the `VirtualImage` creation:

```bash
d8 k get vi ubuntu-22-04-pvc
```

Example output:

```console
NAME              PHASE   CDROM   PROGRESS   AGE
ubuntu-22-04-pvc  Ready   false   100%       23h
```

If the `.spec.persistentVolumeClaim.storageClassName` parameter is not specified, the default `StorageClass` at the cluster level will be used, or for images if specified in module settings.

How to create an image and store it in PVC in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "Disk Images" section.
- Click "Create Image".
- Select "Load data from link (HTTP)" from the list.
- In the form that opens, enter the image name in the "Image name" field.
- In the "Storage" field, select `PersistentVolumeClaim`.
- In the "Storage class" field, you can select StorageClass or leave the default selection.
- In the URL field, specify the link to the image.
- Click the Create button.
- The image status is displayed at the top left, under the image name.

## Creating an image from container registry

An image stored in container registry has a certain format. Let's look at an example:

First, download the image locally:

```bash
curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
```

Next, create a `Dockerfile` with the following contents:

```Dockerfile
FROM scratch
COPY ubuntu2204.img /disk/ubuntu2204.img
```

Build the image and load it into the container registry. The example below uses docker.io as the container registry. you need to have a service account and a customized environment to run it.

```bash
docker build -t docker.io/<username>/ubuntu2204:latest
```

where `username` is the username specified when registering with docker.io.

Load the created image into the container registry:

```bash
docker push docker.io/<username>/ubuntu2204:latest
```

To use this image, create a resource as an example:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-2204
spec:
  storage: ContainerRegistry
  dataSource:
    type: ContainerImage
    containerImage:
      image: docker.io/<username>/ubuntu2204:latest
EOF
```

How to create an image from Container Registry in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "Disk Images" section.
- Click "Create Image".
- Select "Upload data from container image" from the list.
- In the form that opens, enter the image name in the "Image Name" field.
- In the "Storage" field, select `ContainerRegistry`.
- In the "Image in Container Registry" field, specify `docker.io/<username>/ubuntu2204:latest`.
- Click the "Create" button.
- The image status is displayed at the top left, under the image name.

## Load an image from the command line

To load an image from the command line, first create the following resource as shown below with the `VirtualImage` example:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: some-image
spec:
  storage: ContainerRegistry
  dataSource:
    type: Upload
EOF
```

Once created, the resource will enter the `WaitForUserUpload` phase, which means it is ready for image upload.

There are two options available for uploading from a cluster node and from an arbitrary node outside the cluster:

```bash
d8 k get vi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq
```

Example output:

```json
{
  "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
  "inCluster":"http://10.222.165.239/upload"
}
```

As an example, download the Cirros image:

```bash
curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
```

Upload the image using the following command:

```bash
curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
```

After the upload is complete, the image should be created and enter the `Ready` phase

```bash
d8 k get vi some-image
```

Example output:

```console
NAME         PHASE   CDROM   PROGRESS   AGE
some-image   Ready   false   100%       1m
```

How to upload an image from the command line in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" -> "Disk Images" section.
- Click "Create Image" then select "Upload from computer" from the drop-down menu.
- Enter the image name in the "Image Name" field.
- In the "Upload File" field, click the "Choose a file from your computer" link.
- Select the file in the file manager that opens.
- Click the "Create" button.
- Wait until the image changes to `Ready` status.

## Creating an image from a disk

It is possible to create an image from [disk](/products/virtualization-platform/documentation/user/resource-management/disks.html). To do so, one of the following conditions must be met:

- The disk is not attached to any virtual machine.
- The virtual machine to which the disk is attached is in a powered off state.

Example of creating an image from a disk:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: linux-vm-root
spec:
  storage: ContainerRegistry
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDisk
      name: linux-vm-root
EOF
```

How to create an image from a disk in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "Disk Images" section.
- Click "Create Image".
- Select "Write data from disk" from the list.
- In the form that opens, enter `linux-vm-root` in the "Image Name" field.
- In the "Storage" field, select `ContainerRegistry`.
- In the "Disk" field, select the desired disk from the drop-down list.
- Click the "Create" button.
- The image status is displayed at the top left, under its name.

## Creating an image from a disk snapshot

It is possible to create an image from [snapshot](/products/virtualization-platform/documentation/user/resource-management/snapshots.html). This requires that the disk snapshot is in the ready phase.

Example of creating an image from a disk snapshot:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: linux-vm-root
spec:
  storage: ContainerRegistry
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-snapshot
EOF
```
