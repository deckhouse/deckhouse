---
title: Virtual machine (VM)
description: A machine that Deckhouse Platform creates and manages declaratively through the virtualization module. It runs in a pod on a cluster node under QEMU/KVM, managed by KubeVirt.
lang: en
---

A machine is described by [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) in a [project](glossary.html#project) or namespace. Its [disks](glossary.html#vm-disk) and [images](glossary.html#vm-image) are attached as [block devices](glossary.html#vm-block-device), its CPU is set by the number of cores and the [core fraction](glossary.html#vm-core-fraction) within the limits of the VM class, and its state is kept by the [run policy](glossary.html#vm-run-policy). The guest system is configured on the first boot by [provisioning](glossary.html#vm-provisioning), after which Deckhouse Platform communicates with it through the [guest OS agent](glossary.html#vm-guest-agent).

The module core is based on KubeVirt: the QEMU process started by libvirtd in the `compute` container of the machine pod is the running machine. Machines move between nodes by [live migration](glossary.html#live-migration), are grouped into [pools](glossary.html#vm-pool), and go through the phases of the [virtual machine lifecycle](glossary.html#vm-lifecycle).

For details, see [Virtualization subsystem](architecture/virtualization/).
