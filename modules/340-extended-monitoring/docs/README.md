---
title: "The extended-monitoring module"
description: "Metrics for extended monitoring of the Deckhouse Kubernetes Platform cluster."
---

The `extended-monitoring` module extends cluster monitoring capabilities with additional Prometheus exporters, which allow you to identify potential problems before they affect the operation of services.

Module features:

- Advanced Metrics Collection — collects additional metrics, and also includes ready-made alerts and dashboards that allow you to detect and diagnose incidents faster:
  - collects and expounds metrics for free space and inodes on nodes, as well as for objects with a label `extended-monitoring.deckhouse.io/enabled =""` in the namespace;
  - automatically generates alerts when the thresholds are reached.
- Container image monitoring:
  - adds metrics and sends alerts about unavailability of container images to registry for all types of workload (`Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`);
  - helps to find out in advance about possible problems with launching or updating pods.
- Cluster Events — collects Kubernetes events and displays them as metrics, which allows you to track the dynamics of changes and respond faster to incidents:
- Certificate control:
  - scans the cluster's Secrets and generates metrics about the expiration of x509 certificates;
  - allows you not to miss critical moments and update certificates on time, avoiding application downtime due to expired certificates.
