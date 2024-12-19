---
title: "Images"
permalink: en/virtualization-platform/documentation/user/resource-management/images.html
---

The [VirtualImage](../../../reference/cr/virtualimage.html) resource is designed for uploading virtual machine images and subsequently using them to create virtual machine disks. This resource is only accessible within the namespace or project where it was created.

The image creation process involves the following steps:

1. The user creates a [VirtualImage](../../../reference/cr/virtualimage.html) resource.
1. Once created, the image is automatically downloaded from the specified source in the specification to the storage (DVCR).
1. After the download is complete, the resource becomes available for disk creation.

There are different types of images:

- ISO Image: An image used for the initial installation of an operating system. Such images are released by OS vendors and are used for installation on physical and virtual servers.
- Preinstalled System Disk Image: Contains configured operating system, ready for use after creating a virtual machine. These images are offered by various vendors and can come in formats such as qcow2, raw, vmdk, and others.

Examples of resources for obtaining virtual machine images:

- [Ubuntu](https://cloud-images.ubuntu.com).
- [Alt Linux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/platform/images/cloud/x86_64).
- [Astra Linux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit).

After creating the resource, the type and size of the image are automatically determined, and this information is reflected in the resource's status.

Images can be uploaded from various sources, such as HTTP servers where the image files are hosted, or container registries. Additionally, images can be uploaded directly from the command line using the `curl` utility.

Images can also be created from other images and virtual machine disks.

For project-specific images, two storage options are supported:

- Container registry: The default type where the image is stored in the DVCR.
- Persistent Volume Claim: This type uses PVC as the storage for the image. This option is preferable when using storage that supports fast PVC cloning, as disk creation from images will be faster in this case.

### Creating an image from an HTTP server

Here is an example of creating an image with storage in DVCR. Execute the following command to create a [VirtualImage](../../../reference/cr/virtualimage.html):

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22.04
spec:
  # Save the image in DVCR
  storage: ContainerRegistry
  # Source for creating the image.
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
EOF
```

Verifying the creation of [VirtualImage](../../../reference/cr/virtualimage.html):

```bash
d8 k get virtualimage ubuntu-22.04

# A shorter version of the command
d8 k get vi ubuntu-22.04
```

Example output:

```console
NAME           PHASE   CDROM   PROGRESS   AGE
ubuntu-22.04   Ready   false   100%       23h
```

After creation, the [VirtualImage](../../../reference/cr/virtualimage.html) resource can be in the following states:

- `Pending` — Waiting for readiness of all dependent resources required for image creation.
- `WaitForUserUpload` — Waiting for the user to upload the image (this state is present only for `type=Upload`).
- `Provisioning` — The image creation process is ongoing.
- `Ready` — The image is created and ready for use.
- `Failed` — An error occurred during the image creation process.
- `Terminating` — The image deletion process is ongoing; this process may "hang" in this state if the image is still attached to a virtual machine.

Until the image transitions to the `Ready` phase, the entire `.spec` block can be modified. Changing it will restart the image creation process. Once the image reaches the `Ready` phase, the contents of the `.spec` block can no longer be changed.

To monitor the image creation process, add the `-w` flag to the previous command:

```bash
d8 k get vi ubuntu-22.04 -w
```

Example output:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-22.04   Provisioning   false              4s
ubuntu-22.04   Provisioning   false   0.0%       4s
ubuntu-22.04   Provisioning   false   28.2%      6s
ubuntu-22.04   Provisioning   false   66.5%      8s
ubuntu-22.04   Provisioning   false   100.0%     10s
ubuntu-22.04   Provisioning   false   100.0%     16s
ubuntu-22.04   Ready          false   100%       18s
```

You can obtain additional information about the downloaded image in the [VirtualImage](../../../reference/cr/virtualimage.html) resource description:

```bash
d8 k describe vi ubuntu-22.04
```

Now, let's look at an example of creating an image stored in a PVC:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22.04-pvc
spec:
  # Configuration for storing the project image.
  storage: PersistentVolumeClaim
  # Source for creating the image.
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
EOF
```

Check the result of creating the [VirtualImage](../../../reference/cr/virtualimage.html):

```bash
d8 k get vi ubuntu-22.04-pvc
```

Example output:

```console
NAME              PHASE   CDROM   PROGRESS   AGE
ubuntu-22.04-pvc  Ready   false   100%       23h
```

### Creating an image from a container registry

An image stored in a container registry follows a specific format. Here's an example:

First, download the image locally:

```bash
curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
```

Next, create a Dockerfile with the following content:

```Dockerfile
FROM scratch
COPY ubuntu2204.img /disk/ubuntu2204.img
```

Then, build the image and push it to the container registry. In the example below, docker.io is used as the container registry. You will need to have a service account and a configured environment to proceed.

```bash
docker build -t docker.io/<username>/ubuntu2204:latest
```

Where `username` is the username used during registration on docker.io.

Push the created image to the container registry:

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

### Uploading an image from the command line

To upload an image from the command line, first create the following resource, as shown in the example for [VirtualImage](../../../reference/cr/virtualimage.html):

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: some-image
spec:
  # Project image storage settings.
  storage: ContainerRegistry
  # Image source settings.
  dataSource:
    type: Upload
EOF
```

After creation, the resource will transition to the `WaitForUserUpload` phase, indicating that it is ready for image upload.

There are two upload options available: from a cluster node and from an external node outside the cluster:

```bash
d8 k get vi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq

# {
#   "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
#   "inCluster":"http://10.222.165.239/upload"
# }
```

As an example, let's consider uploading the Cirros image:

```bash
curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
```

Upload the image using the following command:

```bash
curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
```

After the upload is complete, the image should be created and transition to the `Ready` phase.

```bash
d8 k get vi some-image
```

Example output:

```console
NAME         PHASE   CDROM   PROGRESS   AGE
some-image   Ready   false   100%       1m
```

### Creating an image from a disk

It is possible to create an image from a [disk](./disks.html). To do this, one of the following conditions must be met:

- The disk should not be attached to any virtual machine.
- The virtual machine that the disk is attached to must be in a powered-off state.

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
