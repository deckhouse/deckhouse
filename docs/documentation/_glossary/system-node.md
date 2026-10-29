---
title: System node
description: A node dedicated to running Deckhouse Platform modules so that they do not interfere with user applications.
lang: en
---

System nodes are isolated with labels and taints so that user pods are not scheduled on them. Two system nodes are recommended. System nodes can be [static](glossary.html#static-node) or cloud nodes and usually form the `system` [node group](glossary.html#node-group). Without dedicated [monitoring nodes](glossary.html#monitoring-node), the monitoring components also run on system nodes.

For details, see [Recommendations for configuring cluster nodes](admin/configuration/high-reliability-and-availability/recommendations.html).
