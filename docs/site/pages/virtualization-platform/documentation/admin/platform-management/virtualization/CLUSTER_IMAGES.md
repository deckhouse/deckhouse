---
title: "Cluster images"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/cluster-images.html
---

## Images

The [`ClusterVirtualImage`](../../../../reference/cr/clustervirtualimage.html) resource is used to upload virtual machine images to the in-cluster storage, enabling the creation of disks for virtual machines. This resource is available in any namespace or project within the cluster.

The process of creating an image involves the following steps:

1. The user creates a [`ClusterVirtualImage`](../../../reference/cr/clustervirtualimage.html) resource.
1. After creation, the image is automatically downloaded from the source specified in the specification to the storage (DVCR).
1. Once the download is complete, the resource becomes available for disk creation.

There are different types of images:

- ISO Image — an installation image used for the initial installation of an operating system. These images are released by OS vendors and are used for installing on physical or virtual servers.
- Pre-installed System Disk Image — contains an already installed and configured operating system, ready for use after creating a virtual machine. These images are offered by several vendors and can be available in formats such as qcow2, raw, vmdk, and others.

Examples of resources for obtaining pre-installed virtual machine disk images:

- [Ubuntu](https://cloud-images.ubuntu.com).
- [Alt Linux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/platform/images/cloud/x86_64).
- [Astra Linux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit).

After creating the resource, the type and size of the image are automatically determined and reflected in the resource's status.

Images can be downloaded from various sources, such as HTTP servers hosting image files or container registries. Additionally, there is an option to upload images directly from the command line using the `curl` utility.

Images can also be created based on other images or virtual machine disks.

For a complete description of the configuration parameters for the `ClusterVirtualImage` resource, refer to [the documentation](../../../../reference/cr/clustervirtualimage.html).

### Creating an image from an HTTP server

Let's explore how to create a cluster image.

Run the following command to create a `ClusterVirtualImage`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: ClusterVirtualImage
metadata:
  name: ubuntu-22.04
spec:
  # Source for creating the image.
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
EOF
```

Check the result of creating the `ClusterVirtualImage` with the following command:

```shell
d8 k get clustervirtualimage ubuntu-22.04

# A shorter version of the command
d8 k get cvi ubuntu-22.04
```

In the output, you should see information about the `ClusterVirtualImage` resource:

```console
NAME           PHASE   CDROM   PROGRESS   AGE
ubuntu-22.04   Ready   false   100%       23h
```

After creation, the `ClusterVirtualImage` resource may have the following states (phases):

- `Pending` — waiting for all dependent resources required for image creation to become ready.
- `WaitForUserUpload` — waiting for the user to upload the image (this phase exists only for `type=Upload`).
- `Provisioning` — the image creation process is in progress.
- `Ready` —  the image has been created and is ready for use.
- `Failed` — an error occurred during the image creation process.
- `Terminating` — the image is being deleted. The image may "hang" in this state if it is still attached to a virtual machine.

Until the image transitions to the `Ready` phase, the contents of the `.spec` block can be modified. If any changes are made, the image creation process will be reinitiated.

Once the image reaches the `Ready` phase, modifications to the `.spec` block are not allowed, as the image is considered fully created and ready for use. Making changes to this block after the image has reached the `Ready` state may compromise its integrity or affect its proper usage.

You can track the image creation process by adding the `-w` flag to the previous command:

```shell
d8 k get cvi ubuntu-22.04 -w
```

In the output, you should see information about the image creation progress:

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

Additional information about the downloaded image can be retrieved by describing the `ClusterVirtualImage` resource:

```shell
d8 k describe cvi ubuntu-22.04
```

### Creating an image from a Container Registry

An image stored in a container registry has a specific format. Let’s consider an example:

Download the image locally:

```shell
curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
```

Create a `Dockerfile` with the following content:

```shell
FROM scratch
COPY ubuntu2204.img /disk/ubuntu2204.img
```

Build the image and push it to a container registry. In this example, [docker.io](https://www.docker.com/) is used. To perform these steps, you need an account on the service and a properly configured environment:

```shell
docker build -t docker.io/<username>/ubuntu2204:latest
```

Where `username` is the username you specified during registration on docker.io.

Push the created image to the container registry:

```shell
docker push docker.io/<username>/ubuntu2204:latest
```

To use this image, create a resource as an example:

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

### Uploading an image from the command line

To upload an image from the command line, first create the following resource as shown in the example `ClusterVirtualImage`:

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

There are two options for uploading — from a cluster node or from any external node outside the cluster:

```shell
d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq

# {
#   "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
#   "inCluster":"http://10.222.165.239/upload"
# }
```

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
