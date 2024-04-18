---
title: "The monitoring-custom module"
type:
  - instruction
search: prometheus
---

The module extends the capabilities of the [prometheus](../../modules/300-prometheus/) module for monitoring user applications.

To enable the `monitoring-custom` module to collect application metrics, you must:

- Attach the `prometheus.deckhouse.io/custom-target` label to a Service or a Pod. The label value defines the name in the Prometheus list of targets.
  - You should use the application's name (in lowercase, separated by a hyphen "-") as the value of the `prometheus.deckhouse.io/custom-target` label. This way, the application will be uniquely identified in the cluster.

     Suppose the application is deployed in the cluster more than once (in staging, testing, etc.) or even has several copies in the same namespace. In that case, you still only need to specify its name since all metrics have namespace/Pod labels anyway (and the service label if it is accessed via the Service). In other words, the application's name uniquely identifies the application in the cluster (and not a specific installation of it).
- Set the `http-metrics` or `https-metrics` name to the port that will be used for collecting metrics to connect to it over HTTP or HTTPS, respectively.

  If it is not feasible for some reason (e.g., the port is already defined and has a different name), you can use the `prometheus.deckhouse.io/port: port_number` annotation to set the port number and `prometheus.deckhouse.io/tls: "true"` if metrics are collected over HTTPS.

  > **Note!** When annotating a Service, you must use `targetPort` as the port value. I.e., the port that is open and listening by your application, not the Service port.

  - Example No. 1:

    ```yaml
    ports:
    - name: https-metrics
      containerPort: 443
    ```

  - Example No. 2:

    ```yaml
    annotations:
      prometheus.deckhouse.io/port: "443"
      prometheus.deckhouse.io/tls: "true"  # you don't need to specify this annotation if metrics are sent over HTTP
    ```

- When using service mesh [Istio](../110-istio/) in mTLS STRICT mode, add the following Service annotation to force collecting metrics with proper mTLS certificate: `prometheus.deckhouse.io/istio-mtls: "true"`. Note that the application metrics must be exported via pure HTTP without TLS.
- *(Optional)* Attach the following annotations to fine-tune the monitoring:

  * `prometheus.deckhouse.io/path` — the path to collect metrics (default: `/metrics`);
  * `prometheus.deckhouse.io/query-param-$name` — GET parameters; they will be converted to a map of the following form: `$name=$value` (''  by default);
    - you can specify several such annotations;

      For example, `prometheus.deckhouse.io/query-param-foo=bar` & `prometheus.deckhouse.io/query-param-bar=zxc` annotations will be converted to the `http://...?foo=bar&bar=zxc` query;
  * `prometheus.deckhouse.io/allow-unready-pod` — enables collecting metrics for Pods in any state (by default, Prometheus scrapes metrics from the Ready Pods only). Note that this option is rarely helpful. For example, it can come in handy if it takes a long time to start your application (e.g., some data are loaded into the database at startup or some caching activity occurs), but valuable metrics are supplied during the startup process, helping to monitor it;
  * `prometheus.deckhouse.io/sample-limit` — sample limit for a Pod (5000 by default). The default value prevents a situation when the application suddenly starts to supply too many metrics, disrupting the entire monitoring process. This annotation must be attached to the resource with the `prometheus.deckhouse.io/custom-target` label;

### An example: Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
  annotations:
    prometheus.deckhouse.io/port: "8061"                      # by default, either the http-metrics or https-metrics service port is used
    prometheus.deckhouse.io/path: "/my_app/metrics"           # /metrics by default
    prometheus.deckhouse.io/query-param-format: "prometheus"  # '' by default
    prometheus.deckhouse.io/allow-unready-pod: "true"         # by default, NON-ready Pods are ignored
    prometheus.deckhouse.io/sample-limit: "5000"              # by default, the sample is limited to 5000 metrics for a single Pod
    prometheus.deckhouse.io/scrape-interval: "60s"            # by default scrapeInterval from monitoring module config
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

### An example: Deployment

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
        prometheus.deckhouse.io/sample-limit: "5000"   # by default, the sample is limited to 5000 metrics for a single Pod
        prometheus.deckhouse.io/scrape-interval: "60s" # by default scrapeInterval from monitoring module config
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```
