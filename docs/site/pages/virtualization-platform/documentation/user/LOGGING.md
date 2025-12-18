---
title: "Logging settings"
permalink: en/virtualization-platform/documentation/user/logging.html
---

Deckhouse Virtualization Platform (DVP) provides log collection and delivery from cluster nodes and pods
to internal or external storage systems.

DKP allows you to:

- Collect logs from all or specific pods and namespaces;
- Filter logs by labels, message content, and other criteria;
- Send logs to multiple storage systems simultaneously (e.g., Loki and Elasticsearch);
- Enrich logs with Kubernetes metadata;
- Use log buffering to improve performance;
- Store logs in internal short-term storage based on Grafana Loki.

The general mechanism for log collection, delivery, and filtering is described in detail [in the "Architecture" section](/products/virtualization-platform/documentation/architecture/logging/delivery.html).

DVP users can configure log collection parameters from applications using the [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource, which describes the log source within a given namespace, including collection, filtering, and parsing rules.

## Configuring Log Collection from Applications

1. Check with the DKP administrator whether log collection and storage are configured in your cluster.
   Also ask them to provide you with the storage name that you will specify in the [`clusterDestinationRefs`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-clusterdestinationrefs) parameter.

1. Create a [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource in your namespace.

   In this example, logs are collected from all pods in the specified namespace
   and sent to short-term storage [based on Grafana Loki](/products/virtualization-platform/documentation/admin/platform-management/logging/storage.html):

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
   for example, only from applications with the `app=backend` label, add the [`labelSelector` parameter](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-labelselector):

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
   For example, in this case, only those logs that do not contain fields with the string `.*GET /status" 200$` will be sent to storage:

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

### Creating a Source in a Namespace and Reading Logs from All Pods in It with Direction to Loki

The following pipeline creates a source in the `test-whispers` namespace, reads logs from all pods in it, and sends them to Loki:

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

### Reading Pods in a Specified Namespace with a Specific Label

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
