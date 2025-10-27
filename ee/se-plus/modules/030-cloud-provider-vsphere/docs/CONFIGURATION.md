---
title: "Cloud provider — VMware vSphere: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in vSphere.

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

If the cluster control plane is hosted on a virtual machines or bare-metal servers, the cloud provider uses the settings from the `cloud-provider-vsphere` module in the Deckhouse configuration (see below). Otherwise, if the cluster control plane is hosted in a cloud, the cloud provider uses the [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) structure for configuration.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/node-manager/cr.html#nodegroup) custom resource of the `node-manager` module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` parameter of NodeGroup). In the case of the vSphere cloud provider, the instance class is the [`VsphereInstanceClass`](cr.html#vsphereinstanceclass) custom resource that stores specific parameters of the machines.

## Storage

The module automatically creates a StorageClass for each Datastore and DatastoreCluster in the zone (or zones).

Also, it can set the name of StorageClass that will be used in the cluster by default (the [default](#parameters-storageclass-default) parameter), and
filter out the unnecessary StorageClasses (the [exclude](#parameters-storageclass-exclude) parameter).

### CSI

By default, the storage subsystem uses CNS volumes with the ability of online-resize. FCD volumes are also supported, but only in the legacy or migration modes. You can set this via the [compatibilityFlag](#parameters-storageclass-compatibilityflag) parameter.

### Important information concerning the increase of the PVC size

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) f volume-resizer, CSI, and vSphere API, you have to do the following after increasing the PVC size:

1. On the node where the Pod is located, run the `d8 k cordon <node_name>` command.
2. Delete the Pod.
3. Make sure that the resize was successful. The PVC object must *not have* the `Resizing` condition.
   > The `FileSystemResizePending` state is OK.
4. On the node where the Pod is located, run the `d8 k uncordon <node_name>` command.

{% include module-settings.liquid %}
