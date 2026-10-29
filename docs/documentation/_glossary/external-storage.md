---
title: External storage
description: Storage systems outside the cluster nodes, Ceph, NFS, S3, SCSI and vendor storage systems, connected to Deckhouse Platform through csi-* modules, as opposed to software-defined storage.
lang: en
---

Deckhouse Platform ships ready-to-use integrations with distributed Ceph storage, HPE, Huawei and TATLIN.UNIFIED (Yadro) storage systems, NFS, S3 object storage and SCSI-based storage. Each integration is a module with a [CSI driver](glossary.html#csi-driver). The second group of storage types is [software-defined storage](glossary.html#sds) on the disks of cluster nodes.

For details, see [Storage](admin/configuration/storage/).
