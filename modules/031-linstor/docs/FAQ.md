---
title: "The linstor module: FAQ"
---

<div class="docs__information warning active">
The module is actively developed, and it might significantly change in the future.
</div>

## What is difference between LVM and LVMThin?

Briefly:
- LVM is simpler and has performance comparable to native drives;
- LVMThin allows you to use snapshots and overprovisioning, but twice as slow.

For more details read the next section.

## Performance and reliability notes and comparison to Ceph

We take a practical view of the issue. A difference of several tens of percent — in practice it never matters. The difference is several times or more important. 

Comparison factors:
- Sequential read and write: do not matter, because on any technology they always run into the network (which is 10Gb/s, which is 1Gb/s). From a practical point of view, this indicator can be completely ignored;
- Random read and write (which is 1Gb/s, which is 10Gb/s):
  - drbd+lvm 5 times better (latency — 5 times less, IOPS — 5 times more) than ceph-rbd; 
  - drbd+lvm is 2 times better than drbd+lvmthin.
- If one of the replicas is located on local storage, then the read speed will be approximately equal to the storage device speed; 
- If there are no replicas located on local storage, then the write speed will be approximately equal to half the network bandwidth for two replicas, or ⅓ network bandwidth for three replicas;
- With a large number of clients (more than 10, with iodepth 64), ceph starts to fall behind more (up to 10 times) and consume much more CPU.

All in all, in practice, it doesn’t matter how many knobs you have for tuning, only three factors are significant: 
- **Read locality** — if all reading is performed locally, then it works at the speed (throughput, IOPS, latency) of the local disk (the difference is practically insignificant);
- **1 network hop when writing** — in drbd, the replication is performed by the *client*, and in ceph, by *server*, so ceph latency for writing always has at least x2 from drbd;
- **Complexity of code** — latency of calculations on the datapath (how much assembler code is executed for each io operation), drbd+lvm is simpler than drbd+lvmthin, and much simpler than ceph-rbd. 

## What to use in which situation?

By default, we use two replicas (the third is an automatically created diskless replica used for quorum). This approach guarantees protection against split-brain and a sufficient level of storage reliability, but the following features must be taken into account:
  - When one of the replicas (replica A) is unavailable, data is written only to a single replica (replica B). It means that:
    - If at this moment the second replica (replica B) is also turned off, writing and reading will be unavailable;
    - If at the same time the second replica (replica B) is irretrievably lost, then the data will be partially lost (there is only the old, outdated replica A);
    - If the old replica (replica A) was also irretrievably lost, the data will be completely lost.
  - When the second replica is turned off, in order to turn it back on (without operator intervention), both replicas must be available (in order to correctly work out the split-brain);
  - Enabling a third replica solves both problems (at least two copies of data at any given time), but increases the overhead (network, disk).

It is strongly recommended to have one replica locally. This doubles the possible write bandwidth (with two replicas) and significantly increases the read speed. But if this is not the case, then everything still continues to work normally (but reading over the network, and double network utilization for writing).

Depending on the task, choose one of the following:
- drbd+lvm — faster (x2) and more reliable (lvm is simpler);
- drbd+lvmthin — support for snapshots and the possibility of overcommitment.

## How to add existing LVM or LVMThin pool?

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
- Configure storage-pools and StorageClass, as shown in [Usage](usage.html);
- Specify the [longtermStorageClass](../300-prometheus/configuration.html#parameters-longtermstorageclass) and [storageClass](../300-prometheus/configuration.html#parameters-storageclass) parameters in the [prometheus](../300-prometheus/) module configuration. E.g.:
  ```yaml
  prometheus: |
    longtermStorageClass: linstor-data-r2
    storageClass: linstor-data-r2
  ```
- Wait for the restart of Prometheus Pods.
