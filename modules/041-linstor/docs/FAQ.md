---
title: "The linstor module: FAQ"
---

## What is difference between LVM and LVMThin?

Briefly:

- LVM is simpler and has performance comparable to native drives;
- LVMThin allows you to use snapshots and overprovisioning, but twice as slow.

## Performance and reliability notes, comparison to Ceph

> You may be interested in our article ["Comparing Ceph, LINSTOR, Mayastor, and Vitastor storage performance in Kubernetes"](https://www.reddit.com/r/kubernetes/comments/v3tzze/comparing_ceph_linstor_mayastor_and_vitastor/).

We take a practical view of the issue. A difference of several tens of percent — in practice it never matters. The difference is several times or more important.

Comparison factors:

- Sequential read and write: do not matter, because on any technology they always run into the network (which is 10Gb/s, which is 1Gb/s). From a practical point of view, this indicator can be completely ignored;
- Random read and write (which is 1Gb/s, which is 10Gb/s):
  - DRBD + LVM 5 times better (latency — 5 times less, IOPS — 5 times more) than Ceph RBD;
  - DRBD + LVM is 2 times better than DRBD + LVMThin.
- If one of the replicas is located on local storage, then the read speed will be approximately equal to the storage device speed;
- If there are no replicas located on local storage, then the write speed will be approximately equal to half the network bandwidth for two replicas, or ⅓ network bandwidth for three replicas;
- With a large number of clients (more than 10, with iodepth 64), Ceph starts to fall behind more (up to 10 times) and consume much more CPU.

All in all, in practice, it doesn’t matter how many knobs you have for tuning, only three factors are significant:

- **Read locality** — if all reading is performed locally, then it works at the speed (throughput, IOPS, latency) of the local disk (the difference is practically insignificant);
- **1 network hop when writing** — in DRBD, the replication is performed by the *client*, and in Ceph, by *server*, so Ceph latency for writing always has at least x2 from DRBD;
- **Complexity of code** — latency of calculations on the datapath (how much assembler code is executed for each io operation), DRBD + LVM is simpler than DRBD + LVMThin, and much simpler than Ceph RBD.

## What to use in which situation?

By default, we use two replicas (the third is an automatically created `diskless` replica used for quorum). This approach guarantees protection against split-brain and a sufficient level of storage reliability, but the following features must be taken into account:

- When one of the replicas (replica A) is unavailable, data is written only to a single replica (replica B). It means that:
  - If at this moment the second replica (replica B) is also turned off, writing and reading will be unavailable;
  - If at the same time the second replica (replica B) is irretrievably lost, then the data will be partially lost (there is only the old, outdated replica A);
  - If the old replica (replica A) was also irretrievably lost, the data will be completely lost.
- When the second replica is turned off, in order to turn it back on (without operator intervention), both replicas must be available (in order to correctly work out the split-brain);
- Enabling a third replica solves both problems (at least two copies of data at any given time), but increases the overhead (network, disk).

It is strongly recommended to have one replica locally. This doubles the possible write bandwidth (with two replicas) and significantly increases the read speed. But if this is not the case, then everything still continues to work normally (but reading over the network, and double network utilization for writing).

Depending on the task, choose one of the following:

- DRBD + LVM — faster (x2) and more reliable (LVM is simpler);
- DRBD + LVMThin — support for snapshots and the possibility of overcommitment.

## Changing the default StorageClass

List the StorageClasses in your cluster:

```bash
kubectl get storageclass
```

Mark the default StorageClass as non-default:

```bash
kubectl annotate storageclass local-path storageclass.kubernetes.io/is-default-class-
```

Mark a StorageClass as default:

```bash
kubectl annotate storageclass linstor-data-r2 storageclass.kubernetes.io/is-default-class=true
```

## How to add existing LVM or LVMThin pool?

> The general method is described in`[LINSTOR storage configuration](configuration.html#linstor-storage-configuration) page.
> Unlike commands listed below it will automatically configure the StorageClasses as well.

Example of adding an existing LVM pool:

```shell
linstor storage-pool create lvm node01 lvmthin linstor_data
```

Example of adding an existing LVMThin pool:

```shell
linstor storage-pool create lvmthin node01 lvmthin linstor_data/data
```

You can also add pools with some volumes have already been created. LINSTOR will just create new ones nearby.

## How to configure Prometheus to use LINSTOR for storing data?

To configure Prometheus to use LINSTOR for storing data:

- [Configure](configuration.html#linstor-storage-configuration) storage-pools and StorageClass;
- Specify the [longtermStorageClass](../300-prometheus/configuration.html#parameters-longtermstorageclass) and [storageClass](../300-prometheus/configuration.html#parameters-storageclass) parameters in the [prometheus](../300-prometheus/) module configuration. E.g.:

  Example:

  ```yaml
  prometheus: |
    longtermStorageClass: linstor-data-r2
    storageClass: linstor-data-r2
  ```

- Wait for the restart of Prometheus Pods.

## How to evict resources from a node?

To do this, just run the command:

```bash
linstor node evacuate <node_name>
```

It will move resources to other free nodes and replicate them.

## Troubleshooting

Problems can arise at different levels of component operation.
This simple cheat sheet will help you quickly navigate through the diagnosis of various problems with LINSTOR-created volumes:

![LINSTOR cheatsheet](../../images/041-linstor/linstor-debug-cheatsheet.svg)
<!--- Source: https://docs.google.com/drawings/d/19hn3nRj6jx4N_haJE0OydbGKgd-m8AUSr0IqfHfT6YA/edit --->

Some typical problems are described here:

### linstor-node cannot start because the drbd module cannot be loaded

Check the status of the `linstor-node` Pods:

```shell
kubectl get pod -n d8-linstor -l app.kubernetes.io/instance=linstor,\
app.kubernetes.io/managed-by=piraeus-operator,app.kubernetes.io/name=piraeus-node
```

If you see that some of them get stuck in `Init:CrashLoopBackOff` state, check the logs of `kernel-module-injector` container:

```shell
kubectl logs -n d8-linstor linstor-node-xxwf9 -c kernel-module-injector
```

The most likely reasons why it cannot load the kernel module:

- You may already have an in-tree kernel version of the DRBDv8 module loaded when LINSTOR requires DRBDv9.
  Check loaded module version: `cat /proc/drbd`. If the file is missing, then the module is not loaded and this is not your case.

- You have Secure Boot enabled.
  Since the DRBD module we provide is compiled dynamically for your kernel (similar to dkms), it has no digital sign.
  We do not currently support running the DRBD module with a Secure Boot configuration.

### Pod cannot start with the `FailedMount` error

#### **Pod is stuck in the `ContainerCreating` phase**

If the Pod is stuck in the `ContainerCreating` phase, and you see the following errors in `kubectl describe pod`:

```text
rpc error: code = Internal desc = NodePublishVolume failed for pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d: checking
for exclusive open failed: wrong medium type, check device health
```

... it means that device is still mounted on one of the other nodes.

To check it, use the following command:

```shell
linstor resource list -r pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d
```

The `InUse` flag will indicate which node the device is being used on.

#### **Pod cannot start due to missing CSI driver**

An example error in `kubectl describe pod`:

```text
kubernetes.io/csi: attachment for pvc-be5f1991-e0f8-49e1-80c5-ad1174d10023 failed: CSINode b-node0 does not
contain driver linstor.csi.linbit.com
```

Check the status of the `linstor-csi-node` Pods:

```shell
kubectl get pod -n d8-linstor -l app.kubernetes.io/component=csi-node,app.kubernetes.io/instance=linstor,\
app.kubernetes.io/managed-by=piraeus-operator,app.kubernetes.io/name=piraeus-csi
```

Most likely they are stuck in the `Init` state, waiting for the node to change its status to `Online` in LINSTOR. Run the following command to check the list of nodes:

```shell
linstor node list
```

If you see any nodes in the `EVICTED` state, then they have been unavailable for 2 hours, to return them to the cluster, run:

```shell
linstor node rst <name>
```

#### **Errors like `Input/output error`**

Such errors usually occur at the stage of creating the file system (mkfs).

Check `dmesg` on the node where your Pod is running:

```shell
dmesg | grep 'Remote failed to finish a request within'
```

If you get any output (there are lines with the "Remote failed to finish a request within ..." parts in the `dmesg` output), then most likely, your disk subsystem is too slow for the normal functioning of DRBD.
