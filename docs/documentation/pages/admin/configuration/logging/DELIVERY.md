---
title: Log collection and delivery
permalink: en/admin/configuration/logging/delivery.html
description: "Configure log collection and delivery in Deckhouse Kubernetes Platform. Centralized logging from pods and nodes to internal or external storage systems with filtering and routing."
---

Deckhouse Kubernetes Platform (DKP) provides log collection and delivery from cluster nodes and pods to internal or external storage systems.

DKP allows you to:

- Collect logs from all or specific pods and namespaces.
- Filter logs by labels, message content, and other criteria.
- Send logs to multiple storage systems simultaneously (e.g., Loki and Elasticsearch).
- Enrich logs with Kubernetes metadata.
- Use log buffering to improve performance.

The general mechanism of log collection, delivery, and filtering is described in detail in the [Architecture](../../../architecture/logging/delivery.html) section.

DKP administrators can configure log collection and delivery using three custom resources:

- [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig): Describes log sources at the cluster level,
  including collection, filtering, and parsing rules.
- [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig): Describes log sources
  within a specified namespace, including collection, filtering, and parsing rules.
- [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination): Defines log storage parameters.

Based on these resources, a *pipeline* is formed, which is used in DKP to read logs
and further work with them using the [`log-shipper`](/modules/log-shipper/) module.
A complete list of `log-shipper` module settings is available in the [separate documentation section](/modules/log-shipper/configuration.html).

## Configuring log collection and delivery

Below is a basic DKP configuration option
where logs from all cluster pods are sent to Elasticsearch-based storage.

To configure, follow these steps:

1. Enable the [`log-shipper`](/modules/log-shipper/) module using the following command:

   ```shell
   d8 platform module enable log-shipper
   ```

1. Create a [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) resource that defines log collection rules.
   This resource allows you to configure log collection from pods in a specific namespace and with specific labels,
   flexibly configure multi-line log parsing, and set other rules.

   In this example, it is specified that logs should be collected from all pods and sent to Elasticsearch:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLoggingConfig
   metadata:
     name: all-logs
   spec:
     type: KubernetesPods
     destinationRefs:
     - es-storage
   ```

1. Create a [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resource,
   which describes the parameters for sending logs to storage.
   This resource allows you to specify one or more storage systems and describe connection parameters, buffering, and additional labels that will be applied to logs before sending.

   In this example, Elasticsearch is specified as the receiving storage:

   ```yaml
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

## Integration with external systems

You can configure DKP to work with external log storage and analysis systems,
such as Elasticsearch, Splunk, Logstash, and others,
using the [`type` parameter](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-type) of the ClusterLogDestination resource.

### Elasticsearch

To send logs to Elasticsearch, create a [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resource following this example:

```yaml
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

#### Using index templates

To send messages to specific indices based on metadata using index templates,
use the following configuration:

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

In the example above, a separate index will be created in Elasticsearch for each Kubernetes namespace.

This feature is useful in combination with the [`extraLabels` parameter](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-extralabels):

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

- If the message has JSON format, the `service_name` field of this JSON document is moved to the metadata level.
- The new metadata field `service` is used in the index template.

#### Support for Elasticsearch < 6.X

To work with Elasticsearch versions prior to 6.0, enable support for [`docType` indices](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-elasticsearch-doctype) using the ClusterLogDestination resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    docType: "myDocType" # Specify the value here. It should not start with '_'.
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

### Splunk

To configure sending events to Splunk, follow these steps:

1. Configure Splunk:
   - Define the endpoint. It should match your Splunk instance name with port `8088`, but without specifying the path,
   for example, `https://prd-p-xxxxxx.splunkcloud.com:8088`.
   - Create an access token. To do this, in Splunk, open the **Setting** -> **Data inputs** section,
   add a new **HTTP Event Collector** and copy the generated token.
   - Specify the Splunk index for storing logs, for example, `logs`.

1. Configure DKP by adding a [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resource to send logs to Splunk:

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

{% alert level="info" %}
`destination` does not support pod labels for indexing.
To add the required labels, use the [`extraLabels`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-extralabels) option:

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

{% endalert %}

### Logstash

To configure sending logs to Logstash, do the following:

1. Configure an incoming `tcp` stream with `json` codec on the Logstash side.

   Example Logstash configuration:

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

1. Add a [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resource:

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

### Graylog

To configure sending logs to Graylog, do the following:

1. Ensure that Graylog has an incoming stream configured to receive messages via TCP protocol on the specified port.
1. Create a [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resource following the example:

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

## Message formats

You can choose the format of sent messages using the [`.encoding.codec` parameter](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-socket-encoding-codec) of the ClusterLogDestination resource:

- CEF
- GELF
- JSON
- Syslog
- Text

Below are configuration examples for some of them.

### Syslog

Use the following configuration example to send messages via socket using TCP protocol in syslog format:

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
    # The request_id field must be present in the message.
    syslog.message_id: "{{ request_id }}"
```

### CEF

DKP can send logs in CEF format by using `codec: CEF`,
with overriding `cef.name` and `cef.severity` based on values from the `message` field of the application log in JSON format.

In the example below, `app` and `log_level` are keys containing values for overriding:

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

You can also set values manually:

```yaml
extraLabels:
  cef.name: 'TestName'
  cef.severity: '1'
```

## Log transformation

You can configure one or more types of transformations that will be applied to logs before sending to storage.

### Converting records to structured objects

The [`ParseMessage`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-parsemessage) transformation allows you to convert a string in the `message` field to a structured JSON object
based on one or more specified formats (String, Klog, SysLog, and others).

{% alert level="warning" %}
When using multiple `ParseMessage` transformations,
string conversion (`sourceFormat: String`) must be performed last.
{%- endalert %}

Example configuration for converting mixed format records:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLogDestination
metadata:
  name: parse-json
spec:
  ...
  transformations:
  - action: ParseMessage
    parseMessage:
      sourceFormat: JSON
  - action: ParseMessage
    parseMessage:
      sourceFormat: Klog
  - action: ParseMessage
    parseMessage:
      sourceFormat: String
      string:
        targetField: "text"
```

Example of the original log record:

```text
/docker-entrypoint.sh: Configuration complete; ready for start up
{"level" : { "severity": "info" },"msg" : "fetching.module.release"}
I0505 17:59:40.692994   28133 klog.go:70] hello from klog
```

Transformation result:

```json
{... "message": {
  "text": "/docker-entrypoint.sh: Configuration complete; ready for start up"
  }
}
{... "message": {
  "level" : "{ "severity": "info" }",
  "msg" : "fetching.module.release"
  }
}
{... "message": {
  "file":"klog.go",
  "id":28133,
  "level":"info",
  "line":70,
  "message":"hello from klog",
  "timestamp":"2025-05-05T17:59:40.692994Z"
  }
}
```

### Label replacement

The [`ReplaceKeys`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-replacekeys) transformation allows you to recursively replace all matches of the `source` pattern with the `target` value in the specified label keys.

{% alert level="warning" %}
Before applying the `ReplaceKeys` transformation to the `message` field or its nested fields,
convert the log record to a structured object using the [`ParseMessage`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-parsemessage) transformation.
{%- endalert %}

Example configuration for replacing dots with underscores in labels:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLogDestination
metadata:
  name: replace-dot
spec:
  ...
  transformations:
    - action: ReplaceKeys
      replaceKeys:
        source: "."
        target: "_"
        labels:
          - .pod_labels
```

Example of the original log record:

```json
{"msg" : "fetching.module.release"} # Pod label pod.app=test
```

Transformation result:

```json
{... "message": {
  "msg" : "fetching.module.release"
  },
  "pod_labels": {
    "pod_app": "test"
  }
}
```

### Label removal

The [`DropLabels`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-droplabels) transformation allows you to remove specified labels from a structured JSON message.

{% alert level="warning" %}
Before applying the `DropLabels` transformation to the `message` field or its nested fields,
convert the log record to a structured object using the [`ParseMessage`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-parsemessage) transformation.
{%- endalert %}

Example configuration with label removal and preliminary `ParseMessage` transformation:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLogDestination
metadata:
  name: drop-label
spec:
  ...
  transformations:
    - action: ParseMessage
      parseMessage:
        sourceFormat: JSON
    - action: DropLabels
      dropLabels:
        labels:
          - .message.example
```

Example of the original log record:

```json
{"msg" : "fetching.module.release", "example": "test"}
```

Transformation result:

```json
{... "message": {
  "msg" : "fetching.module.release"
  }
}
```

## Log filtering

DKP provides filters to exclude unnecessary messages to optimize the log collection process:

- [`labelFilter`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-labelfilter) — applied to metadata,
  such as container name (`container`), namespace (`namespace`), or pod name (`pod_name`);
- [`logFilter`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-logfilter) — applied to message fields,
  if the message is in JSON format.

### Collecting logs from a specific container

To configure filtering using `labelFilter`,
create a [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) resource,
using the configuration below as an example.

In this case, the filter selects logs from containers named `nginx`,
and then sends them to internal Loki-based storage.

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

### Collecting logs without a specific string

Example configuration for collecting logs with filtering via `labelFilter`,
where the `NotRegex` operator excludes strings matching the specified regular expression.

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

### Kubelet audit events

Example configuration for collecting and filtering audit events related to kubelet operation,
stored in the `/var/log/kube-audit/audit.log` file.
Filtering is performed using `logFilter`, which searches for records in the `userAgent` field
matching the regular expression `"kubelet.*"`.

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

### DKP system logs

Example configuration for collecting DKP system logs located in the `/var/log/syslog` file.
Message filtering using `labelFilter` allows you to select only those records
that relate to the following components:
`d8-kubelet-forker`, `containerd`, `bashible`, and `kernel`.

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

{% alert level="info" %}
If you need logs from only one pod or a small group of pods,
use [`kubernetesPods`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-kubernetespods) to limit the collection scope.
Filters should be used only for fine-tuning.
{%- endalert %}

## Log buffering

Using buffering improves the reliability and performance of the log collection system.
Buffering can be useful in the following cases:

- **Temporary connection issues**.
  If there are temporary interruptions or connection instability with the log storage system (e.g., Elasticsearch),
  the buffer allows temporarily storing logs and sending them when the connection is restored.

- **Load spike smoothing**.
  During sudden log volume spikes, the buffer helps smooth the peak load on the storage system,
  preventing its overload and potential data loss.

- **Performance optimization**.
  Buffering helps optimize the performance of the log collection system by accumulating logs and sending them in batches,
  which reduces the number of network requests and improves overall throughput.

The [`buffer` parameter](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-buffer) of the ClusterLogDestination resource is responsible for configuring buffering.

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

### Example of defining buffer overflow behavior

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

## Debugging and advanced features

### Enabling debug logs for the log-shipper agent

To enable debug logs for the [`log-shipper`](/modules/log-shipper/) agent on nodes with information about HTTP requests, connection reuse,
tracing, and other data, enable the [`debug` parameter](/modules/log-shipper/configuration.html#parameters-debug) in the `log-shipper` module configuration.

Example module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: log-shipper
spec:
  version: 1
  enabled: true
  settings:
    debug: true
```

### Additional information about log transmission channels

Using Vector commands, you can get additional information about data transmission channels.

First, connect to one of the `log-shipper` pods:

```bash
d8 k -n d8-log-shipper get pods -o wide | grep $node
d8 k -n d8-log-shipper exec $pod -it -c vector -- bash
```

Execute subsequent commands from the pod's command shell.

#### Topology overview

To get a diagram of your configuration topology:

1. Run the `vector graph` command. A diagram in DOT format will be generated.
1. Use [WebGraphviz](https://www.webgraphviz.com/) or a similar service to render the diagram based on the DOT file content.

Example diagram for one log transmission channel in ASCII format:

```text
+------------------------------------------------+
|  d8_cluster_source_flant-integration-d8-logs   |
+------------------------------------------------+
  |
  |
  v
+------------------------------------------------+
|       d8_tf_flant-integration-d8-logs_0        |
+------------------------------------------------+
  |
  |
  v
+------------------------------------------------+
|       d8_tf_flant-integration-d8-logs_1        |
+------------------------------------------------+
  |
  |
  v
+------------------------------------------------+
| d8_cluster_sink_flant-integration-loki-storage |
+------------------------------------------------+
```

#### Monitoring channel load

To view the traffic volume at each log processing stage, use the `vector top` command.

Example command output:

![Vector TOP output](../../../images/log-shipper/vector_top.png)

#### Getting raw and intermediate logs

To view input data at different log processing stages, use the `vector tap` command.
By specifying the ID of a specific processing stage, you can see logs that arrive at that stage.
Glob pattern selections are also supported, for example, `cluster_logging_config/*`.

Examples:

- Viewing logs before applying transformation rules
  (`cluster_logging_config/*` is the first processing stage according to the `vector graph` command output):

  ```bash
  vector tap 'cluster_logging_config/*'
  ```

- Modified logs entering the input of the next components in the channel chain:

  ```bash
  vector tap 'transform/*'
  ```

#### Debugging VRL rules

To debug rules [in Vector Remap Language (VRL)](https://vector.dev/docs/reference/vrl/),
use the `vector vrl` command.

Example VRL program:

```text
. = {"test1": "lynx", "test2": "fox"}
del(.test2)
.
```

### Adding support for new source or sink

The [`log-shipper`](/modules/log-shipper/) module in DKP is built based on Vector with a limited set of [cargo features](https://doc.rust-lang.org/cargo/reference/features.html),
to minimize the size of the executable file and speed up the build.

To view the complete list of supported features, run the `vector list` command.

If the required source or sink is missing, add the corresponding cargo feature to the Dockerfile.

## Special cases

### Collecting logs from production namespaces using the labelSelector option

If your cluster namespaces are labeled (e.g., `environment=production`),
you can use the [`labelSelector` option](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-kubernetespods-labelselector) to collect logs from production namespaces.

Example configuration:

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

### Label for excluding pods and namespaces

DKP provides the `log-shipper.deckhouse.io/exclude=true` label for excluding specific pods and namespaces.
It helps stop log collection from pods and namespaces without changing the global configuration.

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
