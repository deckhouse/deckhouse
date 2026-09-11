---
title: "Virtual machine images"
permalink: en/user/virtualization/images.html
description: "Virtual machine images in a project: sources and storage options, creating from an HTTP server, a container registry, a disk, and a disk snapshot."
search: VM images, VirtualImage, creating an image, image upload
---

An image holds the contents of a disk that you use to create virtual machine disks. A [VirtualImage](/modules/virtualization/cr.html#virtualimage) is created in a project and available only in the project or namespace where it was created.

{% alert level="warning" %}
To make the same image available to every project in the cluster, you need a cluster image, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage). Only an administrator can create it; the procedure is described in [Cluster images of virtual machines](../../admin/configuration/virtualization/cluster-images.html).
{% endalert %}

A virtual machine accesses an attached image in read-only mode.

An image appears in a project in three steps:

1. You create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource and specify a data source in it.
1. DP downloads the image from that source to the storage, which is either DVCR or a PVC, depending on the selected type.
1. The downloaded image becomes available for creating disks.

## Sources and storage options

The image source can be an HTTP server hosting the image file, a container image registry, or a file on your computer that you upload from the command line. You can also create an image from another image, from a virtual machine disk, or from a disk snapshot.

Image types, supported file formats, and compression algorithms are described in [Image types and formats](../../admin/configuration/virtualization/cluster-images.html#image-types-and-formats).

The downloaded image is stored in one of two ways, set by the [`.spec.storage`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-storage) parameter:

- `ContainerRegistry`: The default option, the image is stored in DVCR.
- `PersistentVolumeClaim`: The image is stored in a PVC. This option is preferable if the storage can clone PVCs quickly, because disks are created from such an image faster.

{% alert level="warning" %}
An image stored with `storage: PersistentVolumeClaim` can only be used to create disks in the same storage class.
{% endalert %}

The `PHASE` column in the `d8 k get vi` output shows the progress of image creation; for its values, see the [`.status.phase`](/modules/virtualization/cr.html#virtualimage-v1alpha2-status-phase) field. To follow the creation in real time, add the `-w` flag. If an image stays not ready for a long time, check the [`.status.conditions`](/modules/virtualization/cr.html#virtualimage-v1alpha2-status-conditions) block and the `d8 k describe vi` output for the reason.

Until an image reaches the `Ready` phase, you can change its `.spec` block, and the download restarts after each change. For a ready image, the `.spec` block can no longer be changed. For all image parameters, see [VirtualImage](/modules/virtualization/cr.html#virtualimage).

## Creating an image from an HTTP server

The simplest way to create an image is to provide a link to a file hosted on an HTTP server.

{% tabs vi-http %}

{% tab "Using the CLI" %}

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource. In the example, the image is stored in DVCR:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-24-04
   spec:
     # Store the image in DVCR.
     storage: ContainerRegistry
     # Source for the image.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. Verify that the image is created:

   ```bash
   d8 k get virtualimage ubuntu-24-04

   # Short form of the command.
   d8 k get vi ubuntu-24-04
   ```

   Example output:

   ```console
   NAME           PHASE   CDROM   PROGRESS   AGE
   ubuntu-24-04   Ready   false   100%       23h
   ```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Images**.
1. Click **Create**.
1. In the **Source** block, select **By link**.
1. In the form that opens, enter the image name in the **Image name** field.
1. In the **Storage** block, select `ContainerRegistry` in the **Storage type** field.
1. In the **URL** field, specify the link to the image.
1. Click **Create**.
1. Check the image status on its page.

{% endtab %}

{% endtabs %}

### Verifying the integrity of a downloaded image

The [`checksum`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-http-checksum) block makes DP verify what it downloaded from the HTTP server. The image reaches the `Ready` phase only if the downloaded file matches every specified checksum, otherwise the resource ends up in the `Failed` phase:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-24-04
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
      checksum:
        sha256: 78be890d71dde316c412da2ce8332ba47b9ce7a29d573801d2777e01aa20b9b5
EOF
```

Take the checksum from the mirror that publishes the image and put it in the field of the matching algorithm:

| Field         | Algorithm                             | Verification speed |
| ------------- | ------------------------------------- | ------------------ |
| `sha1`        | SHA-1                                 | ~1.6 GB/s          |
| `sha256`      | SHA-256                               | ~1.5 GB/s          |
| `md5`         | MD5                                   | ~700 MB/s          |
| `sha512`      | SHA-512                               | ~570 MB/s          |
| `streebog256` | GOST R 34.11-2012 (Streebog), 256-bit | ~17 MB/s           |
| `streebog512` | GOST R 34.11-2012 (Streebog), 512-bit | ~17 MB/s           |

The speeds are an order of magnitude, not a promise. They were measured on an x86-64 CPU with the SHA instruction set, and a CPU without it computes SHA-1 and SHA-256 several times slower. What holds on any CPU is the distance between the rows. SHA-1 and SHA-256 are computed by dedicated instructions, MD5 and SHA-512 by hand-written assembly, and all four hash data faster than it arrives over the network, so their cost stays invisible against the download itself.

The Streebog algorithms have no hardware support anywhere and are about two orders of magnitude slower. For a 10 GiB image, that's around ten minutes of hashing alone, and image creation becomes CPU-bound rather than network-bound. Use them only when a GOST checksum is actually required. Both lengths cost the same, because GOST R 34.11-2012 uses one compression function for 256 and 512 bits, and the shorter variant differs only in the initial value.

Checksums are computed in a single pass over the downloaded data, so specifying several checksums at once costs the sum of their times. Empty fields cost nothing.

The same block is available for the `Upload` source in the [`dataSource.upload.checksum`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-upload-checksum) parameter and works the same way. The uploaded data is verified against every specified checksum, and on a mismatch the resource stays in the `Failed` phase.

### Storing an image in a PVC

To create disks from an image faster, store it in a PVC. DP can then clone the volume instead of unpacking the image again.

{% tabs vi-pvc %}

{% tab "Using the CLI" %}

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource with the `PersistentVolumeClaim` storage type:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-24-04-pvc
   spec:
     # Storage settings for the project image.
     storage: PersistentVolumeClaim
     persistentVolumeClaim:
       # Specify the name of your StorageClass.
       storageClassName: rv-thin-r2
     # Source for the image.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

   If the [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-persistentvolumeclaim-storageclassname) parameter isn't set, DP uses the cluster-wide default StorageClass or the class set for images in the [module settings](../../admin/configuration/virtualization/storage-classes.html).

1. Verify that the image is created:

   ```bash
   d8 k get vi ubuntu-24-04-pvc
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               PHASE   CDROM   PROGRESS   AGE
   ubuntu-24-04-pvc   Ready   false   100%       23h
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Images**.
1. Click **Create**.
1. In the **Source** block, select **By link**.
1. In the form that opens, enter the image name in the **Image name** field.
1. In the **Storage** block, select `PersistentVolumeClaim` in the **Storage type** field.
1. In the **Storage class** field, select a StorageClass or keep the default one.
1. In the **URL** field, specify the link to the image.
1. Click **Create**.
1. Check the image status on its page.

{% endtab %}

{% endtabs %}

## Creating an image from a container image registry

DP can pull an image from an external container image registry, but the disk file must be located in the container image under the `/disk` path. The following steps show how to prepare such a container image and create a project image from it.

{% tabs vi-registry %}

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

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource that points to the container image:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-2404
   spec:
     storage: ContainerRegistry
     dataSource:
       type: ContainerImage
       containerImage:
         image: docker.io/<USERNAME>/ubuntu2404:latest
   EOF
   ```

DP works only with registries that have TLS enabled. If the registry uses its own certificate authority, provide the certificate chain in the [`caBundle`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-containerimage-cabundle) parameter, and take the credentials for a private registry from the secret specified in the `imagePullSecret` parameter.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Images**.
1. Click **Create**.
1. In the **Source** block, select **From registry**.
1. In the form that opens, enter the image name in the **Image name** field.
1. In the **Storage** block, select `ContainerRegistry` in the **Storage type** field.
1. In the **Image in container registry** field, specify the path to the container image.
1. Click **Create**.
1. Check the image status on its page.

{% endtab %}

{% endtabs %}

## Uploading an image from the command line

If the image file is on your computer, upload it directly. DP creates a temporary upload endpoint for this and waits for the data.

{% tabs vi-upload %}

{% tab "Using the CLI" %}

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource with the `Upload` source:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: some-image
   spec:
     # Storage settings for the project image.
     storage: ContainerRegistry
     # Image source settings.
     dataSource:
       type: Upload
   EOF
   ```

   The resource moves to the `WaitForUserUpload` phase and is ready to accept the file. Start the upload within 10 minutes, otherwise the resource moves to the `Failed` phase and you have to create it again.

1. Get the addresses that accept the file:

   ```bash
   d8 k get vi some-image -o jsonpath="{.status.imageUploadURLs}" | jq
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   {
     "external": "https://virtualization.example.com/upload/<SECRET_URL>",
     "inCluster": "http://10.222.165.239/upload"
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
   d8 k get vi some-image
   ```

   Example output:

   ```console
   NAME         PHASE   CDROM   PROGRESS   AGE
   some-image   Ready   false   100%       1m
   ```

You can also verify the uploaded file against a checksum. To do this, specify the [`checksum`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-upload-checksum) block in the data source. The checksums are computed over the bytes the client sends, and on a mismatch the resource stays in the `Failed` phase, so the upload has to be repeated on a recreated resource.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Images**.
1. Click **Create**, then select **Upload** in the **Source** block.
1. In the **Image name** field, enter the image name.
1. In the **Upload file** block, drag the file to the highlighted area or click **select on your computer**.
1. Select the file in the file manager that opens.
1. Click **Create**.
1. Wait until the image reaches the **Ready** state.

{% endtab %}

{% endtabs %}

## Creating an image from a disk

You can create an image from a [disk](disks.html) if the disk isn't attached to any virtual machine, or if the machine it's attached to is powered off.

{% tabs vi-from-disk %}

{% tab "Using the CLI" %}

Create an image using the required disk as the source:

```bash
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

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Images**.
1. Click **Create**.
1. In the **Source** block, select **Create from**.
1. In the form that opens, enter `linux-vm-root` in the **Image name** field.
1. In the **Storage** block, select `ContainerRegistry` in the **Storage type** field.
1. In the **Source** field, select the disk you need from the drop-down list.
1. Click **Create**.
1. Check the image status on its page.

{% endtab %}

{% endtabs %}

## Creating an image from a disk snapshot

You can create an image from a [disk snapshot](snapshots.html#creating-disk-snapshots) if the snapshot is in the `Ready` phase.

{% tabs vi-from-snapshot %}

{% tab "Using the CLI" %}

Create an image using the disk snapshot as the source:

```bash
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

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Images**.
1. Click **Create**.
1. In the form that opens, enter the image name in the **Image name** field.
1. In the **Storage** block, select the image storage type in the **Storage type** field.
1. In the **Source** block, select **Create from**.
1. In the **Source** field, expand the list and select the snapshot you need in the **Disk snapshots** group.
1. Click **Create**.
1. Check the image status on its page.

{% endtab %}

{% endtabs %}

Image properties are convenient to review in the web interface, in **Virtualization** → **Images**:

- The list shows the image name, status, size, format in the **Type** column, and visibility scope in the **Availability** column, and the filters narrow it down by status, image, and type.
- On the image page, the **Information** tab collects the creation parameters in the **Storage** and **Source** blocks, and the **State** block shows the average download speed, format, unpacked size, size in storage, creation time, the **CD-ROM** flag, and the image path in DVCR.
- The **Meta** and **YAML** tabs show labels with annotations and the full resource specification.
