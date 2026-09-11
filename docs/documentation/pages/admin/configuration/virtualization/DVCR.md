---
title: "Virtual machine image storage"
permalink: en/admin/configuration/virtualization/image-storage.html
description: "Internal virtual machine image storage (DVCR): volume size and storage class, scheduled cleanup of stale data."
search: DVCR, image storage, volume size, storage cleanup, garbage collection
---

The module stores virtual machine images in an internal container image storage (DVCR) that resides on a persistent volume of the cluster. Images travel from there to virtual machine disks, so the size of the volume determines how many images fit into the cluster.

## Size and storage class

Set the volume size and the storage class in the [`.spec.settings.dvcr.storage`](/modules/virtualization/configuration.html#parameters-dvcr-storage) block. To expand the storage, increase the volume size.

{% alert level="warning" %}
After the volume is created, you can't reduce its size or change its storage class.
{% endalert %}

## Cleaning up image storage

When images and disks are deleted from the cluster, their data remains in DVCR for some time. To keep the storage from filling up with stale data, the module runs garbage collection on a schedule.
By default, it runs daily at 02:00. To set your own schedule, use the [`.spec.settings.dvcr.gc.schedule`](/modules/virtualization/configuration.html#parameters-dvcr-gc-schedule) parameter in the `virtualization` [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig):

{% tabs dvcr-gc %}

{% tab "Using the CLI" %}

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

While garbage collection is running, the storage works in read-only mode, so creating images and disks is postponed until it completes.

To see how much space is occupied and which data will be removed during the next collection, run:

```bash
d8 k -n d8-virtualization exec deploy/dvcr -- dvcr-cleaner gc check
```

Example output:

```console
Found 2 cvi, 5 vi, 1 vd manifests in registry
Found 1 cvi, 5 vi, 11 vd resources in cluster
  Total     Used    Avail     Use%
36.3GiB  13.1GiB  22.4GiB      39%
Images eligible for cleanup:
KIND                   NAMESPACE            NAME
ClusterVirtualImage                         debian-12
VirtualDisk            default              debian-10-root
VirtualImage           default              ubuntu-2404
```
{: .nowrap-default }

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Deckhouse** → **Modules**.
1. Select the `virtualization` module from the list.
1. In the window that opens, on the **Configuration** tab, enable the **Advanced settings** toggle.
1. In the **Disk and ISO image storage** block, set the schedule in the **Cleanup schedule in Cron format** field.
1. Click **Save**.

{% endtab %}

{% endtabs %}
