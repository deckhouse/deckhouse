---
title: "Resizing and migrating virtual machine disks"
permalink: en/user/storage/vm-disk-operations.html
description: "Resizing a virtual machine disk and moving a disk to another storage by changing its storage class."
search: disk resize, disk migration, storageClassName, disk expansion
---

An existing disk can be expanded or moved to another storage without deleting it and without recreating the machine.

## Changing the disk size

You can grow a disk even while it's attached to a running virtual machine. You can't shrink a disk.

{% tabs vd-resize %}

{% tab "Using the CLI" %}

1. Check the current disk size:

   ```bash
   d8 k get vd linux-vm-root
   ```

   Example output:

   ```console
   NAME            PHASE   CAPACITY   VIRTUALMACHINE   AGE
   linux-vm-root   Ready   10Gi       linux-vm         10m
   ```
   {: .nowrap-default }

1. Set the new size in the [`.spec.persistentVolumeClaim.size`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-size) parameter:

   ```bash
   d8 k patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'

   # You can achieve the same result by editing the resource.
   d8 k edit vd linux-vm-root
   ```

1. Verify that the size has changed:

   ```bash
   d8 k get vd linux-vm-root
   ```

   Example output:

   ```console
   NAME            PHASE   CAPACITY   VIRTUALMACHINE   AGE
   linux-vm-root   Ready   11Gi       linux-vm         12m
   ```
   {: .nowrap-default }

{% endtab %}

{% tab "Using the web interface" %}

You can change the size from the virtual machine page:

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM the disk is attached to from the list and click its name.
1. On the **Configuration** tab, in the **Disks** section, click the pencil icon next to the disk size.
1. In the window that opens, specify a larger size.
1. Click **Apply**.

Or from the disk page itself:

1. Go to **Virtualization** → **Disks**.
1. Select the disk you need and click its name.
1. On the **Configuration** tab, specify a larger size in the **Size** field.
1. Click the **Save** button that appears.
1. Check the disk status on its page.

{% endtab %}

{% endtabs %}

## Migrating disks to other storage

In commercial Deckhouse Platform (DP) editions, you can move a disk to another storage by changing its storage class. The move works both for disks defined in the VM specification and for disks attached as a separate resource.

{% alert level="warning" %}
The virtual machine must be in the `Running` phase, and the source and target storage must be of the same type. You can't move a disk from a file system volume to a block device or the other way around.
{% endalert %}

To move a disk, specify the new storage class in the [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-storageclassname) parameter:

```bash
d8 k patch vd disk --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'

# You can achieve the same result by editing the resource.
d8 k edit vd disk
```

After that, a live migration of the VM starts, during which the disk moves to the new storage.

If you need to move several disks of the same machine, change the storage class one disk at a time:

```bash
d8 k patch vd disk1 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
d8 k patch vd disk2 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
```

The module retries a failed migration with a growing delay. The first attempt runs immediately, the next ones after 5 and 10 seconds, then the delay doubles and from the seventh attempt stays at 300 seconds. To cancel the migration, restore the previous storage class in the specification.
