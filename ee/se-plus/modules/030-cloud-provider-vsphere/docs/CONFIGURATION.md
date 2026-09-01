---
title: "Cloud provider — VMware vSphere: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in vSphere.

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

The source of the settings depends on where the cluster control plane is hosted.
If the control plane runs on virtual machines or bare metal, the module uses its own settings described below.
If the control plane is hosted in a cloud, the module uses the [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) resource.

The number of nodes and their provisioning parameters are set in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource of the `node-manager` module.
The same resource specifies the instance class of the node group in the `cloudInstances.classReference` parameter.
For vSphere, the instance class is the [VsphereInstanceClass](cr.html#vsphereinstanceclass) custom resource that describes the parameters of the virtual machines.

## Connecting to vCenter

The vCenter address and credentials are set by the [`host`](#parameters-host), [`username`](#parameters-username), and [`password`](#parameters-password) parameters.

The module connects to vCenter over TLS and verifies its certificate.
If the certificate is issued by a custom or enterprise certificate authority, pass the certificate chain in the [`caBundle`](#parameters-cabundle) parameter.
The [`insecure`](#parameters-insecure) parameter set to `true` disables certificate verification completely.
Set either `caBundle` or `insecure: true`, since these parameters are not accepted together.

## Storage

The module creates a StorageClass for each Datastore and DatastoreCluster in the zones in use.
If SPBM storage policies are configured in vSphere, the module additionally creates a StorageClass for each combination of a Datastore and a policy.

Unnecessary StorageClasses are filtered out by the [`exclude`](#parameters-storageclass-exclude) parameter, which takes names or regular expressions.
The expression is matched against the Datastore name, so an exclusion removes both the base StorageClass and all StorageClasses with storage policies for that Datastore.

To set the default StorageClass, use the [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) global parameter.
The module's [`default`](#parameters-storageclass-default) parameter is deprecated.

### CSI

By default, the storage subsystem uses CNS volumes with online resize support.
The legacy mode with FCD volumes is also supported, but resizing is not available in it.
The mode is selected by the [`compatibilityFlag`](#parameters-storageclass-compatibilityflag) parameter.

### Expanding a PersistentVolumeClaim

Due to [specifics](https://github.com/kubernetes-csi/external-resizer/issues/44) of the CSI volume-resizer and the vSphere API, perform the following steps after expanding a PersistentVolumeClaim:

1. Run `d8 k cordon <NODE_NAME>` for the node that hosts the Pod.
1. Delete the Pod that uses the PersistentVolumeClaim.
1. Wait for the operation to complete. The PersistentVolumeClaim must no longer have the `Resizing` condition, while the `FileSystemResizePending` condition is not an issue.
1. Run `d8 k uncordon <NODE_NAME>`.

{% include module-settings.liquid %}
