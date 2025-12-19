---
title: "Configuring application monitoring"
permalink: en/user/monitoring/app.html
---

Deckhouse Kubernetes Platform (DKP) supports four ways to connect an application to the monitoring system:

| Connection method | Description |
| ------------------ | -------- |
| [Via labels and annotations](#configuring-metrics-collection-via-labels-and-annotations) | The simplest and fastest method, requiring only metadata to be added to a Service or Pod. Allows you to configure basic monitoring parameters. |
| [Using PodMonitor or ServiceMonitor](#configuring-metrics-collection-using-podmonitor-or-servicemonitor-resources) | An advanced monitoring configuration method for cases where Prometheus relabeling rules are required. Provides flexible control over metrics collection and label processing. This approach is suitable for complex monitoring scenarios but requires a deeper understanding of Prometheus and its scraping mechanism. |
| [Using ScrapeConfig](#configuring-metrics-collection-via-scrape_configs-using-the-scrapeconfig-resource) | A monitoring configuration method that is as close as possible to the native Prometheus configuration structure. Provides full control over scrape settings, including relabeling, and allows collecting metrics both from Kubernetes and from targets located outside the cluster. |
| [Availability monitoring using blackbox-exporter](#configuring-metrics-collection-using-blackbox-exporter) | A method for monitoring endpoint availability using probes. It is integrated using [blackbox-exporter](https://github.com/prometheus/blackbox_exporter/), which must be installed in the cluster separately. |

## Configuring metrics collection via labels and annotations

{% alert level="info" %}
This section describes a basic application integration scenario.
For advanced configuration options, refer to [additional annotations](#additional-annotations-for-advanced-configuration).
{% endalert %}

1. Make sure the [`monitoring-custom`](/modules/monitoring-custom/) module is enabled.
   If necessary, contact your DKP administrator.

1. Ensure that the application exposing metrics does so in the [Prometheus format](https://prometheus.io/docs/instrumenting/exposition_formats/).

1. Add the `prometheus.deckhouse.io/custom-target` label to the Service or Pod that should be connected to monitoring.
   The label value defines the name in the Prometheus targets list.

   Example:

   ```yaml
   labels:
     prometheus.deckhouse.io/custom-target: my-app
   ```

   It is recommended to use the application name as the value of the `prometheus.deckhouse.io/custom-target` label,
   which allows it to be uniquely identified in the cluster.

   The label format must comply with [Kubernetes requirements](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/):
   no more than 63 characters, which can include alphanumeric characters (`[a-z0-9A-Z]`),
   as well as hyphens (`-`), underscores (`_`), dots (`.`).

   If the application is deployed in the cluster more than once (staging, testing, etc.)
   or even deployed several times in one namespace, one common name is sufficient,
   since all metrics will have `namespace`, `pod` labels anyway, and if access is through Service, the `service` label.
   This is the name that uniquely identifies the application in the cluster, not its single installation.

1. Specify the name `http-metrics` and `https-metrics` for the port from which metrics need to be collected
   for HTTP or HTTPS connection respectively.

   If this is not possible (for example, the port is already defined and named differently), use the following annotations:

   - `prometheus.deckhouse.io/port: port_number`: To specify the port.
   - `prometheus.deckhouse.io/tls: "true"`: If metrics collection will be over HTTPS.

   > When specifying an annotation on a Service, you must use `targetPort` as the port value.
   > That is, the port that is open and listened to by the application, not the Service port.

   - Example 1:

     ```yaml
     ports:
     - name: https-metrics
       containerPort: 443
     ```

   - Example 2:

     ```yaml
     annotations:
       prometheus.deckhouse.io/port: "443"
       prometheus.deckhouse.io/tls: "true"  # If metrics are provided over HTTP, do not specify this annotation.
     ```

1. When using service mesh [Istio](../../admin/configuration/network/internal/encrypting-pods.html) in STRICT mTLS mode,
   specify the following annotation for metrics collection on Service or Pod: `prometheus.deckhouse.io/istio-mtls: "true"`.
   It is important that application metrics should be exported over HTTP protocol without TLS.

   Example:

   ```yaml
   annotations:
     prometheus.deckhouse.io/istio-mtls: "true"
   ```

### Example of configuring metrics collection from a Service

Below is an example of setting up metrics collection from a Service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
  annotations:
    prometheus.deckhouse.io/port: "8061"                      # By default, the service port with the name http-metrics or https-metrics will be used.
    prometheus.deckhouse.io/path: "/my_app/metrics"           # Set to /metrics by default.
    prometheus.deckhouse.io/query-param-format: "prometheus"  # Set to '' by default.
    prometheus.deckhouse.io/allow-unready-pod: "true"         # By default, Pods NOT in Ready state are ignored.
    prometheus.deckhouse.io/sample-limit: "5000"              # By default, no more than 5000 metrics are accepted from a single Pod.
spec:
  ports:
  - name: my-app
    port: 8060
  - name: http-metrics
    port: 8061
    targetPort: 8061
  selector:
    app: my-app
```

### Example of configuring metrics collection from a Deployment

Below is an example of setting up metrics collection from a Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
        prometheus.deckhouse.io/custom-target: my-app
      annotations:
        prometheus.deckhouse.io/sample-limit: "5000"  # By default, no more than 5000 metrics are accepted from a single Pod.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```

### Additional annotations for advanced configuration

For more precise application monitoring configuration,
you can specify additional annotations for the Pod or Service for which monitoring is configured:

- `prometheus.deckhouse.io/path`: Path for metrics collection (default: `/metrics`).
- `prometheus.deckhouse.io/query-param-$name`: GET parameters that will be converted to a map of the form `$name=$value` (default: '').
  You can specify several such annotations.
  For example, `prometheus.deckhouse.io/query-param-foo=bar` and `prometheus.deckhouse.io/query-param-bar=zxc` will be converted to a request like `http://...?foo=bar&bar=zxc`.
- `prometheus.deckhouse.io/allow-unready-pod`: Allows metrics collection from pods in any state
   (by default, metrics are collected only from pods in Ready state). This option is useful in rare cases.
   For example, if your application starts very slowly (data is loaded into the database or caches are warmed up at startup),
   but useful metrics are already provided during startup that help monitor the application startup.
- `prometheus.deckhouse.io/sample-limit`: How many samples are allowed to be collected from a pod (`5000` by default).
  The default value protects against situations where the application suddenly starts providing too many metrics,
  which can disrupt the entire monitoring system.
  The annotation must be placed on the same resource where the `prometheus.deckhouse.io/custom-target` label is attached.

## Configuring metrics collection using PodMonitor or ServiceMonitor resources

DKP supports connecting applications using two functionally similar resources:

- [PodMonitor](/modules/operator-prometheus/cr.html#podmonitor) (recommended): Discovers Pods directly
  and collects metrics from their containers. In most cases, this is the preferred option,
  as it works directly with Pods and does not depend on the presence of Services.
- [ServiceMonitor](/modules/operator-prometheus/cr.html#servicemonitor): Discovers Services
  and collects metrics from the Pods behind them. Services are used as a source of metadata (such as labels),
  while the actual metrics scraping is performed against the Pod addresses included in the corresponding endpoints.

Both resources let you configure the scrape interval, paths, TLS settings, relabeling rules, and other parameters.

The difference between these resources lies in the source of the collected metrics.
Use PodMonitor if you need to scrape metrics directly from Pods,
and ServiceMonitor if your application exposes metrics via a Service.

To connect an application to the monitoring system using one of these resources, follow these steps:

1. Add the `prometheus.deckhouse.io/monitor-watcher-enabled: "true"` label to the namespace where the PodMonitor
   or ServiceMonitor will be created:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: frontend
     labels:
       prometheus.deckhouse.io/monitor-watcher-enabled: "true"
   ```

1. Create a PodMonitor or ServiceMonitor resource,
   specifying the required `prometheus: main` label and the target endpoint parameters.

   PodMonitor example:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: example-app
     namespace: frontend
     labels:
       prometheus: main
   spec:
     selector:
       matchLabels:
         app: example-app
     podMetricsEndpoints:
       - port: metrics
         interval: 30s
         path: /metrics
   ```

   ServiceMonitor example:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: example-app
     namespace: frontend
     labels:
       prometheus: main
   spec:
     selector:
       matchLabels:
         app: example-app
     endpoints:
       - port: web
         interval: 30s
         path: /metrics
   ```

   If necessary, configure additional settings using the reference for available resource parameters:
   [PodMonitor](/modules/operator-prometheus/cr.html#podmonitor), [ServiceMonitor](/modules/operator-prometheus/cr.html#servicemonitor).

## Configuring metrics collection via scrape_configs using the ScrapeConfig resource

[ScrapeConfig](/modules/operator-prometheus/cr.html#scrapeconfig) is a custom resource
that lets you configure the `scrape_config` section of Prometheus configuration,
providing full control over the metrics scraping process.

To connect an application to the monitoring system, follow these steps:

1. Add the `prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"` label to the namespace
   where the ScrapeConfig will be created:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: frontend
     labels:
       prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"
   ```
  
1. Create a [ScrapeConfig](/modules/operator-prometheus/cr.html#scrapeconfig) resource
   with the required `prometheus: main` label:

   ```yaml
   apiVersion: monitoring.coreos.com/v1alpha1
   kind: ScrapeConfig
   metadata:
     name: example-scrape-config
     namespace: frontend
     labels:
       prometheus: main
   spec:
     honorLabels: true
     staticConfigs:
       - targets: ['example-app.frontend.svc.{{ .Values.global.discovery.clusterDomain }}.:8080']
     relabelings:
       - regex: endpoint|namespace|pod|service
         action: labeldrop
       - targetLabel: scrape_endpoint
         replacement: main
       - targetLabel: job
         replacement: kube-state-metrics
     metricsPath: '/metrics'
   ```

   If necessary, configure additional settings using the [reference](/modules/operator-prometheus/cr.html#scrapeconfig)
   for available resource parameters.

## Configuring metrics collection using blackbox-exporter

DKP supports availability metrics collection using [blackbox-exporter](https://github.com/prometheus/blackbox_exporter/),
which is not included in DKP and must be installed separately in the cluster.
The [Probe](/modules/operator-prometheus/cr.html#probe) custom resource is used to define availability checks (probes)
executed by Prometheus.

To connect a Probe to the DKP monitoring system, follow these steps:

1. Add the `prometheus.deckhouse.io/probe-watcher-enabled: "true"` label to the namespace where the Probe will be created:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: frontend
     labels:
       prometheus.deckhouse.io/probe-watcher-enabled: "true"
   ```

1. Create a [Probe](/modules/operator-prometheus/cr.html#probe) resource with the required `prometheus: main` label:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: Probe
   metadata:
     labels:
       app: prometheus
       component: probes
       prometheus: main
     name: cdn-is-up
     namespace: frontend
   spec:
     interval: 30s
     jobName: httpGet
     module: http_2xx
     prober:
       path: /probe
       scheme: http
       url: blackbox-exporter.blackbox-exporter.svc.cluster.local:9115
     targets:
       staticConfig:
         static:
         - https://example.com/status
   ```

   If necessary, configure additional settings using the [reference](/modules/operator-prometheus/cr.html#probe)
   for available resource parameters.
