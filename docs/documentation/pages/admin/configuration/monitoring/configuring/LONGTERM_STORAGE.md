---
title: "Writing Prometheus data to longterm storage"
permalink: en/admin/configuration/monitoring/configuring/longterm-storage.html
---

Prometheus supports the remote_write mechanism for sending data from a local Prometheus instance to a separate long-term storage (for example, [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)). In DKP, support for this mechanism is implemented using the [PrometheusRemoteWrite](/modules/prometheus/cr.html#prometheusremotewrite) custom resource.

{% alert level="info" %}
For VictoriaMetrics, detailed information about ways to send data to vmagent can be found in the [VictoriaMetrics documentation](https://docs.victoriametrics.com/vmagent/index.html#how-to-push-data-to-vmagent).
{% endalert %}

## Example of minimal PrometheusRemoteWrite

Below is an example of minimal configuration for the [PrometheusRemoteWrite](/modules/prometheus/cr.html#prometheusremotewrite) custom resource:

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

## Example of advanced PrometheusRemoteWrite

Below is an example of advanced configuration for the [PrometheusRemoteWrite](/modules/prometheus/cr.html#prometheusremotewrite) custom resource:

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
  basicAuth:
    username: username
    password: password
  writeRelabelConfigs:
  - sourceLabels: [__name__]
    action: keep
    regex: prometheus_build_.*|my_cool_app_metrics_.*
  - sourceLabels: [__name__]
    action: drop
    regex: my_cool_app_metrics_with_sensitive_data
```

For a complete description of the custom resource fields, refer to the [prometheus module documentation](/modules/prometheus/cr.html#prometheusremotewrite).
