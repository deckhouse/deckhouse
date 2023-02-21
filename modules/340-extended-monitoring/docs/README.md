---
title: "The extended-monitoring module"
---

Contains the following Prometheus exporters:

- `extended-monitoring-exporter` — implements extended scraping of metrics (free space and inode-related) and [alerts](configuration.html#non-namespaced-kubernetes-objects); also, it enables the "extended monitoring" of objects in the Namespaces with the `extended-monitoring.deckhouse.io/enabled=””` label.
- `image-availability-exporter` — adds metrics (and send alerts) for tracking the availability of the container image specified in the `image` field in the Pod's spec in `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`.
- `events-exporter` — collects Kubernetes cluster events end exposes them as metrics.
- `cert-exporter`— scans Kubernetes Secrets and generates metrics about certificates expiration in them.
