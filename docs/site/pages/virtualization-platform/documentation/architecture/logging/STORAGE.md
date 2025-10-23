---
title: Short-term storage
permalink: en/virtualization-platform/documentation/architecture/logging/storage.html
---

Deckhouse provides a built-in solution for short-term log storage based on the [Grafana Loki](https://grafana.com/oss/loki/) project.

The storage is deployed in the cluster and integrated with the log collection system.
After configuring [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig), and [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resources,
logs automatically flow from all system components.
The configured storage is added to Grafana as a data source for visualization and analysis.

{% alert level="warning" %}
Short-term storage based on Grafana Loki does not support high availability mode.
For long-term storage of important logs, use external storage.
{% endalert %}
