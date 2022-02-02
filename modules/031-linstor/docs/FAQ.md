---
title: "The linstor module: FAQ"
---

## What is difference between LVM and LVMThin?

Briefly:
- LVM is simpler and has performance comparable to native drives.
- LVMThin allows you to use snapshots and overprovisioning, but twice as slow.

for more details read the next section

## What is actual perfomance and reliability of LINSTOR?

We take a practical view of the issue. A difference of several tens of percent - in practice it never matters. The difference is several times or more important. 
- Sequential read and write do not matter, because on any technology they always run into the network (which is 10Gb/s, which is 1Gb/s). From a practical point of view, this indicator can be completely ignored.
- Random read and write (which is 1Gb/s, which is 10Gb/s) 
  - drbd+lvm 5 times better (latency - 5 times less, iops - 5 times more) than ceph-rbd 
  - drbd+lvm is 2 times better than drbd+lvmthin 
- if one of the replicas is local
  - read - at device speed 
- if there are no local replicas
  - write - limited to half the network bandwidth (with two replicas, or ⅓ network bandwidth with three replicas) 
- with a large number of clients (more than 10, with iodepth 64), ceph starts to fall behind more (up to 10 times) and consume much more CPU.

The dry result - in practice, it doesn’t matter which twists to twist, only three factors affect: 
- **read locality** - if all reading is performed locally, then it works at the speed (throughput, iops, latency) of the local disk (the difference is practically insignificant)
- **1 network hop when writing** - in drbd, the replication is performed by the “client”, and in ceph, by “server”, so ceph latency for writing always has at least x2 from drbd
- **complexity of code** - latency of calculations on the datapath (how much assembler code is executed for each io operation), drbd+lvm is simpler than drbd+lvmthin, and much simpler than ceph-rbd. 

## What to use in which situation? 

- By default, we use 2 replicas (the third one is diskless used for quorum, it is created automatically), this guarantees protection against split brain and a sufficient level of storage reliability.
    - You need to understand that when one of the replicas (replica A) is unavailable, data is written only to a single replica (replica B). It means that:
       1. if at this moment the second replica (replica B) is also turned off, writing and reading will be unavailable,
       1. if at the same time the second replica (replica B) is irretrievably lost, then the data will be partially lost (there is only the old, outdated replica A),
       1. if the old replica (replica A) was also irretrievably lost, the data will be completely lost.
    - In addition, when the second replica is turned off, in order to turn it back on (without operator intervention), both replicas must be available (in order to correctly work out the split-brain).
    - Enabling a third replica solves both problems (at least 2 copies of data at any given time), but increases the overhead (network, disk). 

- It is strongly recommended to have one replica locally. This is x2 write bandwidth (with 2 replicas) and free reading.
     - But if this is not the case, then everything still continues to work normally (but reading over the network, and x2 network utilization for writing).
   - Depending on the task, choose one of the following:
     - drbd+lvm - faster (x2) and more reliable (lvm is simpler),
     - drbd+lvmthin - support for snapshots and the possibility of overcommitment

## How to add existing LVM or LVMThin pool?

Very simple:

- LVM:
  ```
  linstor storage-pool create lvm node01 lvmthin linstor_data
  ```

- LVMThin:
  ```
  linstor storage-pool create lvmthin node01 lvmthin linstor_data/data
  ```

You can also add pools with some volumes have already been created. LINSTOR will just create new ones nearby. 

## How to configure Prometheus to use LINSTOR for storing data?

Configure storage-pools and StorageClass, as shown in [Usage](usage.html)

Add to the Deckhouse configuration (configMap `d8-system/deckhouse`):
```yaml
prometheus: |
  longtermStorageClass: linstor-r2
  storageClass: linstor-r2
```

Wait for the restart of Prometheus Pods.
