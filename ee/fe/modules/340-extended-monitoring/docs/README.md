---
title: "The extended-monitoring module"
---

This module consists of two Prometheus exporters:

- `extended-monitoring-exporter` — generates free space / inode-related metrics and [alerts](configuration.html#non-namespaced-kubernetes-objects); also, it enables the "extended monitoring" of objects in the selected `namespaces`.
- `image-availability-exporter` — generates metrics about issues with accessing Docker images in the registry.
- `events-exporter` — collects Kubernetes cluster events end exposes them as metrics.
