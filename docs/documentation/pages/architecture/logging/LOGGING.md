---
title: Log collection and delivery
permalink: en/architecture/logging/delivery.html
---

This section describes the operation of logging system components in Deckhouse Kubernetes Platform (DKP).

## Log collection and delivery mechanism

The [`log-shipper` module](/modules/log-shipper/) is used for log collection and delivery in DKP.
A separate `log-shipper` instance runs on each cluster node and is configured based on DKP resources.
The `log-shipper` module uses [Vector](https://vector.dev/) as a logging agent.
The combination of settings for log collection and delivery forms a *pipeline*.

![log-shipper architecture](../../images/log-shipper/log_shipper_architecture.svg)

<!-- Source diagram: https://docs.google.com/drawings/d/1cOm5emdfPqWp9NT1UrB__TTL31lw7oCgh0VicQH-ouc/edit -->

1. DKP monitors ClusterLoggingConfig, ClusterLogDestination, and PodLoggingConfig resources:

   - [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig): Describes log sources at the cluster level,
     including collection, filtering, and parsing rules;
   - [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig): Describes log sources
     within a specified namespace, including collection, filtering, and parsing rules;
   - [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination): Sets log storage parameters.

1. Based on the specified parameters, DKP automatically creates a configuration file and saves it in a Secret in Kubernetes.
1. The Secret is mounted on all `log-shipper` agent pods.
   When the configuration changes, updates occur automatically using the `reloader` sidecar container.

## Log delivery schemes

DKP supports various log delivery topologies
depending on reliability requirements and resource consumption.

### Distributed

`log-shipper` agents send logs directly to storage, such as Loki or Elasticsearch.

![log-shipper distributed](../../images/log-shipper/log_shipper_distributed.svg)

<!-- Source images: https://docs.google.com/drawings/d/1FFuPgpDHUGRdkMgpVWXxUXvfZTsasUhEh8XNz7JuCTQ/edit -->

Advantages:

- Simple configuration.
- Available "out of the box" without additional dependencies except storage.

Disadvantages:

- Complex transformations consume more resources on application nodes.

### Centralized

All logs are sent to one of the available aggregators, such as Logstash or Vector.
Agents on nodes send logs as quickly as possible, consuming minimal resources.
Complex transformations are performed on the aggregator side.

![log-shipper centralized](../../images/log-shipper/log_shipper_centralized.svg)

<!-- Source images: https://docs.google.com/drawings/d/1TL-YUBk0CKSJuKtRVV44M9bnYMq6G8FpNRjxGxfeAhQ/edit -->

Advantages:

- Reduces resource consumption on application nodes.
- Users can configure any transformations in the aggregator and send logs to many more storage systems.

Disadvantages:

- Requires dedicated nodes for aggregators. Their number may increase depending on the load.

### Streaming

The main task of this architecture is to send logs to a message queue (e.g., Kafka) as quickly as possible,
from which they are transferred to long-term storage for further analysis in a service order.

![log-shipper stream](../../images/log-shipper/log_shipper_stream.svg)

<!-- Source images: https://docs.google.com/drawings/d/1R7vbJPl93DZPdrkSWNGfUOh0sWEAKnCfGkXOvRvK3mQ/edit -->

Advantages:

- Reduces resource consumption on application nodes.
- Users can configure any transformations in the aggregator and send logs to many more storage systems.
- High reliability. Suitable for infrastructure where log delivery is a priority task.

Disadvantages:

- Adds an intermediate link (message queue).
- Requires dedicated nodes for aggregators. Their number may increase depending on the load.

## Log processing

### Message filters

Before sending logs, DKP can filter out unnecessary records
to reduce the number of messages sent to storage.
For this, the `labelFilter` and `logFilter` filters of the `log-shipper` module are used.

![log-shipper pipeline](../../images/log-shipper/log_shipper_pipeline.svg)

<!-- Source images: https://docs.google.com/drawings/d/1SnC29zf4Tse4vlW_wfzhggAeTDY2o9wx9nWAZa_A6RM/edit -->

Filters run immediately after combining strings using multiline parsing.

- [`labelFilter`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-labelfilter):
  - Rules are applied to message metadata.
  - Metadata fields (or labels) are populated based on the log source,
    so different sources will have different sets of fields.
  - Rules are used, for example, to exclude messages from a specific container or pod
    matching a given label.
- [`logFilter`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-logfilter):
  - Rules are applied to the original message.
  - Allows excluding a message based on the value of a JSON field.
  - If the message is not in JSON format, you can use a regular expression to search the string.

Both filters have a unified configuration structure:

- `field`: Data source for filtering. Most often this is a label value or field from a JSON document.
- `operator`: Comparison action. Available options: `In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`.
- `values`: This option has different values for different operators:
  - `In`, `NotIn`: The field value must equal or not equal one of the values in the `values` list.
  - `Regex`, `NotRegex`: The value must match at least one
    or not match any regular expression from the `values` list.
  - `Exists`, `DoesNotExist`: Not supported.

{% alert level="info" %}
Additional labels (`extraLabels`) are added at the **Destination** stage, so filtering logs by them is not possible.
{% endalert %}

### Metadata

When processing logs, `log-shipper` automatically enriches messages with metadata depending on their source.
Enrichment occurs at the `Source` stage.

#### Kubernetes

When collecting logs from Kubernetes pods and nodes, the following fields are automatically exported:

| Label        | Pod spec path             |
|--------------|---------------------------|
| `pod`        | `metadata.name`           |
| `namespace`  | `metadata.namespace`      |
| `pod_labels` | `metadata.labels`         |
| `pod_ip`     | `status.podIP`            |
| `image`      | `spec.containers[].image` |
| `container`  | `spec.containers[].name`  |
| `node`       | `spec.nodeName`           |
| `pod_owner`  | `metadata.ownerRef[0]`    |

| Label        | Node spec path                              |
|--------------|---------------------------------------------|
| `node_group` | `metadata.labels[].node.deckhouse.io/group` |

{% alert level="info" %}
For Splunk, the `pod_labels` field is not exported because it is a nested object that Splunk does not support.
{% endalert %}

#### File

When collecting logs from file sources, only the `host` label is available,
which contains the hostname of the server from which the log came.
