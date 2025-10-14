---
title: "Overview"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/
---

Reliable data storage is one of the key tasks when deploying and operating Kubernetes clusters. In Deckhouse Virtualization Platform (DVP), this is achieved through flexible support for both software-defined and external storage systems, as well as convenient automation and management tools.

In this section, you will learn:

- Which types of storage Deckhouse supports.
- How to set up both local (SDS) and external storage systems.
- Which tools Deckhouse provides to simplify storage configuration and operation.
- How to choose the optimal solution for your scenarios, based on requirements for reliability, performance, and scalability.

## Supported storage types

DVP offers a wide range of solutions, which can be divided into two main groups.

### Software-defined storage

- [Managing logical volumes on cluster nodes](../storage/sds/node-configurator/about.html)
- [Local storage based on LVM (Logical Volume Manager)](../storage/sds/lvm-local.html)
- [Replicated storage based on DRBD (Distributed Replicated Block Device)](../storage/sds/lvm-replicated.html)

### External storage

- [Distributed Ceph storage](../storage/external/ceph.html)
- [HPE data storage](../storage/external/hpe.html)
- [Huawei data storage](../storage/external/huawei.html)
- [NFS data storage](../storage/external/nfs.html)
- [SCSI-based data storage](../storage/external/scsi.html)
- [TATLIN.UNIFIED (Yadro) unified storage](../storage/external/yadro.html)

## Key features

- All storage configurations are performed through DVP and its modules, simplifying integration with all cluster components.
- Ready-to-use solutions for Ceph, NFS, corporate storage systems, and other options, which make connecting storage to the cluster straightforward.
- Thanks to SDS support (including DRBD) and distributed solutions (Ceph), you can scale up as your workload grows.
- Data replication and integration with robust storage systems help protect critical services and applications from data loss.
