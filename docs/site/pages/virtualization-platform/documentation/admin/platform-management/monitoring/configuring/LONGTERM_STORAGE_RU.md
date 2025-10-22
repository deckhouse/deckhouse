---
title: "Запись данных Prometheus в долговременное хранилище"
permalink: ru/virtualization-platform/documentation/admin/platform-management/monitoring/configuring/longterm-storage.html
lang: ru
---

Prometheus поддерживает механизм remote_write для отправки данных из локального экземпляра Prometheus в отдельное долговременное хранилище (например, [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)). В Deckhouse поддержка этого механизма реализована с помощью кастомного ресурса PrometheusRemoteWrite.

{% alert level="info" %}
Для VictoriaMetrics подробную информацию о способах передачи данные в vmagent можно получить в [документации](https://docs.victoriametrics.com/vmagent/index.html#how-to-push-data-to-vmagent) VictoriaMetrics.
{% endalert %}

## Пример минимального PrometheusRemoteWrite

Ниже приведён пример минимальной конфигурации кастомного ресурса PrometheusRemoteWrite:

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

## Пример расширенного PrometheusRemoteWrite

Ниже приведён пример расширенной конфигурации кастомного ресурса PrometheusRemoteWrite:

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

С полным описанием полей кастомного ресурса можно ознакомиться [в документации модуля `prometheus`](/modules/prometheus/cr.html#prometheusremotewrite).
