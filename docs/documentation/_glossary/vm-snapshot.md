---
title: Virtual machine snapshot
description: The captured state of a disk or of a whole virtual machine at a point in time, kept for recovery or cloning. A disk snapshot saves only the data of the selected disk, a VM snapshot includes the VM parameters and the state of all its disks.
lang: en
---

A disk snapshot is described by [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) and serves as a source for new [disks](glossary.html#vm-disk) and [images](glossary.html#vm-image). A virtual machine snapshot is described by [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot) and is the saved state of the machine at a certain moment. Attached images are not saved with it, so they have to be present in the cluster at recovery.

Snapshots are consistent or inconsistent, see [consistent snapshot](glossary.html#vm-consistent-snapshot). A snapshot of a machine can also be the source of a [virtual machine clone](glossary.html#vm-clone).

For details, see [Snapshots of disks and virtual machines](user/virtualization/snapshots.html).
