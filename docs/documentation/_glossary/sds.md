---
title: Software-defined storage
description: A group of storage types in Deckhouse Platform that run on the disks of the cluster nodes and are managed by the sds-* modules, as opposed to external storage systems.
lang: en
---

Software-defined storage (SDS) includes managing logical volumes on cluster nodes, [local storage](glossary.html#local-storage) based on Local Path Provisioner or LVM, and [replicated storage](glossary.html#replicated-storage) based on DRBD. The other group is external storage: Ceph, NFS, S3 and storage systems of hardware vendors.

For details, see [Storage](admin/configuration/storage/).
