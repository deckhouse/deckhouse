---
title: Virtual machine disk
description: Storage of virtual machine data, including the operating system and application files. A disk is described by its storage parameters, the StorageClass and the size, and by a data source, which can be an image, another disk or a snapshot.
lang: en
---

A disk is described by [VirtualDisk](/modules/virtualization/cr.html#virtualdisk). Without a data source an empty disk of the given size is created. With a source, Deckhouse Platform takes the size from the source and picks the storage class by it. Disk behaviour depends on the storage behind the StorageClass: on file system volumes the disk is created in the `qcow2` format, on block devices data is written directly, and the volume binding mode defines whether the disk is created right away or only after it is attached to a [virtual machine](glossary.html#virtual-machine).

For a ready disk only the size and the storage class can be changed. A disk is attached to a machine read-write as a [block device](glossary.html#vm-block-device). Its data can be captured in a [snapshot](glossary.html#vm-snapshot) or written to a file by [disk data export](glossary.html#vm-disk-export).

For details, see [Virtual machine disks](user/virtualization/disks.html).
