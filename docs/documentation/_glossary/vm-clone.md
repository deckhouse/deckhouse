---
title: Virtual machine clone
description: A copy of a virtual machine created from an existing machine or from its snapshot. The clone gets a new IP address and new MAC addresses, and labels are not copied to it.
lang: en
---

Because a clone copies the machine, the resources of the new machine need unique names: `nameReplacements` replaces the names of specific resources, and `customization` adds a prefix or a suffix to the names of all cloned resources, such as [disks](glossary.html#vm-disk) and IP addresses.

The clone gets a new IP address in the cluster network and new MAC addresses for additional interfaces, so the network settings of the guest OS have to be reconfigured. Labels are not copied, so the clone does not receive Service traffic until the needed labels are added. A [snapshot](glossary.html#vm-snapshot) of the machine can be the source of the clone.

For details, see [Cloning virtual machines](user/virtualization/vm-cloning.html).
