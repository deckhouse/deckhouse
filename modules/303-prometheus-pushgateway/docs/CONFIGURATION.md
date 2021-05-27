---
title: "The Prometheus Pushgateway module: configuration"
---

This module installs [Prometheus Pushgateway](https://github.com/prometheus/pushgateway) into the cluster. It receives metrics from the app and pushes them to Prometheus.

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  prometheusPushgatewayEnabled: "true"
  prometheusPushgateway: |
    instances:
    - example
```

## Parameters

<!-- SCHEMA -->
