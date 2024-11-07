---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/admin/install/scenarios.html
---

## Typical Configuration

In a typical high availability (HA) mode configuration, it is recommended to use 3 master nodes to ensure stability and plan for "n+1" worker nodes depending on your workload. Each virtual machine should be located on a separate physical machine.

*Note:* ControlPlane in the standard setup is not used for running virtual machines.