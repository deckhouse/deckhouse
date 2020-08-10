---
title: "Запись данных Prometheus в longterm storage"
tags:
  - prometheus
type:
  - instruction
search: prometheus remote write
---


Общая информация
----------------

1. У Prometheus есть поддержка remote_write данных из локального Prometheus в отдельный longterm storage (например: [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics))
2. В Deckhouse появилась поддержка данного механизма с помощью Custom Resource `PrometheusRemoteWrite`

### Ресурс PrometheusRemotwWrite

Таких ресурсов может быть сколько угодно в кластере. Данный Custom Resource является глобальным (не namespaced объект).

Параметры:
* `spec.url` — адрес по которому Prometheus будет отправлять данные.
  * Пример: `https://victoriametrics-test.domain.com/api/v1/write`
  * Обязательный параметр.
* `spec.basicAuth` — параметры базовой авторизации для отправки данных.
  * Необязательный параметр.
* `spec.writeRelabelConfigs` — параметры для relabel'инга данных для отпрваки (например удалить лишние метрики, или произвести релейбл данных). [Спецификация данного параметра](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#relabelconfig).
  * Необязательный параметр.

### Пример ресурса PrometheusRemoteWrite

Минимальный пример ресурса:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

Расширенный пример ресурса:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
  basicAuth:
    username: blahblah
    password: dddddddd
  writeRelabelConfigs:
  - sourceLabels: [__name__]
    action: keep
    regex: prometheus_build_.*
  - sourceLabels: [__name__]
    action: keep
    regex: my_cool_app_metrics_.*
```
