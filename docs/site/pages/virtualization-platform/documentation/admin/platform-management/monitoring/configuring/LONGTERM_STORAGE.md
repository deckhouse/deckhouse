---
title: "Recording Prometheus data to long-term storage"
permalink: en/virtualization-platform/documentation/admin/platform-management/monitoring/configuring/longterm-storage.html
---

Prometheus supports the remote_write mechanism for sending data from a local Prometheus instance to a separate long-term storage (for example, [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)). In Deckhouse, support for this mechanism is implemented using the PrometheusRemoteWrite custom resource.

{% alert level="info" %}
For VictoriaMetrics, detailed information about ways to transfer data to vmagent can be found in the [VictoriaMetrics documentation](https://docs.victoriametrics.com/vmagent/index.html#how-to-push-data-to-vmagent).
{% endalert %}

## Example of minimal PrometheusRemoteWrite

Below is an example of a minimal configuration of the PrometheusRemoteWrite custom resource:

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

## Example of extended PrometheusRemoteWrite

Below is an example of an extended configuration of the PrometheusRemoteWrite custom resource:

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

For a complete description of the custom resource fields, see the [documentation of the `prometheus` module](/modules/prometheus/cr.html#prometheusremotewrite).
