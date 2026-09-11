---
title: "Cluster images of virtual machines"
permalink: en/admin/configuration/virtualization/cluster-images.html
description: "Cluster images of virtual machines: types and formats, creating an image from an HTTP server, from a container registry, and by uploading from the command line."
search: cluster image, ClusterVirtualImage, image formats, image upload
---

An image holds the contents of a disk that project owners use to create virtual machine disks. A cluster image, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), is available in every namespace and project of the cluster, so an image uploaded once serves all projects at once.

An image appears in the cluster in three steps:

1. The administrator creates a [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resource and specifies a data source in it.
1. DP downloads the image from that source to the internal storage (DVCR).
1. The downloaded image becomes available for creating disks.

The image source can be an HTTP server hosting the image file, a container image registry, or a file on your computer that you upload from the command line. You can also create an image from another image, from a virtual machine disk, or from a disk snapshot.

The `PHASE` column in the `d8 k get cvi` output shows the progress of image creation; for its values, see the [`.status.phase`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-status-phase) field. To follow the creation in real time, add the `-w` flag. If an image stays not ready for a long time, check the [`.status.conditions`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-status-conditions) block and the `d8 k describe cvi` output for the reason.

Until an image reaches the `Ready` phase, you can change its `.spec` block, and the download restarts after each change. For a ready image, the `.spec` block can no longer be changed. For all image parameters, see [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).

## Image types and formats

There are two types of images:

- **ISO image**: An installation image used for the initial installation of an operating system (OS). OS vendors publish such images and use them to install the OS on physical and virtual servers.
- **Disk image with a preinstalled system**: Contains an OS that is already installed and configured, and is ready to work as soon as the virtual machine (VM) is created. Distribution vendors publish such images, or you can build them yourself.

Distribution vendors publish ready-made images with a preinstalled system. The following table lists their download pages and the users configured in those images by default:

<a id="image-resources-table"></a>

| Distribution                                                                      | Default user |
| --------------------------------------------------------------------------------- | ------------ |
| [AlmaLinux](https://almalinux.org/get-almalinux/#Cloud_Images)                    | `almalinux`  |
| [AlpineLinux](https://alpinelinux.org/cloud/)                                     | `alpine`     |
| [AltLinux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/)                   | `altlinux`   |
| [AstraLinux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit/) | `astra`      |
| [CentOS](https://cloud.centos.org/centos/)                                        | `cloud-user` |
| [Debian](https://cdimage.debian.org/images/cloud/)                                | `debian`     |
| [Rocky](https://rockylinux.org/download/)                                         | `rocky`      |
| [Ubuntu](https://cloud-images.ubuntu.com/)                                        | `ubuntu`     |

DP accepts image files in the following formats:

- `qcow2`
- `raw`
- `vmdk`
- `vdi`
- `vhd`
- `vhdx`

You can provide an image compressed with `gz`, `xz`, or `zst`. DP unpacks it during the upload.

DP detects the image type and size on its own and records them in the resource status. There are two sizes, and both appear in the `d8 k get cvi -o wide` output:

- `STOREDSIZE`: The space the image occupies in the storage. For an image uploaded in a compressed form, it's smaller than the unpacked size. Use this column to estimate how much space the images take in DVCR.
- `UNPACKEDSIZE`: The size of the image after unpacking. It defines the minimum size of a disk that can be created from this image.

{% alert level="info" %}
When creating a disk from an image, specify a size no smaller than the `UNPACKEDSIZE` value.
If you don't specify a size, the disk is created with exactly the unpacked size of the image.
{% endalert %}

## Creating a cluster image from an HTTP server

The simplest way to create an image is to provide a link to a file hosted on an HTTP server.

{% tabs cvi-http %}

{% tab "Using the CLI" %}

1. Create a [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resource:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-24-04
   spec:
     # Source for the image.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. Verify that the image is created:

   ```bash
   d8 k get clustervirtualimage ubuntu-24-04

   # Short form of the command.
   d8 k get cvi ubuntu-24-04
   ```

   Example output:

   ```console
   NAME           PHASE   CDROM   PROGRESS   AGE
   ubuntu-24-04   Ready   false   100%       23h
   ```

To make DP verify the downloaded file against a checksum, add the [`checksum`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-spec-datasource-http-checksum) block to the source. If the file doesn't match any of the specified checksums, the image moves to the `Failed` phase.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Virtualization** → **Cluster images**.
1. Click **Create**, then select **By link** in the **Source** block.
1. In the **Image name** field, enter the image name.
1. In the **URL** field, specify the link to the image.
1. Click **Create**.
1. Wait until the image reaches the **Ready** state.

{% endtab %}

{% endtabs %}

## Creating a cluster image from a container image registry

DP can pull an image from an external container image registry, but the disk file must be located in the container image under the `/disk` path. The following steps show how to prepare such a container image and create a cluster image from it.

{% tabs cvi-registry %}

{% tab "Using the CLI" %}

1. Download the image file to your local machine:

   ```bash
   curl -L https://cloud-images.ubuntu.com/minimal/releases/noble/release/ubuntu-24.04-minimal-cloudimg-amd64.img -o ubuntu2404.img
   ```

1. Create a `Dockerfile` with the following contents:

   ```Dockerfile
   FROM scratch
   COPY ubuntu2404.img /disk/ubuntu2404.img
   ```

1. Build the container image. The example uses the [docker.com](https://www.docker.com/) registry, which requires an account and a configured environment:

   ```bash
   docker build -t docker.io/<USERNAME>/ubuntu2404:latest
   ```

   Where `<USERNAME>` is the username you specified when registering in the registry.

1. Push the built container image to the registry:

   ```bash
   docker push docker.io/<USERNAME>/ubuntu2404:latest
   ```

1. Create a [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resource that points to the container image:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-2404
   spec:
     dataSource:
       type: ContainerImage
       containerImage:
         image: docker.io/<USERNAME>/ubuntu2404:latest
   EOF
   ```

DP works only with registries that have TLS enabled. If the registry uses its own certificate authority, provide the certificate chain in the [`caBundle`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-spec-datasource-containerimage-cabundle) parameter, and take the credentials for a private registry from the secret specified in the `imagePullSecret` parameter.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Virtualization** → **Cluster images**.
1. Click **Create**, then select **From registry** in the **Source** block.
1. In the **Image name** field, enter the image name.
1. In the **Image in container registry** field, specify the link to the image.
1. Click **Create**.
1. Wait until the image reaches the **Ready** state.

{% endtab %}

{% endtabs %}

## Uploading a cluster image from the command line

If the image file is on your computer, upload it directly. DP creates a temporary upload endpoint for this and waits for the data.

{% tabs cvi-upload %}

{% tab "Using the CLI" %}

1. Create a [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resource with the `Upload` source:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: some-image
   spec:
     # Image source settings.
     dataSource:
       type: Upload
   EOF
   ```

   The resource moves to the `WaitForUserUpload` phase and is ready to accept the file. Start the upload within 10 minutes, otherwise the resource moves to the `Failed` phase and you have to create it again.

1. Get the addresses that accept the file:

   ```bash
   d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}" | jq
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   {
     "external":"https://virtualization.example.com/upload/<SECRET_URL>",
     "inCluster":"http://10.222.165.239/upload"
   }
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Use the `inCluster` address if you upload the file from one of the cluster nodes, and `external` in all other cases.

1. Upload the file to the selected address. The example first downloads the Cirros image and then sends it to the cluster:

   ```bash
   curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
   curl https://virtualization.example.com/upload/<SECRET_URL> --progress-bar -T cirros.img | cat
   ```

   Where `<SECRET_URL>` is the last part of the address from the previous step.

1. Verify that the image has reached the `Ready` phase:

   ```bash
   d8 k get cvi some-image
   ```

   Example output:

   ```console
   NAME         PHASE   CDROM   PROGRESS   AGE
   some-image   Ready   false   100%       1m
   ```

You can also verify the uploaded file against a checksum. To do this, specify the [`checksum`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-spec-datasource-upload-checksum) block in the data source.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Virtualization** → **Cluster images**.
1. Click **Create**, then select **Upload** in the **Source** block.
1. In the **Image name** field, enter the image name.
1. In the **Upload file** block, drag the file to the highlighted area or click **select on your computer**.
1. Select the file in the file manager that opens.
1. Click **Create**.
1. Wait until the image reaches the **Ready** state.

{% endtab %}

{% endtabs %}
