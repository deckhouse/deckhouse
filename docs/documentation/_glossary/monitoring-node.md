---
title: Monitoring node
description: A dedicated node for Grafana, Prometheus and other monitoring components. Without such nodes the monitoring components run on system nodes.
lang: en
---

Dedicated monitoring nodes are recommended in heavily loaded clusters with many [alerts](glossary.html#alert) and large volumes of metrics. They need fast disks, at least 400 IOPS. Monitoring nodes usually form the `monitoring` [node group](glossary.html#node-group).

For details, see [Recommendations for configuring cluster nodes](admin/configuration/high-reliability-and-availability/recommendations.html).
