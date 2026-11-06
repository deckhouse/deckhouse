---
title: Virtual machine image
description: The contents of a disk from which virtual machine disks are created. A project image is available only in the project or namespace where it was created, and a virtual machine accesses an attached image read-only.
lang: en
---

An image is described by [VirtualImage](/modules/virtualization/cr.html#virtualimage). Its source can be an HTTP server with the image file, a container image registry, a file uploaded from the command line, another image, a [virtual machine disk](glossary.html#vm-disk) or a [disk snapshot](glossary.html#vm-snapshot). The downloaded image is stored either in [DVCR](glossary.html#dvcr) or in a PVC. An image stored in a PVC can be used to create disks only in the same storage class.

To make an image available to every [project](glossary.html#project) of the cluster, an administrator creates a cluster image, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage). Images are attached to a [virtual machine](glossary.html#virtual-machine) as [block devices](glossary.html#vm-block-device): read-only or as a CD-ROM for ISO images.

For details, see [Virtual machine images](user/virtualization/images.html).
