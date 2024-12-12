---
title: "CEPH Storage"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/external/ceph.html
---

To create StorageClass objects based on RBD (RADOS Block Device) or Ceph file system, you can use the csi-ceph module, which allows you to configure connections to one or more Ceph clusters.

## Enabling the module

To enable the `csi-ceph` module, apply the `ModuleConfig` resource:

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

To configure a connection to a Ceph cluster, apply the `CephClusterConnection` resource. Example command:

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
EOF
```

You can check the connection creation status with the following command (the phase should be in `Created` status):

```shell
d8 k get cephclusterconnection ceph-cluster-1
```

## Authentication

To authenticate with the Ceph cluster, you need to define the authentication parameters in the `CephClusterAuthentication` resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterAuthentication
metadata:
  name: ceph-auth-1
spec:
  # User name without `client.`.
  userID: admin
  # Authentication key corresponding to the userID.
  userKey: AQDbc7phl+eeGRAAaWL9y71mnUiRHKRFOWMPCQ==
EOF
```

## Creating StorageClass

The creation of StorageClass objects is done through the `CephStorageClass` resource, which defines the configuration for the desired storage class. Manually creating a StorageClass resource without `CephStorageClass` may lead to errors.

Example of creating a StorageClass based on RBD (RADOS Block Device):

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-rbd-sc
spec:
  clusterConnectionName: ceph-cluster-1
  clusterAuthenticationName: ceph-auth-1
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
  clusterAuthenticationName: ceph-auth-1
  reclaimPolicy: Delete
  type: CephFS
  cephFS:
    fsName: cephfs
EOF
```

Check that the created `CephStorageClass` resources have transitioned to the `Created` phase by running the following command:

```shell
d8 k get cephstorageclass
```

In the output, you should see information about the created `CephStorageClass` resources:

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

If the StorageClass objects are listed, it means the csi-ceph module configuration is complete. Users can now create PersistentVolumes by specifying the created StorageClass objects.
