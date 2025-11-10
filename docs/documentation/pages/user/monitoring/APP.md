---
title: "Configuring application monitoring"
permalink: en/user/monitoring/app.html
---

To organize metrics collection from any applications in the cluster, follow these steps:

1. Enable the `monitoring-custom` module if it is not enabled.

   You can enable cluster monitoring in the [Deckhouse web UI](/modules/console/), or using the following command:

   ```shell
   d8 platform module enable monitoring-custom
   ```
  
   > The current platform user may not have rights to enable or disable modules. If there are no rights, you need to contact the platform administrator.

1. Make sure that the application from which metrics will be collected provides them in Prometheus format.

1. Set the `prometheus.deckhouse.io/custom-target` label on the Service or pod that needs to be connected to monitoring. The label value will determine the name in the list of Prometheus targets.
  
   Example:

   ```yaml
   labels:
     prometheus.deckhouse.io/custom-target: my-app
   ```

   It is recommended to use the application name as the value of the `prometheus.deckhouse.io/custom-target` label, which allows it to be uniquely identified in the cluster.

   The label format must comply with [Kubernetes requirements](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/): no more than 63 characters, which can include alphanumeric characters (`[a-z0-9A-Z]`), as well as hyphens (`-`), underscores (`_`), dots (`.`).

   If the application is deployed in the cluster more than once (staging, testing, etc.) or even deployed several times in one namespace, one common name is sufficient, since all metrics will have `namespace`, `pod` labels anyway, and if access is through Service, the `service` label. This is the name that uniquely identifies the application in the cluster, not its single installation.

1. Specify the name `http-metrics` and `https-metrics` for the port from which metrics need to be collected for HTTP or HTTPS connection respectively.

   If this is not possible (for example, the port is already defined and named differently), you need to use annotations: `prometheus.deckhouse.io/port: port_number` — to specify the port and `prometheus.deckhouse.io/tls: "true"` — if metrics collection will be over HTTPS.

   > When specifying an annotation on a Service, you must use `targetPort` as the port value. That is, the port that is open and listened to by the application, not the Service port.

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

1. When using service mesh [Istio](../../admin/configuration/network/internal/encrypting-pods.html) in STRICT mTLS mode, specify the following annotation for metrics collection on Service or Pod: `prometheus.deckhouse.io/istio-mtls: "true"`. It is important that application metrics should be exported over HTTP protocol without TLS.

   Example:

   ```yaml
   annotations:
     prometheus.deckhouse.io/istio-mtls: "true"
   ```

## Example: Service

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
    prometheus.deckhouse.io/path: "/my_app/metrics"           # By default /metrics.
    prometheus.deckhouse.io/query-param-format: "prometheus"  # By default ''.
    prometheus.deckhouse.io/allow-unready-pod: "true"         # By default, pods NOT in Ready state are ignored.
    prometheus.deckhouse.io/sample-limit: "5000"              # By default, no more than 5000 metrics are accepted from one pod.
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

## Example: Deployment

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
        prometheus.deckhouse.io/sample-limit: "5000"  # By default, no more than 5000 metrics are accepted from one pod.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```

## Additional annotations for fine-tuning

For more precise application monitoring configuration, you can specify additional annotations for the pod or service for which monitoring is configured:

- `prometheus.deckhouse.io/path`: Path for metrics collection (default: `/metrics`).
- `prometheus.deckhouse.io/query-param-$name`: GET parameters that will be converted to a map of the form `$name=$value` (default: '').
  You can specify several such annotations.
  For example, `prometheus.deckhouse.io/query-param-foo=bar` and `prometheus.deckhouse.io/query-param-bar=zxc` will be converted to a request like `http://...?foo=bar&bar=zxc`.
- `prometheus.deckhouse.io/allow-unready-pod`: Allows metrics collection from pods in any state (by default, metrics are collected only from pods in Ready state). This option is useful in rare cases. For example, if your application starts very slowly (data is loaded into the database or caches are warmed up at startup), but useful metrics are already provided during startup that help monitor the application startup.
- `prometheus.deckhouse.io/sample-limit`: How many samples are allowed to be collected from a pod (default 5000). The default value protects against situations where the application suddenly starts providing too many metrics, which can disrupt the entire monitoring system. The annotation must be placed on the same resource where the `prometheus.deckhouse.io/custom-target` label is attached.
