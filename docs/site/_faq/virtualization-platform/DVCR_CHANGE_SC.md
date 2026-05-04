---
title: How do I change the DVCR StorageClass when a PVC already exists?
section: platform_management
lang: en
---

{% alert level="warning" %}
You can change the DVCR storage StorageClass only by recreating the PVC. All images previously loaded into DVCR are lost, along with data for existing [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) and [VirtualImage](/modules/virtualization/cr.html#virtualimage) resources.
{% endalert %}

The [`spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`](configuration.html#parameters-dvcr-storage-persistentvolumeclaim-storageclassname) field in the `virtualization` module ModuleConfig sets the StorageClass for the virtual machine image storage volume (DVCR). While a PVC for that volume exists in the `d8-virtualization` namespace, you cannot change the field via the API.

You cannot change `storageClassName` on an existing PVC in place, and DVCR data is not migrated between storage classes.

To change the DVCR StorageClass, perform the following steps:

1. Stop DVCR:

   ```shell
   d8 k -n d8-virtualization scale deployment dvcr --replicas=0
   ```

1. List PVCs in the `d8-virtualization` namespace and find the PVC for the DVCR volume:

   ```shell
   d8 k get pvc -n d8-virtualization
   ```

1. Delete the PVC you found. Replace `<pvc-name>` with the resource name. If the command fails because of insufficient permissions, run it as `system:sudouser`:

   ```shell
   d8 k --as system:sudouser -n d8-virtualization delete pvc/<pvc-name>
   ```

1. Set the new StorageClass in ModuleConfig. Replace `<storage-class-name>` with the class name you need.

   ```shell
   d8 k patch mc virtualization --type merge -p '{"spec":{"settings":{"dvcr":{"storage":{"persistentVolumeClaim":{"storageClassName":"<storage-class-name>"}}}}}}'
   ```

   Example output:

   ```console
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Start DVCR:

   ```shell
   d8 k -n d8-virtualization scale deployment dvcr --replicas=1
   ```

1. Verify the PVC:

   ```shell
   d8 k get pvc -n d8-virtualization
   ```

   Example output:

   ```console
   NAME   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          VOLUMEATTRIBUTESCLASS   AGE
   dvcr   Bound    pvc-b43f2e33-32cc-435a-aa1d-b53df35b030a   100Gi      RWO            linstor-thin-r1-hdd   <unset>                 34s
   ```

{% alert level="warning" %}
The storage for the chosen StorageClass must be reachable from the nodes where DVCR runs: system nodes, or worker nodes if the cluster has no system nodes.
{% endalert %}
