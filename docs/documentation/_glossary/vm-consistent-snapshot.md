---
title: Consistent snapshot
description: A snapshot of a virtual machine disk that captures a coherent and integral state of its data. It is created when the disk is not attached to any machine, the machine is powered off, or the guest OS agent freezes the file system while the snapshot is taken.
lang: en
---

Whether a consistent [snapshot](glossary.html#vm-snapshot) is required is set by the `requiredConsistency` parameter, which is `true` by default. With a running machine the file system is frozen by the [guest OS agent](glossary.html#vm-guest-agent). Its hook scripts can also prepare applications for the snapshot without stopping services.

An inconsistent snapshot is created when the machine is running and the agent is not installed or not running. Recovery from such a snapshot can lead to file system integrity problems.

For details, see [Snapshots of disks and virtual machines](user/virtualization/snapshots.html).
