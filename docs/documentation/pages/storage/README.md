---
title: "Overview"
permalink: en/storage/admin/
---

Reliable data storage is one of the key tasks when deploying and operating Kubernetes clusters. In Deckhouse Kubernetes Platform, this is achieved through flexible support for both software-defined and external storage systems, as well as convenient automation and management tools.

In this section, you will learn:
- Which types of storage Deckhouse supports;
- How to set up both local (SDS) and external storage systems;
- Which tools Deckhouse provides to simplify storage configuration and operation;
- How to choose the optimal solution for your scenarios, based on requirements for reliability, performance, and scalability.

## Supported Storage Types

Deckhouse Kubernetes Platform offers a wide range of solutions, which can be divided into two main groups.

### Software-Defined Storage

- [Local storage based on LVM (Logical Volume Manager)](../admin/sds/lvm-local.html);
- [Replicated storage based on DRBD (Distributed Replicated Block Device)](../admin/sds/lvm-replicated.html).

### External Storage

- [Distributed Ceph storage](../admin/external/ceph.html);
- [HPE data storage](../admin/external/hpe.html);
- [Huawei data storage](../admin/external/huawei.html);
- [NFS data storage](../admin/external/nfs.html);
- [S3-based object storage](../admin/external/s3.html);
- [SCSI-based data storage](../admin/external/scsi.html);
- [TATLIN.UNIFIED (Yadro) unified storage](../admin/external/yadro.html).

## Key Features

- All storage configurations are performed through Deckhouse and its modules, simplifying integration with all cluster components;
- Ready-to-use solutions for Ceph, NFS, S3, corporate storage systems, and other options, which make connecting storage to the cluster straightforward;
- Thanks to SDS support (including DRBD) and distributed solutions (Ceph, S3), you can scale up as your workload grows;
- Data replication and integration with robust storage systems help protect critical services and applications from data loss.
