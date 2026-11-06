---
title: Virtual machine block device
description: A disk or an image attached to a virtual machine. Images are attached read-only or as a CD-ROM for ISO images, disks are attached read-write.
lang: en
---

Devices are attached in two ways. The `blockDeviceRefs` list of the [virtual machine](glossary.html#virtual-machine) specification sets the boot order by position or by the `bootOrder` field and is used for manual configuration, for example an ISO for OS installation. A [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) attaches a [disk](glossary.html#vm-disk) as a separate resource that does not take part in the boot order, over the `virtio-scsi` bus, and is used for automation and when the user has no rights to edit the machine.

With paravirtualization enabled, both ways attach and detach disks on a running machine without a reboot, if the storage is available on the node where the machine runs. While a [live migration](glossary.html#live-migration) prepares the target node, devices can be neither attached nor detached.

For details, see [Attaching disks and images to a virtual machine](user/virtualization/vm-block-devices.html).
