---
title: "Overview"
permalink: en/user/monitoring/
---

Deckhouse Kubernetes Platform (DKP) provides a convenient and ready-to-use Kubernetes cluster monitoring system.

By default, monitoring collects a large number of metrics and contains configured triggers for tracking the general state of user applications, as well as provides access to them in the form of convenient dashboards in the Grafana web interface.

You only need to enable the [`monitoring-custom`](/modules/monitoring-custom/) module, add the `prometheus.deckhouse.io/custom-target` label to a Service or Pod, and specify the port (for example, `http-metrics`). After that, the metrics will start being collected without any manual Prometheus configuration.

You can also configure the collection of custom metrics from applications deployed in the cluster.
The system supports flexible configuration options, including HTTPS, custom paths, query parameters, integration with Istio (mTLS), and overload protection (metric limits).
This enables seamless integration of applications into the cluster-wide monitoring system, allowing you to track their health and performance.

Key features:

- **Ready-made dashboards** in Grafana with graphs for CPU, memory, disk and network load: Can be viewed by pods, nodes or namespaces.
- **Useful notifications** in Slack/Telegram/email about problems: Service unavailability, disk space shortage, approaching certificate expiration.
- **Simple integration**: To start monitoring your application, it is enough to add a couple of annotations to Pod or Service.

## Extended monitoring mode

DKP supports an extended monitoring mode via the [`extended-monitoring`](/modules/extended-monitoring/) module, allowing you to configure:

- Monitoring secrets in the cluster (Secret objects) and TLS certificate expiration in them.
- Collecting Kubernetes cluster events as metrics.
- Monitoring container image availability in registry used by controllers (Deployments, StatefulSets, DaemonSets, CronJobs).
- Monitoring objects in namespaces that have the `extended-monitoring.deckhouse.io/enabled=""` label.

The module can send alerts based on the following metrics:

- Free space and inodes on node disks
- Node utilization
- Pod and container image availability
- Certificate expiration
- Other cluster events
