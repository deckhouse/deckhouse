---
title: VM provisioning
description: Initial configuration of the guest system of a virtual machine on its first boot by an initialization script, Cloud-Init for Linux or Sysprep for Windows.
lang: en
---

The initialization script creates users, sets passwords and SSH keys, installs software, runs commands and enables services, for example the [guest OS agent](glossary.html#vm-guest-agent). The Cloud-Init configuration is written in YAML and starts with the `#cloud-config` header.

Replicas of a [virtual machine pool](glossary.html#vm-pool) are differentiated by provisioning, so operating systems without it, the `Legacy` OS type, are not supported in pools.

For details, see [Initial provisioning and the guest OS agent](user/virtualization/vm-provisioning.html).
