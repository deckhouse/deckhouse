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

* `instances` — this parameter contains a list of instances; a separate PushGateway will be created for each instance.
    * **The mandatory parameter**.
* `nodeSelector` — is the same as the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any nodeSelector.
* `tolerations` — is the same as the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any tolerations.
