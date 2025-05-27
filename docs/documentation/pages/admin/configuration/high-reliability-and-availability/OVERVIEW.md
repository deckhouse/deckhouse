---
title: High reliability and availability
permalink: en/admin/configuration/high-reliability-and-availability/
description: High reliability and availability
lang: en
---

A properly configured cluster must be resilient to various situations that may arise during operation,
such as sudden node or component failures, loss of connectivity, and others.
This creates an expectation of stability from the cluster under any circumstances.
Components that may be removed intentionally or accidentally should not cause the entire system to fail,
and availability must not be disrupted.

A cluster managed by Deckhouse Kubernetes Platform (DKP) supports High Reliability and Availability (HA) mode.

In this mode, the overall fault tolerance of the system and cluster reliability increase,
ensuring stable and continuous operation, as well as cluster recovery after failures with minimal delays.

When HA mode is enabled,
the cluster’s critical components are launched with the required redundancy to ensure continuous operation.
If any instance fails, the components continue running, avoiding downtime.

If the cluster has **more than one master node**, HA mode is **enabled automatically**.
This applies both when deploying a cluster with multiple master nodes from the start
and when increasing the number of master nodes from a single one.

In addition to global HA mode,
you can manage this mode [for individual DKP components supporting it](./enable.html#enabling-ha-mode-for-individual-components).
