---
title: Runtime profile
description: A behaviour model of an application or container at runtime, including system calls, executed processes, file access and network interactions, built by the node-agent of the [`operator-trivy`](/modules/operator-trivy/) module with eBPF programs.
lang: en
---

The node-agent is an optional component that runs on all cluster nodes in privileged mode, observes container behaviour with eBPF programs and stores the profiles in the ApplicationProfile and NetworkNeighborhood objects. It works in profiling mode and does not detect attacks by rules. That is the task of the [runtime audit](glossary.html#runtime-audit). The profiles can be used to generate network policies from the observed traffic.

For details, see [Module operator-trivy](architecture/security/operator-trivy.html).
