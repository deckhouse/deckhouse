---
title: "Cluster images"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/cluster-images.html
---

## Images

The [`ClusterVirtualImage`](/modules/virtualization/cr.html#clustervirtualimage) resource is used to upload virtual machine images to the in-cluster storage, enabling the creation of disks for virtual machines. This resource is available in any namespace or project within the cluster.

The process of creating an image involves the following steps:

1. The user creates a [`ClusterVirtualImage`](/modules/virtualization/cr.html#clustervirtualimage) resource.
1. Once created, the image is automatically uploaded from the source specified in the specification to the storage (DVCR).
1. Once the upload is complete, the resource becomes available for disk creation.

There are different types of images:

- **ISO image**: An installation image used for the initial installation of an operating system (OS). Such images are released by OS vendors and are used for installation on physical and virtual servers.
- **Preinstalled disk image**: contains an already installed and configured operating system ready for use after the virtual machine is created. You can obtain pre-configured images from the distribution developers' resources or create them manually.

Examples of resources for obtaining pre-installed virtual machine disk images:

- Ubuntu
  - [24.04 LTS (Noble Numbat)](https://cloud-images.ubuntu.com/noble/current/)
  - [22.04 LTS (Jammy Jellyfish)](https://cloud-images.ubuntu.com/jammy/current/)
  - [20.04 LTS (Focal Fossa)](https://cloud-images.ubuntu.com/focal/current/)
  - [Minimal images](https://cloud-images.ubuntu.com/minimal/releases/)
- Debian
  - [12 bookworm](https://cdimage.debian.org/images/cloud/bookworm/latest/)
  - [11 bullseye](https://cdimage.debian.org/images/cloud/bullseye/latest/)
- AlmaLinux
  - [9](https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/)
  - [8](https://repo.almalinux.org/almalinux/8/cloud/x86_64/images/)
- RockyLinux
  - [9.5](https://dl.rockylinux.org/vault/rocky/9.5/images/x86_64/)
  - [8.10](https://download.rockylinux.org/pub/rocky/8.10/images/x86_64/)
- CentOS
  - [10 Stream](https://cloud.centos.org/centos/10-stream/x86_64/images/)
  - [9 Stream](https://cloud.centos.org/centos/9-stream/x86_64/images/)
  - [8 Stream](https://cloud.centos.org/centos/8-stream/x86_64/)
  - [8](https://cloud.centos.org/centos/8/x86_64/images/)

The following preinstalled image formats are supported:

- `qcow2`
- `raw`
- `vmdk`
- `vdi`

Image files can also be compressed with one of the following compression algorithms: `gz`, `xz`.

After creating the resource, the type and size of the image are automatically determined and reflected in the resource's status.

Images can be downloaded from various sources, such as HTTP servers hosting image files or container registries. Additionally, there is an option to upload images directly from the command line using the `curl` utility.

Images can also be created based on other images or virtual machine disks.

For a complete description of the configuration parameters for the `ClusterVirtualImage` resource, refer to [the documentation](/modules/virtualization/cr.html#clustervirtualimage).

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

### Creating an image from an HTTP server

Let's explore how to create a cluster image.

1. Run the following command to create a `ClusterVirtualImage`:

    ```yaml
    d8 k apply -f - <<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: ubuntu-22-04
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
    d8 k get clustervirtualimage ubuntu-22-04
    ```

    A shorter version of the command:

   ```shell
    d8 k get cvi ubuntu-22-04
    ```

    In the output, you should see information about the `ClusterVirtualImage` resource:

    ```console
    NAME           PHASE   CDROM   PROGRESS   AGE
    ubuntu-22-04   Ready   false   100%       23h
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

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block.

You can trace the image creation process by adding the `-w` key to the command used for verification of the created resource:

```shell
d8 k get cvi ubuntu-22-04 -w
```

In the output, you should see information about the image creation progress:

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

Additional information about the downloaded image can be retrieved by describing the `ClusterVirtualImage` resource:

```shell
d8 k describe cvi ubuntu-22-04
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
