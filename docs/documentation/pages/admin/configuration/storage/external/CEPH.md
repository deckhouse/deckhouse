---
title: "Distributed Ceph storage"
permalink: en/admin/configuration/storage/external/ceph.html
---

Ceph is a scalable distributed storage system that provides high availability and fault tolerance for data. Deckhouse supports integration with Ceph clusters, enabling dynamic storage management and the use of StorageClasses based on RBD (RADOS Block Device) or CephFS.

This page provides instructions on connecting Ceph to Deckhouse, configuring authentication, creating StorageClass objects, and verifying storage functionality.

{% alert level="warning" %}
When switching to this module from the`ceph-csi` module, an automatic migration is performed, but it requires preparation:
1. Scale all operators (redis, clickhouse, kafka, etc.) to zero replicas; during migration, operators in the cluster must not be running. The only exception is the `prometheus` operator in Deckhouse, which will be automatically disabled during migration.
1. Disable the `ceph-csi` module and enable the `csi-ceph` module.
1. Wait for the migration process to complete in the Deckhouse logs (indicated by "Finished migration from Ceph CSI module").
1. Create test pod/PVC to verify CSI functionality.
1. Restore operators to a working state.
   If the CephCSIDriver resource has a `spec.cephfs.storageClasses.pool` field set to a value other than `cephfs_data`, the migration will fail with an error.
   If a Ceph StorageClass was created manually and not via the CephCSIDriver resource, manual migration is required.
   In these cases, contact technical support.
{% endalert %}

## Enabling the module

To connect a Ceph cluster in Deckhouse, you need to enable the `csi-ceph` module. To do this, apply the ModuleConfig resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-ceph
spec:
  enabled: true
EOF
```

## Connecting to a Ceph cluster

To configure a connection to a Ceph cluster, apply the [CephClusterConnection](../../../reference/cr/cephclusterconnection/) resource. Example usage:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterConnection
metadata:
  name: ceph-cluster-1
spec:
  # FSID/UUID of the Ceph cluster.
  # The FSID/UUID of the Ceph cluster can be obtained using the `ceph fsid` command.
  clusterID: 2bf085fc-5119-404f-bb19-820ca6a1b07e
  # List of Ceph monitor IP addresses in the format 10.0.0.10:6789.
  monitors:
    - 10.0.0.10:6789
  # User name without `client.`.
  # The user name can be obtained using the `ceph auth list` command.
  userID: admin
  # Authentication key corresponding to the userID.
  # The authentication key can be obtained using the `ceph auth get-key client.admin` command.
  userKey: AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==
EOF
```

Verify the creation of the connection using the following command (`Phase` should be `Created`):

```shell
d8 k get cephclusterconnection ceph-cluster-1
```

## Creating StorageClass

The creation of StorageClass objects is done through the [CephStorageClass](../../../reference/cr/cephstorageclass/) resource, which defines the configuration for the desired StorageClass. Manually creating a StorageClass resource without [CephStorageClass](../../../reference/cr/cephstorageclass/) may lead to errors. Example of creating a StorageClass based on RBD (RADOS Block Device):

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-rbd-sc
spec:
  clusterConnectionName: ceph-cluster-1
  reclaimPolicy: Delete
  type: RBD
  rbd:
    defaultFSType: ext4
    pool: ceph-rbd-pool
EOF
```

Example of creating a StorageClass based on Ceph file system:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-fs-sc
spec:
  clusterConnectionName: ceph-cluster-1
  reclaimPolicy: Delete
  type: CephFS
  cephFS:
    fsName: cephfs
EOF
```

Check that the created [CephStorageClass](../../../reference/cr/cephstorageclass/) resources have transitioned to the `Created` phase by running the following command:

```shell
d8 k get cephstorageclass
```

In the output, you should see information about the created [CephStorageClass](../../../reference/cr/cephstorageclass/) resources:

```console
NAME          PHASE     AGE
ceph-rbd-sc   Created   1h
ceph-fs-sc    Created   1h
```

Check the created StorageClass using the following command:

```shell
d8 k get sc
```

In the output, you should see information about the created StorageClass:

```console
NAME          PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
ceph-rbd-sc   rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
ceph-fs-sc    rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
```

If the StorageClass objects appear, it means the `csi-ceph` module configuration is complete. Users can now create PersistentVolumes by specifying the created StorageClass objects.

## Listing RBD volumes mapped on each node

To get a list of RBD volumes mounted on each node of the cluster, run the following command:

```shell
d8 k -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash
```

## Supported Ceph Versions

- Official support: Ceph version 16.2.0 and above.
- Compatibility: The solution works with Ceph clusters version 14.2.0 and above; however, upgrading to Ceph version 16.2.0 or later is recommended to ensure maximum stability and access to the latest fixes.

## Supported Volume Access Modes

- RBD: ReadWriteOnce (RWO): access to a block volume is only possible from a single node.
- CephFS: ReadWriteOnce (RWO) and ReadWriteMany (RWX): simultaneous access to the file system from multiple nodes.

## Examples

Example definition of a [CephClusterConnection](../../../reference/cr/cephclusterconnection/):

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterConnection
metadata:
  name: ceph-cluster-1
spec:
  clusterID: 0324bfe8-c36a-4829-bacd-9e28b6480de9
  monitors:
  - 172.20.1.28:6789
  - 172.20.1.34:6789
  - 172.20.1.37:6789
  userID: admin
  userKey: AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==
```

You can verify that the object has been created with the following command (`Phase` should be `Created`):

```shell
d8 k get cephclusterconnection <name-of-cephclusterconnection>
```

Example definition of a [CephStorageClass](../../../reference/cr/cephstorageclass/):

- For RBD:

  ```yaml
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: CephStorageClass
  metadata:
    name: ceph-rbd-sc
  spec:
    clusterConnectionName: ceph-cluster-1
    reclaimPolicy: Delete
    type: RBD
    rbd:
      defaultFSType: ext4
      pool: ceph-rbd-pool  
  ```

- For CephFS:

    ```yaml
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: CephStorageClass
  metadata:
    name: ceph-fs-sc
  spec:
    clusterConnectionName: ceph-cluster-1
    reclaimPolicy: Delete
    type: CephFS
    cephFS:
      fsName: cephfs
  ```

You can verify that the object has been created with the following command (`Phase` should be `Created`):

```shell
d8 k get cephstorageclass <name-of-cephstorageclass>
```
