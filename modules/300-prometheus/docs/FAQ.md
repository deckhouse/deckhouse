---
title: "The Prometheus monitoring module: FAQ"
type:
  - instruction
search: prometheus monitoring, prometheus custom alert, prometheus custom alerting
---


## How do I collect metrics from applications running outside of the cluster?

1. Configure a Service similar to the one that [collects metrics from your application](../../modules/340-monitoring-custom/#an-example-service) (but do not set the `spec.selector` parameter).
1. Create Endpoints for this Service and explicitly specify the `IP:PORT` pairs that your applications use to expose metrics.
> Note that port names in Endpoints must match those in the Service.

### An example:
Application metrics are freely available (no TLS involved) at `http://10.182.10.5:9114/metrics`.
```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
spec:
  ports:
  - name: http-metrics
    port: 9114
---
apiVersion: v1
kind: Endpoints
metadata:
  name: my-app
  namespace: my-namespace
subsets:
  - addresses:
    - ip: 10.182.10.5
    ports:
    - name: http-metrics
      port: 9114
```

## How do I create custom Grafana dashboards?

The custom Grafana dashboards can be added to the project using the infrastructure as a code approach.
To add your dashboard to Grafana, create the dedicated [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition) custom resource in the cluster.

An example:
```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # The folder where the custom dashboard will be located
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```
**Caution!** System dashboards and dashboards added using [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) cannot be modified via the Grafana interface.

## How do I add alerts and/or recording rules?

The `CustomPrometheusRules` resource allows you to add alerts.

Parameters:

`groups` — is the only parameter where you need to define alert groups. The structure of the groups is similar to [that of prometheus-operator](https://github.com/coreos/prometheus-operator/blob/ed9e365370603345ec985b8bfb8b65c242262497/Documentation/api.md#rulegroup).

An example:
```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: my-rules
spec:
  groups:
  - name: cluster-state-alert.rules
    rules:
    - alert: CephClusterErrorState
      annotations:
        description: Storage cluster is in error state for more than 10m.
        summary: Storage cluster is in error state
        plk_markup_format: markdown
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```
### How do I provision additional Grafana Datasources?
The `GrafanaAdditionalDatasource` allows you to provision additional Grafana Datasources.

A detailed description of the resource parameters is available in the [Grafana documentation](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file).

An example:
```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: another-prometheus
spec:
  type: prometheus
  access: Proxy
  url: https://another-prometheus.example.com/prometheus
  basicAuth: true
  basicAuthUser: foo
  jsonData:
    timeInterval: 30s
    httpMethod: POST
  secureJsonData:
    basicAuthPassword: bar
```

## How do I enable secure access to metrics?
To enable secure access to metrics, we strongly recommend using **kube-rbac-proxy**.

## How do I add an additional alertmanager?

Create a service with the `prometheus.deckhouse.io/alertmanager: main` that points to your Alertmanager.

Optional annotations:
* `prometheus.deckhouse.io/alertmanager-path-prefix` — the prefix to add to HTTP requests;
  * It is set to "/" by default.

**Caution!** Currently, only the plain HTTP scheme is supported.

An example:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-alertmanager
  namespace: my-monitoring
  labels:
    prometheus.deckhouse.io/alertmanager: main
  annotations:
    prometheus.deckhouse.io/alertmanager-path-prefix: /myprefix/
spec:
  type: ClusterIP
  clusterIP: None
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  selector:
    app: my-alertmanager
```
**Caution!!**  If you create Endpoint for a Service manually (e.g., to use an external alertmanager), you must specify the port name both in the Service and in Endpoints.

## How do I ignore unnecessary alerts in alertmanager?

The solution comes down to configuring alert routing in the Alertmanager.

You will need to: 
1. Create a parameterless receiver.
1. Route unwanted alerts to this receiver. 

Below is the sample `alertmanager.yaml` for this kind of a situation:
```yaml
receivers:
- name: blackhole
  # the parameterless receiver is similar to "/dev/null".
- name: some-other-receiver
  # ...
route:
  routes:
  - match:
      alertname: DeadMansSwitch
    receiver: blackhole
  - match_re:
      service: ^(foo1|foo2|baz)$
    receiver: blackhole
  - receiver: some-other-receiver
```

A detailed description of all parameters can be found in the [official documentation](https://prometheus.io/docs/alerting/latest/configuration/#configuration-file).

## Why can't different scrape Intervals be set for individual targets?

The Prometheus developer Brian Brazil provides, probably, the most [comprehensive answer](https://www.robustperception.io/keep-it-simple-scrape_interval-id) to this question.
In short, different scrapeIntervals are likely to cause the following complications:
* Increasing configuration complexity;
* Problems with writing queries and creating graphs;
* Short intervals are more like profiling an app, and Prometheus isn't the best tool to do this in most cases.

The most appropriate value for scrapeInterval is in the range of 10-60s.
