---
title: "The log-shipper module: examples"
description: Examples of using the log-shipper Deckhouse module. Examples of module configuration, filtering, and collecting events and logs in a Kubernetes cluster.
---

{% raw %}

## Getting logs from all cluster pods and sending them to Loki

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

## Reading pod logs from a specified namespace with a specified label and redirecting to Loki and Elasticsearch

Reading logs from `namespace=whispers` with label `app=booking` and storing them into Loki and Elasticsearch:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: whispers-booking-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - whispers
    labelSelector:
      matchLabels:
        app: booking
  destinationRefs:
  - loki-storage
  - es-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: logs-%F
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

## Creating a source and reading logs of all pods in a namespace with forwarding them to Loki

The following pipeline creates a source in the `test-whispers` namespace, reads logs of all pods in that namespace and writes them to Loki:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

## Reading pods in the specified namespace, with a certain label

Example of reading logs from pods with the `app=booking` label in the `test-whispers` namespace:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  labelSelector:
    matchLabels:
      app: booking
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

## Migration from Promtail to Log-Shipper

The path `/loki/api/v1/push` has to be removed from the previously used Loki URL.

**Vector** will add this path automatically when working with Loki.

## Working with Grafana Cloud

This documentation expects that you [have created an API key](https://grafana.com/docs/grafana-cloud/reference/create-api-key/).

![Grafana cloud API key](../../images/log-shipper/grafana_cloud.png)

Firstly, you should encode your token with Base64.

```bash
echo -n "<YOUR-GRAFANA-CLOUD-TOKEN>" | base64 -w0
```

Then you can create **ClusterLogDestination**:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  loki:
    auth:
      password: PFlPVVItR1JBRkFOQUNMT1VELVRPS0VOPg==
      strategy: Basic
      user: "<YOUR-GRAFANA-CLOUD-USER>"
    endpoint: <YOUR-GRAFANA-CLOUD-URL> # For example, https://logs-prod-us-central1.grafana.net or https://logs-prod-eu-west-0.grafana.net.
  type: Loki
```

Now you can create PodLogginConfig or ClusterPodLoggingConfig and send logs to **Grafana Cloud**.

## Adding a Loki source to Deckhouse Grafana

You can work with Loki using Grafana embedded into Deckhouse. Just add [**GrafanaAdditionalDatasource**](../prometheus/cr.html#grafanaadditionaldatasource).

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: loki
spec:
  access: Proxy
  basicAuth: false
  jsonData:
    maxLines: 5000
    timeInterval: 30s
  type: loki
  url: http://loki.loki:3100
```

## Elasticsearch < 6.X usage

For Elasticsearch < 6.0, set the `doc_type` indexing using the following configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    docType: "myDocType" # Set any string here. It can't start with '_'.
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

## Index template for Elasticsearch

It is possible to route logs to particular indexes based on metadata using index templating:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ namespace }}-%F"
```

In this example, for each Kubernetes namespace a dedicated Elasticsearch index is created.

This feature works well together with `extraLabels`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ service }}-{{ namespace }}-%F"
  extraLabels:
    service: "{{ service_name }}"
```

1. If a log message is in JSON format, the `service_name` field of this JSON document is moved to the metadata level.
2. The new metadata field `service` is used for the index template.

## Splunk integration

It is possible to send logs from Deckhouse to Splunk.

1. The endpoint must be equal to the Splunk instance name with the `8088` port and no path provided, for example, `https://prd-p-xxxxxx.splunkcloud.com:8088`.
2. To add a token to ingest logs, go to `Setting` -> `Data inputs`, add a new `HTTP Event Collector` and copy the token.
3. Provide a Splunk index to store logs, for example, `logs`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: splunk
spec:
  type: Splunk
  splunk:
    endpoint: https://prd-p-xxxxxx.splunkcloud.com:8088
    token: xxxx-xxxx-xxxx
    index: logs
    tls:
      verifyCertificate: false
      verifyHostname: false
```

{% endraw %}
{% alert -%}
Splunk destination doesn't support pod labels for indexes. Consider exporting necessary labels with the `extraLabels` option.
{%- endalert %}
{% raw %}

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

## Simple Logstash example

To send logs to Logstash, the `tcp` input should be configured on the Logstash instance side, and its codec should be set to `json`.

An example of the minimal Logstash configuration:

```hcl
input {
  tcp {
    port => 12345
    codec => json
  }
}
output {
  stdout { codec => json }
}
```

An example of the ClusterLogDestination manifest:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: logstash
spec:
  type: Logstash
  logstash:
    endpoint: logstash.default:12345
```

## Syslog

The following example configures sending messages over a socket via TCP protocol in syslog format:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: rsyslog
spec:
  type: Socket
  socket:
    mode: TCP
    address: 192.168.0.1:3000
    encoding: 
      codec: Syslog
  extraLabels:
    syslog.severity: "alert"
    # The `request_id` field must be present in the log message.
    syslog.message_id: "{{ request_id }}"
```

## Graylog integration

Make sure that an incoming stream is configured in Graylog to receive messages over the TCP protocol on the specified port. Example manifest for integration with Graylog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-socket2-dest
spec:
  type: Socket
  socket:
    address: graylog.svc.cluster.local:9200
    mode: TCP
    encoding:
      codec: GELF
```

## Logs in CEF format

There is a way to format logs in CEF using `codec: CEF`, with overriding `cef.name` and `cef.severity` based on values from the `message` field (application log) in JSON format.

In the example below, `app` and `log_level` are the keys containing values for overriding:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: siem-kafka
spec:
  extraLabels:
    cef.name: '{{ app }}'
    cef.severity: '{{ log_level }}'
  type: Kafka
  kafka:
    bootstrapServers:
      - my-cluster-kafka-brokers.kafka:9092
    encoding:
      codec: CEF
    tls:
      verifyCertificate: false
      verifyHostname: true
    topic: logs
```

You can also manually set your own values:

```yaml
extraLabels:
  cef.name: 'TestName'
  cef.severity: '1'
```

## Collect Kubernetes events

Kubernetes events can be collected by `log-shipper` if `events-exporter` is enabled in the [`extended-monitoring`](../extended-monitoring/) module configuration.

Enable `events-exporter` by adjusting the `extended-monitoring` settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: extended-monitoring
spec:
  version: 1
  settings:
    events:
      exporterEnabled: true
```

Apply the following `ClusterLoggingConfig` to collect logs from the `events-exporter` pod:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubernetes-events
spec:
  type: KubernetesPods
  kubernetesPods:
    labelSelector:
      matchLabels:
        app: events-exporter
    namespaceSelector:
      matchNames:
      - d8-monitoring
  destinationRefs:
  - loki-storage
```

## Log filters

Users can apply the following filters to logs:

* `labelFilter`: Applies to the top-level metadata, for example, a container, namespace, or pod name.
* `logFilter`: Applies to fields of a message if it is in JSON format.

### Collect only logs of the `nginx` container

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: nginx-logs
spec:
  type: KubernetesPods
  labelFilter:
  - field: container
    operator: In
    values: [nginx]
  destinationRefs:
  - loki-storage
```

### Collect logs without the string containing `GET /status" 200`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
  labelFilter:
  - field: message
    operator: NotRegex
    values:
    - .*GET /status" 200$
```

### Audit of kubelet events

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
  - field: userAgent  
    operator: Regex
    values: ["kubelet.*"]
  destinationRefs:
  - loki-storage
```

### Deckhouse system logs

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: system-logs
spec:
  type: File
  file:
    include:
    - /var/log/syslog
  labelFilter:
  - field: message
    operator: Regex
    values:
    - .*d8-kubelet-forker.*
    - .*containerd.*
    - .*bashible.*
    - .*kernel.*
  destinationRefs:
  - loki-storage
```

{% endraw %}
{% alert -%}
If you need logs from only one or from a small group of a Pods, try to use the kubernetesPods settings to reduce the number of reading filed. Do not use highly grained filters to read logs from a single pod.
{%- endalert %}
{% raw %}

## Collect logs from production namespaces using the namespace label selector option

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: production-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchLabels:
          environment: production
  destinationRefs:
  - loki-storage
```

## Exclude pods or namespaces with a label

There is a preconfigured label to exclude particular namespaces or pods: `log-shipper.deckhouse.io/exclude=true`.
It can help to stop collecting logs from a namespace or pod without changing global configurations.

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
  labels:
    log-shipper.deckhouse.io/exclude: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  ...
  template:
    metadata:
      labels:
        log-shipper.deckhouse.io/exclude: "true"
```

## Exclude logs of a certain container in a pod

To exclude logs of a certain container in a pod, add the following annotation to the pod:

```yaml
vector.dev/exclude-containers: "container1,container2"
```

This annotation instructs Vector to skip logs originating from `container1` and `container2`.
Logs from other containers in the pod will be collected normally.

For details on this feature, refer to the [Vector's official docs](https://vector.dev/docs/reference/configuration/sources/kubernetes_logs/#container-exclusion).

## Enable buffering

The log buffering configuration is essential for improving the reliability and performance of the log collection system. Buffering can be useful in the following cases:

1. Temporary connectivity disruptions. If there are temporary disruptions or instability in the connection to the log storage system (such as Elasticsearch), a buffer allows logs to be temporarily stored and sent when the connection is restored.

1. Smoothing out load peaks. During sudden spikes in log volume, a buffer helps smooth out peak loads on the log storage system, preventing it from becoming overloaded and potentially losing data.

1. Performance optimization. Buffering helps optimize the performance of the log collection system by accumulating logs and sending them in batches, which reduces the number of network requests and improves overall throughput.

### Example of enabling in-memory buffering

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    memory:
      maxEvents: 4096
    type: Memory
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

### Example of enabling disk buffering

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

### Example of defining behavior when the buffer is full

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
    whenFull: DropNewest
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

For detailed description of parameters, refer to the [ClusterLogDestination](cr.html#clusterlogdestination) resource.

{% endraw %}
