---
title: "The csi-ceph module"
---

{% alert level="warning" %}
When switching to this module from the ceph-csi module, an automatic migration is performed, but it requires preparation:
1. Scale all operators (redis, clickhouse, kafka, etc.) to zero replicas; during migration, operators in the cluster must not be running. The only exception is the prometheus operator in Deckhouse, which will be automatically disabled during migration.
2. Disable the ceph-csi module and enable the csi-ceph module.
3. Wait for the migration process to complete in the Deckhouse logs (indicated by "Finished migration from Ceph CSI module").
4. Create test pod/PVC to verify CSI functionality.
5. Restore operators to a working state.
If a Ceph StorageClass was created manually and not via the CephCSIDriver resource, manual migration is required.
In this case, please contact technical support.
{% endalert %}

{% alert level="info" %}
For working with snapshots, the [snapshot-controller](../../snapshot-controller/) module must be connected.
{% endalert %}

Ceph is a scalable distributed storage system that ensures high availability and fault tolerance of data. Deckhouse supports integration with Ceph clusters, enabling dynamic storage management and the use of StorageClass based on RBD (RADOS Block Device) or CephFS.

This page provides instructions on connecting Ceph to Deckhouse, configuring authentication, creating StorageClass objects, and verifying storage functionality.

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

To configure a connection to a Ceph cluster, apply the [CephClusterConnection](cr.html#cephclusterconnection) resource. Example usage:

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

You can check the connection status with the following command (the `Phase` should be in `Created` status):

```shell
d8 k get cephclusterconnection ceph-cluster-1
```

## Creating StorageClass

The creation of StorageClass objects is done through the [CephStorageClass](cr.html#cephstorageclass) resource, which defines the configuration for the desired StorageClass. Manually creating a StorageClass resource without [CephStorageClass](cr.html#cephstorageclass) may lead to errors. Example of creating a StorageClass based on RBD (RADOS Block Device):

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

Check that the created [CephStorageClass](cr.html#cephstorageclass) resources have transitioned to the `Created` phase by running the following command:

```shell
d8 k get cephstorageclass
```

In the output, you should see information about the created [CephStorageClass](cr.html#cephstorageclass) resources:

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
