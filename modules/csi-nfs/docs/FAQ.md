---
title: "The csi-nfs module: FAQ"
description: CSI NFS module FAQ
---

## How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-nfs` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
kubectl -n d8-csi-nfs get pod -owide -w
```

## Is it possible to change the parameters of an NFS server for already created PVs?

No, the connection data to the NFS server is stored directly in the PV manifest and cannot be changed. Changing the Storage Class also does not affect the connection settings in already existing PVs.

## How to Create Volume Snapshots?

In `csi-nfs`, snapshots are created by archiving the volume directory. The archive is saved in the root folder of the NFS server specified in the `spec.connection.share` parameter.

### Step 1: Enabling the snapshot-controller

First, you need to enable the snapshot-controller:

```shell
kubectl apply -f -<<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
EOF

```

### Step 2: Creating a Volume Snapshot

Now you can create volume snapshots. To do this, execute the following command with the necessary parameters:

```shell
kubectl apply -f -<<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
  namespace: <name of the namespace where the PVC is located>
spec:
  volumeSnapshotClassName: csi-nfs-snapshot-class
  source:
    persistentVolumeClaimName: <name of the PVC to snapshot>
EOF

```


### Step 3: Checking the Snapshot Status

To check the status of the created snapshot, execute the command:

```shell
kubectl get volumesnapshot

```

This command will display a list of all snapshots and their current status.
