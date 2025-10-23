---
title: "External access configuration"
permalink: en/admin/configuration/monitoring/configuring/external-access.html
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
  # Basic-auth string is hashed using htpasswd.
  auth: Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK  # foo:bar
```

Add the data source to Grafana:

{% alert level="info" %}
As the URL, you need to specify `https://prometheus-api.<your-cluster-domain>`.
{% endalert %}

<img src="../../../../images/prometheus/prometheus_connect_settings.png" height="500">

* **Basic authorization** is not a reliable security measure. It is recommended to implement additional security measures, such as specifying the `nginx.ingress.kubernetes.io/whitelist-source-range` annotation.

* Due to the need to create an Ingress resource in the system namespace, this connection method is **not recommended**.
  DKP **does not guarantee** the preservation of this connection scheme's functionality due to its active continuous updates.

* This Ingress resource can be used to access the Prometheus API not only for Grafana but also for other integrations, such as Prometheus federation.

## Connecting Third-Party Applications to Prometheus

The connection to Prometheus is secured using [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy). To connect, create a `ServiceAccount` with the necessary permissions.

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: app:prometheus-access
rules:
- apiGroups: ["monitoring.coreos.com"]
  resources: ["prometheuses/http"]
  resourceNames: ["main", "longterm"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: app:prometheus-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: app:prometheus-access
subjects:
- kind: ServiceAccount
  name: app
  namespace: default
```

Execute the request using the `curl` command:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: app-curl
  namespace: default
spec:
  template:
    metadata:
      name: app-curl
    spec:
      serviceAccountName: app
      containers:
      - name: app-curl
        image: curlimages/curl:7.69.1
        command: ["sh", "-c"]
        args:
        - >-
          curl -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" -k -f
          https://prometheus.d8-monitoring:9090/api/v1/query_range?query=up\&start=1584001500\&end=1584023100\&step=30
      restartPolicy: Never
  backoffLimit: 4
```

The `Job` should complete successfully.

## Metrics Collection via Gateway (Pushgateway)

Prometheus, which is the foundation of the DKP monitoring system, primarily uses a pull model for metrics collection. In this approach, DKP polls metric exporters. When applying the pull model is difficult, for example, for services without a permanent network interface, you can use metrics collection via a gateway (Pushgateway). Pushgateway allows such tasks to send metrics themselves, which can then be collected by Prometheus. It's important to note that Pushgateway can become a single point of failure and bottleneck in the system. How to send metrics from an application to Pushgateway can be learned from the [Prometheus documentation](https://prometheus.io/docs/instrumenting/pushing/).

Example of configuring metrics collection via gateway (Pushgateway):
- Enable and configure the [`prometheus-pushgateway`](/modules/prometheus-pushgateway/) module.

  You can enable the module in the web interface (Deckhouse Console), or using the following command:

  ```shell
  d8 platform module enable prometheus-pushgateway
  ```

- Specify the gateway names in the `instances` parameter of the `prometheus-pushgateway` module through the web interface, or using the following command:

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

  The address of the PushGateway instance named `first` from a pod container will be: `http://first.kube-prometheus-pushgateway:9091`.

- Check metrics sending.

  Example of sending metrics via curl:

  ```shell
  echo "test_metric{env="dev"} 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp
  ```

- Check that the metric appeared in the monitoring system. It will be available 30 seconds after data collection.

  Example PromQL query:

  ```text
  test_metric{container="prometheus-pushgateway", env="dev", exported_job="myapp", 
      instance="10.244.1.155:9091", job="prometheus-pushgateway", pushgateway="prometheus-pushgateway", tier="cluster"} 3.14
  ```

### Removing Metrics from Gateway (Pushgateway)

Example of removing all metrics from group `{instance="10.244.1.155:9091",job="myapp"}` via curl:

```shell
curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp/instance/10.244.1.155:9091
```

Note that PushGateway stores received metrics in memory. When the PushGateway pod restarts, all metrics will be lost.
