---
title: Observability module
permalink: en/architecture/observability/observability.html
search: observability, grafana, alertmanager, dashboard
description: Architecture of the observability module in Deckhouse Kubernetes Platform.
---

The `observability` module extends the functionality of the [`prometheus`](/modules/prometheus/) module and the [Deckhouse web UI](/modules/console/), providing additional capabilities for flexible management of metrics, dashboards, and alerts, as well as access control mechanisms for them.

Module capabilities:

* **Dashboard management**: Allows users to add their own dashboards in Grafana format.
* **Trigger and metric rule group management**: Allows creating and configuring custom trigger and metric rule groups.
* **Notification configuration**: Allows configuring notification channels (Telegram, Slack, email, and webhooks), notification policies, and disabling notifications when needed.
* **Alert management**: Provides information about active alerts and stores the history of resolved alerts.
* **Standard data sources provisioning**.
* **Custom data sources support**: The module allows you to add custom data sources in addition to the provided set of standard data sources.

For more details about the module, refer to [the module documentation](/modules/observability/) section.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`observability`](/modules/observability/) module and its interactions with other components of DKP are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Observability module architecture](../../images/architecture/observability/c4-l2-observability.svg)

## Module components

The module consists of the following components:

1. **Observability-controller**: Consists of a single **observability-controller** container and manages the lifecycle of most of the module's custom resources, such as ObservabilityMetricsRulesGroup, ObservabilityNotificationChannels, ObservabilityNotificationSilence, etc. For complete list of resources managed by the module, refer to [the `Custom resourses` section in the module documentation](/modules/observability/cr.html).

1. **Observability-webhook**: Consists of a single **observability-webhook** container that implements a webhook server, which is used to validate and mutate Kubernetes API resources through the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanisms.

1. **Alert-kube-api**: Consists of a single **alert-kube-api** container and implements [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), that extends Kubernetes API with ObservabilityAlert и ClusterObservabilityAlert custom resources. Alert-kube-api allows you to request alerts as custom resources, using the Alertmanager component as a backend, and caches them in memory for quick access.

1. **Alertmanager**: Receives alerts from prometheus-main component of the [`prometheus`](/modules/prometheus/) module, processes and sends them to the end recipients. DKP supports sending alerts through the following delivery channels:

   * `Email`
   * `Telegram`
   * `Slack`
   * `Webhook`

   It consists of the following containers:

   * **alertmanager**: Main container. A fork of [original Alertmanager](https://github.com/prometheus/alertmanager) from the company "Flant" is used in the [`observability`](/modules/observability/) module, that supports [multitenancy](../iam/multitenancy.html): the separation of alerts by system (cluster) and project, the delivery of alerts (policies, channels, silencers) is also multi-tenant, i.e. different channels and delivery policies can be configured in different projects.

   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to the Alertmanager API endpoint. It is an [open-source project](https://github.com/brancz/kube-rbac-proxy).

1. **Grafana**: A component that provides a web interface for visualizing monitoring data. A [fork](https://github.com/okmeter/grafana) of [Grafana](https://github.com/grafana/grafana) from the company "Flant" is used in the [`observability`](/modules/observability/) module. The Grafana modification used has advanced features, such as separate access to metrics and dashboards according to [multitenancy](../iam/multitenancy.html). Grafana dashboards of the [`observability`](/modules/observability/) module are integrated into the [Deckhouse web UI](/modules/console/) (monitoring system management from one window).

   It consists of the following containers:

   * **grafana**: Main container.
   * **grafana-kube-storage**: Sidecar container that implements a backend for grafana container and providing management of Dashboard resources and reading Datasource resources of grafana component API. These resources allow you to view and manage dashboards within namespaces (projects), as well as to connect custom data sources.

   * **nginx**: Sidecar container that is NGINX proxy server, which is used to publish static files. It is an [open-source project](https://github.com/nginx/nginx).

1. **Label-enforcer**: A component that authorizes and proxyes user requests to metrics (prometheus-main via the label-proxy service) and logs (loki via the logs-gateway service) sources, specified in the Datasource resources of grafana component API. Label-enforcer verifies RBAC access to monitoring data based on user rights, retrieves a list of available namespaces, and enriches requests with labels to filter requested data within user namespaces. For more information about access control, refer to [the module documentation](/modules/observability/metrics.html) section. Label-enforcer handles not only read requests, but also write requests.

   It consists of a single container:

   **enforcer**.

1. **Opagent** (DaemonSet): An agent designed to collect metrics from both the operating system and the application software installed on the servers. opAgent is developed by Flant for the [Deckhouse Observability Platform (DOP)](/products/observability-platform/) based on [Okagent (Okmeter agent)](https://okmeter.ru/docs/features/), that is also a part of the [Okmeter](https://okmeter.ru/docs/overview/) monitoring system.

   In the [`observability`](/modules/observability/) module opAgent connects to managed services, for example [`managed-postgres`](/modules/managed-postgres/), [`managed-memcached`](/modules/managed-memcached/), [`managed-kafka`](/modules/managed-kafka/), etc. (the list of supported managed services is constantly expanding), collects metrics from them, and sends them to prometheus-main component of the [`prometheus`](/modules/prometheus/) module. If the [`observability-platform`](/modules/observability-platform/) module is enabled, opAgent collects metrics from the cluster nodes and sends them to the [DOP](/products/observability-platform/).

   opAgent sends the collected metrics using the [Prometheus Remote Write](https://prometheus.io/docs/specs/prw/remote_write_spec/) protocol to prometheus-main via label-enforcer (managed services metrics) and to the [DOP](/products/observability-platform/) (metrics from cluster nodes).

   It consists of a single container:

   **opagent**.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Authorizes requests for monitoring data.
   * Manages module custom resources.

1. **Prometheus**: Uses it as a data source and destination.
1. **Loki**: Uses it as a data source.
1. **Alerts recievers**: Sends alers.
1. [DOP](/products/observability-platform/): Uses it as a data destination (metrics from cluster nodes).

The following external components interact with the module:

1. **Kube-apiserver**:

   * Validates and mutates module custom resources (with validating and mutating webhooks).
   * Forwards to alert-kube-api requests for ObservabilityAlert and ClusterObservabilityAlert custom resources.

1. **Prometheus-main**: Sends alerts to alertmanager.
1. **[Deckhouse web UI](/modules/console/)**: Uses grafana for visualizing monitoring data.
