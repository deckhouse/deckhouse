---
title: Single-node cluster
description: A cluster of one node that performs all node roles. It has no fault tolerance and is intended for familiarisation or workloads with low resource requirements.
lang: en
---

After the taint is removed from the node, it runs all cluster components of the selected [module bundle](glossary.html#module-bundle). Such a cluster needs at least 16 CPUs, 32 GB of RAM and 100 GB of disk space on a fast disk (400+ IOPS). Do not use single-node clusters in production environments.

For details, see [Hardware requirements](guides/hardware-requirements.html).
