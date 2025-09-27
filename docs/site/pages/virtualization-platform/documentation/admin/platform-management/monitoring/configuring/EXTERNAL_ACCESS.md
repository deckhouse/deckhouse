---
title: "External access configuration"
permalink: en/virtualization-platform/documentation/admin/platform-management/monitoring/configuring/external-access.html
---

## Connecting Prometheus to Third-Party Grafana

Each `ingress-nginx-controller` has certificates that, when specified as client certificates, will allow connection to Prometheus. All you need to do is create an additional `Ingress` resource.

{% alert level="info" %}
The example below assumes that the Secret `example-com-tls` already exists in the d8-monitoring namespace.

The names for Ingress `my-prometheus-api` and Secret `my-basic-auth-secret` are provided as examples. Replace them with more suitable ones for your setup.
{% endalert %}

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-prometheus-api
  namespace: d8-monitoring
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/auth-type: basic
    nginx.ingress.kubernetes.io/auth-secret: my-basic-auth-secret
    nginx.ingress.kubernetes.io/app-root: /graph
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_ssl_certificate /etc/nginx/ssl/client.crt;
      proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
      proxy_ssl_protocols TLSv1.2;
      proxy_ssl_session_reuse on;
spec:
  ingressClassName: nginx
  rules:
  - host: prometheus-api.example.com
    http:
      paths:
      - backend:
          service:
            name: prometheus
            port:
              name: https
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - prometheus-api.example.com
    secretName: example-com-tls
---
apiVersion: v1
kind: Secret
metadata:
  name: my-basic-auth-secret
  namespace: d8-monitoring
type: Opaque
data:
  # The basic-auth string is hashed using htpasswd.
  auth: Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK  # foo:bar
```

Add the data source in Grafana:

{% alert level="info" %}
As the URL, you need to specify `https://prometheus-api.<your-cluster-domain>`
{% endalert %}

<img src="/images/prometheus/prometheus_connect_settings.png" height="500">

- **Basic authorization** is not a reliable security measure. It is recommended to implement additional security measures, for example, specify the `nginx.ingress.kubernetes.io/whitelist-source-range` annotation.

- Due to the need to create an Ingress resource in the system namespace, this connection method is **not recommended**.
  Deckhouse **does not guarantee** the preservation of this connection scheme's functionality due to its active continuous updates.

- This Ingress resource can be used to access the Prometheus API not only for Grafana but also for other integrations, for example, for Prometheus federation.

## Metrics Collection via Gateway (Pushgateway)

Prometheus, which is the foundation of the DVP monitoring system, primarily uses a pull model for metrics collection. With this approach, metric exporters are polled from the DVP side. When the pull model is difficult to apply, for example, for services without a permanent network interface, you can use metrics collection via a gateway (Pushgateway). Pushgateway allows such tasks to send metrics themselves, which can then be collected by Prometheus. It is important to note that Pushgateway can become a single point of failure and bottleneck in the system. How to send metrics from an application to Pushgateway can be learned from the [Prometheus documentation](https://prometheus.io/docs/instrumenting/pushing/).

Example of configuring metrics collection via gateway (Pushgateway):

1. Enable and configure the `prometheus-pushgateway` module.

   You can enable the module in the web interface (Deckhouse Console), or using the following command:

   ```shell
   d8 platform module enable prometheus-pushgateway
   ```

2. Specify the gateway names in the `instances` parameter of the `prometheus-pushgateway` module through the web interface, or using the following command:

   ```shell
   d8 k edit mc prometheus-pushgateway
   ```

   Example module configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: prometheus-pushgateway
   spec:
     version: 1
     enabled: true
     settings:
       instances:
       - first
       - second
       - another
   ```

   The address of the PushGateway instance named `first` from the pod container will be: `http://first.kube-prometheus-pushgateway:9091`.

3. Check metrics sending.

   Example of sending a metric via curl:

   ```shell
   echo "test_metric{env="dev"} 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp
   ```

4. Check that the metric appeared in the monitoring system. It will be available 30 seconds after data collection.

   Example PromQL query:

   ```text
   test_metric{container="prometheus-pushgateway", env="dev", exported_job="myapp", 
       instance="10.244.1.155:9091", job="prometheus-pushgateway", pushgateway="prometheus-pushgateway", tier="cluster"} 3.14
   ```

### Removing Metrics from Gateway (Pushgateway)

Example of removing all metrics from the group `{instance="10.244.1.155:9091",job="myapp"}` via curl:

```shell
curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp/instance/10.244.1.155:9091
```

Note that PushGateway stores received metrics in memory. When the PushGateway pod restarts, all metrics will be lost.