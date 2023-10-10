---
title: "The Prometheus monitoring module: FAQ"
type:
  - instruction
search: prometheus monitoring, prometheus custom alert, prometheus custom alerting
---


## How do I collect metrics from applications running outside of the cluster?

1. Configure a Service similar to the one that [collects metrics from your application](../../modules/340-monitoring-custom/#an-example-service) (but do not set the `spec.selector` parameter).
1. Create Endpoints for this Service and explicitly specify the `IP:PORT` pairs that your applications use to expose metrics.

   > Port names in Endpoints must match those in the Service.

### An example

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

Custom Grafana dashboards can be added to the project using the Infrastructure as a Code approach.
To add your dashboard to Grafana, create the dedicated [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition) Custom Resource in the cluster.

An example:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # The folder where the custom dashboard will be located.
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

> **Caution!** System dashboards and dashboards added using [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) cannot be modified via the Grafana interface.

## How do I add alerts and/or recording rules?

The `CustomPrometheusRules` resource allows you to add alerts.

Parameters:
- `groups` — is the only parameter where you need to define alert groups. The structure of the groups is similar to [that of prometheus-operator](https://github.com/prometheus-operator/prometheus-operator/blob/ed9e365370603345ec985b8bfb8b65c242262497/Documentation/api.md#rulegroup).

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

### An example of collecting metrics securely from an application inside a cluster

Do the following to set up application metrics protection via the `kube-rbac-proxy` with the subsequent metrics scraping using Prometheus tools:

1. Create a new `ServiceAccount` with the following permissions:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: rbac-proxy-test
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: d8:rbac-proxy
   subjects:
   - kind: ServiceAccount
     name: rbac-proxy-test
     namespace: default
   ```

   > The example uses the `d8:rbac-proxy` built-in Deckhouse `ClusterRole`.

2. Create a configuration for the `kube-rbac-proxy`:

   ```yaml
   ---
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: rbac-proxy-config-test
     namespace: rbac-proxy-test
   data:
     config-file.yaml: |+
       authorization:
         resourceAttributes:
           namespace: default
           apiVersion: v1
           resource: services
           subresource: proxy
           name: rbac-proxy-test
   ```

   > Get more information on authorization attributes in the [Kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/authorization).

3. Create `Service` and `Deployment` for your application with the `kube-rbac-proxy` as a sidecar container:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: rbac-proxy-test
     labels:
       prometheus.deckhouse.io/custom-target: rbac-proxy-test
   spec:
     ports:
     - name: https-metrics
       port: 8443
       targetPort: https-metrics
     selector:
       app: rbac-proxy-test
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: rbac-proxy-test
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: rbac-proxy-test
     template:
       metadata:
         labels:
           app: rbac-proxy-test
       spec:
         securityContext:
           runAsUser: 65532
         serviceAccountName: rbac-proxy-test
         containers:
         - name: kube-rbac-proxy
           image: quay.io/brancz/kube-rbac-proxy:v0.14.0
           args:
           - "--secure-listen-address=0.0.0.0:8443"
           - "--upstream=http://127.0.0.1:8081/"
           - "--config-file=/kube-rbac-proxy/config-file.yaml"
           - "--logtostderr=true"
           - "--v=10"
           ports:
           - containerPort: 8443
             name: https-metrics
           volumeMounts:
           - name: config
             mountPath: /kube-rbac-proxy
         - name: prometheus-example-app
           image: quay.io/brancz/prometheus-example-app:v0.1.0
           args:
           - "--bind=127.0.0.1:8081"
         volumes:
         - name: config
           configMap:
             name: rbac-proxy-config-test
   ```

4. Add the necessary resource permissions to Prometheus:

   ```yaml
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: rbac-proxy-test-client
   rules:
   - apiGroups: [""]
     resources: ["services/proxy"]
     verbs: ["get"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test-client
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: rbac-proxy-test-client
   subjects:
   - kind: ServiceAccount
     name: prometheus
     namespace: d8-monitoring
   ```

After step 4, your application's metrics should become available in Prometheus.

### An example of collecting metrics securely from an application outside a cluster

Suppose there is a server exposed to the Internet on which the `node-exporter` is running. By default, the `node-exporter` listens on port `9100` and is available on all interfaces. One needs to ensure access control to the `node-exporter` so that metrics can be collected securely. Below is an example of how you can set this up.

Requirements:
- There must be network access from the cluster to the `kube-rbac-proxy` service running on the *remote server*.
- The *remote server* must have access to the Kubernetes API server.

Follow these steps:
1. Create a new `ServiceAccount` with the following permissions:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: prometheus-external-endpoint
   rules:
   - apiGroups: ["authentication.k8s.io"]
     resources:
     - tokenreviews
     verbs: ["create"]
   - apiGroups: ["authorization.k8s.io"]
     resources:
     - subjectaccessreviews
     verbs: ["create"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: prometheus-external-endpoint-server-01
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: prometheus-external-endpoint
   subjects:
   - kind: ServiceAccount
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ```

2. Generate a `kubeconfig` file for the created `ServiceAccount` ([refer to the example on how to generate `kubeconfig` for `ServiceAccount`](https://deckhouse.io/documentation/v1/modules/140-user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access)).

3. Copy the `kubeconfig` file to the *remote server*. You will also have to specify the `kubeconfig` path in the `kube-rbac-proxy` settings (our example uses `${PWD}/.kube/config`).

4. Configure `node-exporter` on the *remote server* to be accessible only on the local interface (i.e., listening on `127.0.0.1:9100`).
5. Run `kube-rbac-proxy` on the *remote server*:

   ```shell
   docker run --network host -d -v ${PWD}/.kube/config:/config quay.io/brancz/kube-rbac-proxy:v0.14.0 --secure-listen-address=0.0.0.0:8443 \
     --upstream=http://127.0.0.1:9100 --kubeconfig=/config --logtostderr=true --v=10
   ```

6. Check that port `8443` is accessible at the remote server's external address.

7. Create `Service` and `Endpoint`, specifying the external address of the *remote server* as `<server_ip_address>`:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: prometheus-external-endpoint-server-01
     labels:
       prometheus.deckhouse.io/custom-target: prometheus-external-endpoint-server-01
   spec:
     ports:
     - name: https-metrics
       port: 8443
   ---
   apiVersion: v1
   kind: Endpoints
   metadata:
     name: prometheus-external-endpoint-server-01
   subsets:
     - addresses:
       - ip: <server_ip_address>
       ports:
       - name: https-metrics
         port: 8443
   ```

## How do I add Alertmanager?

Create a custom resource `CustomAlertmanager` with type `Internal`.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: webhook
spec:
  type: Internal
  internal:
    route:
      groupBy: ['job']
      groupWait: 30s
      groupInterval: 5m
      repeatInterval: 12h
      receiver: 'webhook'
    receivers:
    - name: 'webhook'
      webhookConfigs:
      - url: 'http://webhookserver:8080/'
```

Refer to the description of the [CustomAlertmanager](cr.html#customalertmanager) custom resource for more information about the parameters.

## How do I add an additional Alertmanager?

Create a custom resource `CustomAlertmanager` with the type `External`, it can point to Alertmanager through the FQDN or Kubernetes service.

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
  # default receiver
  receiver: some-other-receiver
  routes:
    - matchers:
        - matchType: =
          name: alertname
          value: DeadMansSwitch
      receiver: blackhole
    - matchers:
        - matchType: =~
          name: service
          value: ^(foo|bar|baz)$
      receiver: some-other-receiver
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

> **Caution!** Using this configuration creates a service in which Prometheus metrics are available without authorization.

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
  "39-log-format.sh": |
    cat > /etc/nginx/conf.d/log-format.conf <<"EOF"
    log_format  body  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"'
                      ' req body: $request_body';
    EOF
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
      access_log /dev/stdout body;
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
        - mountPath: /docker-entrypoint.d/39-log-format.sh
          name: prometheus-lens-proxy-conf
          subPath: 39-log-format.sh
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
Lens Prometheus type — `Prometheus Operator`.

Starting from the version `5.2.7`, Lens requires `pod` and `namespace` labels to be present on node-exporter metrics.
Otherwise, node resource consumption will not appear on Lens charts.

To fix this, apply the following resource:

{% offtopic title="A resource that fix the display of metrics..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: lens-hack
spec:
  groups:
  - name: lens-hack
    rules:
    - expr: node_cpu_seconds_total{mode=~"user|system", pod!~".+", namespace!~".+"}
        * on(node) group_left(namespace, pod) kube_pod_info{namespace="d8-monitoring",
        created_by_name="node-exporter"}
      record: node_cpu_seconds_total
    - expr: node_filesystem_size_bytes{mountpoint="/", pod!~".+", namespace!~".+"}
        * on(node) group_left(namespace, pod) kube_pod_info{namespace="d8-monitoring",
        created_by_name="node-exporter"}
      record: node_filesystem_size_bytes
    - expr: node_filesystem_avail_bytes{mountpoint="/", pod!~".+", namespace!~".+"}
        * on(node) group_left(namespace, pod) kube_pod_info{namespace="d8-monitoring",
        created_by_name="node-exporter"}
      record: node_filesystem_avail_bytes
    - expr: node_memory_MemTotal_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_MemTotal_bytes
    - expr: node_memory_MemFree_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_MemFree_bytes
    - expr: node_memory_Buffers_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_Buffers_bytes
    - expr: node_memory_Cached_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_Cached_bytes
```

{% endofftopic %}

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

## How do I set up a PrometheusRules to work with Prometheus?

Add the label `prometheus.deckhouse.io/rules-watcher-enabled: "true"` to the namespace where the PrometheusRules was created.

Example:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
```

## How to expand disk size

1. To request a larger volume for a PVC, edit the PVC object and specify a larger size in `spec.resources.requests.storage` field.
   * You can only expand a PVC if its storage class's `allowVolumeExpansion` field is set to true.
2. If storage doesn't support online resize, the message `Waiting for user to (re-)start a pod to finish file system resize of volume on node.` will appear in the PersistentVolumeClaim status.
3. Restart the Pod to complete the file system resizing.

## How to get information about alerts in a cluster?

You can get information about active alerts not only in the Grafana/Prometheus web interface but in the CLI. This can be useful if you only have access to the cluster API server and there is no way to open the Grafana/Prometheus web interface.

Run the following command to get cluster alerts:

```shell
kubectl get clusteralerts
```

Example:

```shell
# kubectl get clusteralerts
NAME               ALERT                                      SEVERITY   AGE     LAST RECEIVED   STATUS
086551aeee5b5b24   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
226d35c886464d6e   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
235d4efba7df6af4   D8SnapshotControllerPodIsNotReady          8          5d4h    44s             firing
27464763f0aa857c   D8PrometheusOperatorPodIsNotReady          7          5d4h    43s             firing
ab17837fffa5e440   DeadMansSwitch                             4          5d4h    41s             firing
```

Run the following command to view a specific alert:

```shell
kubectl get clusteralerts <ALERT_NAME> -o yaml
```

Example:

```shell
# kubectl get clusteralerts 235d4efba7df6af4 -o yaml
alert:
  description: |
    The recommended course of action:
    1. Retrieve details of the Deployment: `kubectl -n d8-snapshot-controller describe deploy snapshot-controller`
    2. View the status of the Pod and try to figure out why it is not running: `kubectl -n d8-snapshot-controller describe pod -l app=snapshot-controller`
  labels:
    pod: snapshot-controller-75bd776d76-xhb2c
    prometheus: deckhouse
    tier: cluster
  name: D8SnapshotControllerPodIsNotReady
  severityLevel: "8"
  summary: The snapshot-controller Pod is NOT Ready.
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAlert
metadata:
  creationTimestamp: "2023-05-15T14:24:08Z"
  generation: 1
  labels:
    app: prometheus
    heritage: deckhouse
  name: 235d4efba7df6af4
  resourceVersion: "36262598"
  uid: 817f83e4-d01a-4572-8659-0c0a7b6ca9e7
status:
  alertStatus: firing
  lastUpdateTime: "2023-05-15T18:10:09Z"
  startsAt: "2023-05-10T13:43:09Z"
```

Remember the special alert `DeadMansSwitch` — its presence in the cluster indicates that Prometheus is working.
