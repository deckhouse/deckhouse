---
title: Guest OS agent
description: QEMU Guest Agent installed in the guest system so that Deckhouse Platform can interact with the operating system inside the virtual machine.
lang: en
---

The agent is needed for three things: it makes [consistent snapshots](glossary.html#vm-consistent-snapshot) of disks and machines possible, it reports information about the running system to the `.status.guestOSInfo` block, and it shows that the operating system has actually booted rather than just the [virtual machine](glossary.html#virtual-machine) having started. The `AgentReady` condition of a running machine means that the agent is working.

Deckhouse Platform works with `qemu-guest-agent` version 5.2.0 and later. The agent can be installed and enabled by the [VM provisioning](glossary.html#vm-provisioning) script.

For details, see [Initial provisioning and the guest OS agent](user/virtualization/vm-provisioning.html).
