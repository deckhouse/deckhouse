---
title: "Setup scenarios"
permalink: en/virtualization-platform/documentation/admin/install/scenarios.html
---

In a typical configuration, it is recommended to use 3 master nodes to ensure stability and any number of worker nodes to run virtual machines.

To calculate the number of worker nodes, use the formula: N + 1, where N is the desired number of virtual machines to run divided by 10.
An additional worker node is required to migrate virtual machines in case of failure or scheduled maintenance.
