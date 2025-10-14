---
title: "Logging settings"
permalink: en/user/logging/
---

Deckhouse Kubernetes Platform (DKP) provides log collection and delivery from cluster nodes and pods
to internal or external storage systems.

DKP allows you to:

- Collect logs from all or specific pods and namespaces.
- Filter logs by labels, message content and other attributes.
- Send logs to multiple storage systems simultaneously (e.g., Loki and Elasticsearch).
- Enrich logs with Kubernetes metadata.
- Use log buffering to improve performance.
- Store logs in internal short-term storage based on Grafana Loki.

The general mechanism of log collection, delivery and filtering is described in detail [in the "Architecture" section](../../architecture/logging/delivery.html).

DKP users can configure application log collection parameters using the [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource, which describes log sources within a specified namespace, including collection, filtering and parsing rules.

## Configuring application log collection

1. Check with the DKP administrator whether log collection and storage are configured in your cluster.
   Also ask them to provide you with the storage name that you will specify in the [`clusterDestinationRefs`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-clusterdestinationrefs) parameter.
1. Create a [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource in your namespace.

   In this example, logs are collected from all pods in the specified namespace
   and sent to short-term storage [based on Grafana Loki](../../admin/configuration/logging/storage.html):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: PodLoggingConfig
   metadata:
     name: app-logs
     namespace: my-namespace
   spec:
     clusterDestinationRefs:
       - loki-storage
   ```

1. (**Optional**) Limit log collection by label.

   If you need to collect logs only from specific pods,
   for example, only from applications with the `app=backend` label, add the [`labelSelector`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-labelselector) parameter:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: PodLoggingConfig
   metadata:
     name: app-logs
     namespace: my-namespace
   spec:
     clusterDestinationRefs:
       - loki-storage
     labelSelector:
       matchLabels:
         app: backend
   ```

1. (**Optional**) Configure log filtering.

   Using [`labelFilter`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-labelfilter) and [`logFilter`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-logfilter) filters, you can set up filtering by metadata or message fields.
   For example, in this case, only logs that do not contain fields with the string `.*GET /status" 200$` will be sent to storage:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: PodLoggingConfig
   metadata:
     name: app-logs
     namespace: my-namespace
   spec:
     clusterDestinationRefs:
       - loki-storage
     labelSelector:
       matchLabels:
         app: backend
     logFilter:
     - field: message
       operator: NotRegex
       values:
       - .*GET /status" 200$
   ```

1. Apply the created manifest using the following command:

   ```shell
   d8 k apply -f pod-logging-config.yaml
   ```

## Examples

### Creating a source in a namespace and reading logs from all pods in it with direction to Loki

The following pipeline creates a source in the `test-whispers` namespace, reads logs from all pods in it and sends them to Loki:

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

### Reading pods in a specified namespace with a specific label

Example of configuring reading pods with the `app=booking` label in the `test-whispers` namespace:

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
