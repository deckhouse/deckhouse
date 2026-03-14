---
title: Observability subsystem
permalink: en/architecture/observability/
search: observability subsystem
description: Observability subsystem architecture in Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This section describes the architecture of the Observability subsystem of Deckhouse Kubernetes Platform (DKP).

The Observability subsystem includes the following modules:

* [`prometheus`](/modules/prometheus/): Deploys a monitoring stack with predefined settings for DKP and applications, simplifying the initial configuration.
* [`operator-prometheus`](/modules/operator-prometheus/): Installs [Prometheus Operator](https://github.com/coreos/prometheus-operator), which automates the deployment and management of [Prometheus](https://prometheus.io/) instances.
* [`prometheus-metrics-adapter`](/modules/prometheus-metrics-adapter/): Allows HPA and VPA autoscalers to use monitoring metrics when making scaling decisions.
* [`log-shipper`](/modules/log-shipper/): Simplifies log collection setup in Kubernetes clusters.
* [`loki`](/modules/loki/): Deploys a short-term log storage system in the cluster based on [Grafana Loki](https://grafana.com/oss/loki/).
* [`observability`](/modules/observability/): Extends the functionality of the [`prometheus`](/modules/prometheus/) and [`console`](/modules/console/stable/) modules, providing additional capabilities for flexible metric visualization and access control.
* [`extended-monitoring`](/modules/extended-monitoring/): Enhances cluster monitoring by deploying additional Prometheus exporters that help detect potential issues before they affect service operation.
* [`monitoring-custom`](/modules/monitoring-custom/): Simplifies monitoring configuration for user applications by requiring only a specific label to be set for the target application.
* [`monitoring-deckhouse`](/modules/monitoring-deckhouse/): Provides monitoring of DKP components and services.
* [`monitoring-kubernetes`](/modules/monitoring-kubernetes/): Provides transparent and timely monitoring of all cluster nodes and key infrastructure components.
* [`monitoring-kubernetes-control-plane`](/modules/monitoring-kubernetes-control-plane/): Organizes secure metrics collection and provides a basic set of monitoring rules for cluster control plane components.
* [`upmeter`](/modules/upmeter/): Checks platform availability and cluster component health in real time and displays the results on dedicated dashboards.

The following components are currently described in this section:

* [Monitoring architecture in DKP](monitoring.html)
* [Logging modules](logging.html)
