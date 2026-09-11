---
title: "Exporting virtual machine disk data"
permalink: en/user/virtualization/data-export.html
description: "Exporting the contents of a virtual machine disk or its snapshot to a file outside the cluster."
search: disk export, data export, DataExport, d8 data export
---

Export writes the contents of a disk or its snapshot to a file so that you can move the data outside the cluster.

{% tabs data-export %}

{% tab "Using the CLI" %}

You can export virtual machine disks and disk snapshots with the `d8` utility (version 0.20.7 and later). For this feature to work, the [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) module has to be enabled.

> **Important:** The disk must not be in use at the moment of export. If the disk is attached to a virtual machine, stop the VM first.

An example of exporting a disk, with the command run on a cluster node:

```bash
d8 data export download -n <NAMESPACE> vd/<VD_NAME> -o file.img
```

An example of exporting a disk snapshot, with the command run on a cluster node:

```bash
d8 data export download -n <NAMESPACE> vds/<VD_SNAPSHOT_NAME> -o file.img
```

If you export data from somewhere other than a cluster node (for example, from your local machine), use the `--publish` flag.

> To import a downloaded disk back into the cluster, upload it as an [image](images.html#uploading-an-image-from-the-command-line) or as a [disk](disks.html#uploading-a-disk-from-the-command-line).

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Stop the virtual machine the disk is attached to. While the disk is in use, the **Download** item isn't available.
1. Go to **Virtualization** → **Disks**.
1. In the row of the disk you need, click the ellipsis button and select **Download**.
1. Wait until the **Preparing...** step in the **Download** window changes to **Ready**: the file downloads automatically as soon as it's created. If that doesn't happen, click **Download** in the window.

{% endtab %}

{% endtabs %}

{% alert level="warning" %}
While an export is active, the disk is in the `Exporting` phase and a virtual machine with this disk won't start. The export ends when its lifetime expires or when the DataExport resource created for it is deleted.
{% endalert %}
