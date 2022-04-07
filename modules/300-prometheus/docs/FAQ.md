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

Custom Grafana dashboards can be added to the project using the infrastructure as a code approach.
To add your dashboard to Grafana, create the dedicated [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition) Custom Resource in the cluster.

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
- `groups` — is the only parameter where you need to define alert groups. The structure of the groups is similar to [that of prometheus-operator](https://github.com/coreos/prometheus-operator/blob/ed9e365370603345ec985b8bfb8b65c242262497/Documentation/api.md#rulegroup).

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
### How do I provision additional Grafana data sources?
The `GrafanaAdditionalDatasource` allows you to provision additional Grafana data sources.

A detailed description of the resource parameters is available in the [Grafana documentation](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file). 

See the datasource type in the documentation for the specific [datasource](https://grafana.com/docs/grafana/latest/datasources/).

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

## How do I add an additional Alertmanager?

Create a Custom Resource `CustomAlertmanager`, it can point to Alertmanager through the FQDN or Kubernetes service

FQDN Alertmanager example:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-fqdn-alertmanager
spec:
  external:
    address: https://alertmanager.mycompany.com/myprefix
  type: External
```

Alertmanager with a Kubernetes service:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-service-alertmanager
spec:
  external:
    service: 
      namespace: myns
      name: my-alertmanager
      path: /myprefix/
  type: External
```

Refer to the description of the [CustomAlertmanager](cr.html#customalertmanager) Custom Resource for more information about the parameters.

## How do I ignore unnecessary alerts in Alertmanager?

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

## How do I limit Prometheus resource consumption?

To avoid situations when VPA requests more resources for Prometheus or Longterm Prometheus than those available on the corresponding node, you can explicitly limit VPA using [module parameters](configuration.html):
- `vpa.longtermMaxCPU`
- `vpa.longtermMaxMemory`
- `vpa.maxCPU`
- `vpa.maxMemory`

## How do I get access to Prometheus metrics from Lens?

> ⛔ **_Attention!!!_** Using this configuration creates a service in which Prometheus metrics are available without authorization.

To provide Lens access to Prometheus metrics, you need to create some resources in a cluster.

{% offtopic title="Resource templates to be created..." %}
```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: lens-proxy
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus-lens-proxy
  namespace: lens-proxy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus-lens-proxy:prometheus-access
rules:
- apiGroups: ["monitoring.coreos.com"]
  resources: ["prometheuses/http"]
  resourceNames: ["main", "longterm"]
  verbs: ["get", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus-lens-proxy:prometheus-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus-lens-proxy:prometheus-access
subjects:
- kind: ServiceAccount
  name: prometheus-lens-proxy
  namespace: lens-proxy
---
apiVersion: v1
kind: Secret
metadata:
  name: prometheus-lens-proxy-sa
  namespace: lens-proxy
  annotations:
    kubernetes.io/service-account.name: prometheus-lens-proxy
type: kubernetes.io/service-account-token
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-lens-proxy-conf
  namespace: lens-proxy
data:
  "40-prometheus-proxy-conf.sh": |
    #!/bin/sh
    prometheus_service="$(getent hosts prometheus.d8-monitoring | awk '{print $2}')"
    nameserver="$(awk '/nameserver/{print $2}' < /etc/resolv.conf)"
    cat > /etc/nginx/conf.d/prometheus.conf <<EOF
    server {
      listen 80 default_server;
      resolver ${nameserver} valid=30s;
      set \$upstream ${prometheus_service};
      location / {
        proxy_http_version 1.1;
        proxy_set_header Authorization "Bearer ${BEARER_TOKEN}";
        proxy_pass https://\$upstream:9090$request_uri;
      }
    }
    EOF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus-lens-proxy
  namespace: lens-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus-lens-proxy
  template:
    metadata:
      labels:
        app: prometheus-lens-proxy
    spec:
      containers:
      - name: nginx
        image: nginx:1.21.4-alpine
        env:
        - name: BEARER_TOKEN
          valueFrom:
            secretKeyRef:
              name: prometheus-lens-proxy-sa
              key: token
        ports:
        - containerPort: 80
        volumeMounts:
        - mountPath: /docker-entrypoint.d/40-prometheus-proxy-conf.sh
          subPath: "40-prometheus-proxy-conf.sh"
          name: prometheus-lens-proxy-conf
      serviceAccountName: prometheus-lens-proxy
      volumes:
      - name: prometheus-lens-proxy-conf
        configMap:
          name: prometheus-lens-proxy-conf
          defaultMode: 0755
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus-lens-proxy
  namespace: lens-proxy
spec:
  selector:
    app: prometheus-lens-proxy
  ports:
  - protocol: TCP
    port: 8080
    targetPort: 80
```
{% endofftopic %}

After the resources deployment, Prometheus metrics will be available at address `lens-proxy/prometheus-lens-proxy:8080`.
Lens Prometheus type - `Prometheus Operator`.

## How do I set up a ServiceMonitor or PodMonitor to work with Prometheus?

Add the `prometheus: main` label to the PodMonitor or ServiceMonitor.
Add the label `prometheus.deckhouse.io/monitor-watcher-enabled: "true"` to the namespace where the PodMonitor or ServiceMonitor was created.

Example:
```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/monitor-watcher-enabled: "true"
---
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
```
