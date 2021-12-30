---
title: "The extended-monitoring module"
---

Contains the following Prometheus exporters:

- `extended-monitoring-exporter` — generates free space / inode-related metrics and [alerts](configuration.html#non-namespaced-kubernetes-objects); also, it enables the "extended monitoring" of objects in the selected Namespaces.
- `image-availability-exporter` — generates metrics about issues with accessing images in the container registry.
- `events-exporter` — collects Kubernetes cluster events end exposes them as metrics.
- `cert-exporter`— scans Kubernetes Secrets and generates metrics about certificates expiration in them.
