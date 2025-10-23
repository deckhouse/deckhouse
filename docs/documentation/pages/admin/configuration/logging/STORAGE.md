---
title: Short-term log storage
permalink: en/admin/configuration/logging/storage.html
description: "Configure short-term log storage in Deckhouse Kubernetes Platform using Grafana Loki. Log collection, storage management, and centralized logging solution setup."
---

Deckhouse Kubernetes Platform (DKP) provides a built-in solution for short-term log storage based on the [Grafana Loki](https://grafana.com/oss/loki/) project.

The storage is deployed in the cluster and integrated with the log collection system.
After configuring [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig), and [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resources,
logs automatically flow from all system components.
The configured storage is added to Grafana as a data source for visualization and analysis.

Log collection from user applications is configured separately.

Short-term storage parameters are set in the [`loki`](/modules/loki/configuration.html) module settings.
You can configure disk size and retention period, specify the StorageClass to use, and set resource limits.

{% alert level="warning" %}
Short-term storage based on Grafana Loki does not support high availability mode.
For long-term storage of important logs, use external storage.
{% endalert %}

## Integration with Grafana Cloud

To configure DKP to work with the Grafana Cloud platform, follow these steps:

1. Create a [Grafana Cloud API access key](https://grafana.com/docs/grafana-cloud/reference/create-api-key/).
1. Encode the Grafana Cloud access token in Base64 format:

   ![Grafana Cloud API key](../../../images/log-shipper/grafana_cloud.png)

   ```bash
   echo -n "<YOUR-GRAFANACLOUD-TOKEN>" | base64 -w0
   ```

1. Create a [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resource following the example:

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
         user: "<YOUR-GRAFANACLOUD-USERNAME>"
       endpoint: <YOUR-GRAFANACLOUD-URL> # For example https://logs-prod-us-central1.grafana.net or https://logs-prod-eu-west-0.grafana.net
     type: Loki
   ```

## Migration from Grafana Promtail

To migrate from Promtail, edit the Loki URL by removing the `/loki/api/v1/push` path from it.

The Vector logging agent used in DKP will automatically add this path when sending data to Loki.
