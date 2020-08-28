---
title: "Prometheus-мониторинг: custom resources"
type:
  - instruction
search: prometheus
---

## GrafanaDashboardDefinition

Ресурс хранения и управления Dashboard в Grafana. Подробнее [о разработке графиков Grafana](grafana_dashboard_development.html).

### Параметры
* `spec.folder` — в какой Folder попадёт данная дашборда. Если такого Folder'а нет, он будет создан.
* `spec.definition` — json-манифест дашборды.
  * **Важно!** Следите, чтобы помимо `uid` в манифесте не было "местного" `id` по адресу `.id`.


### Пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # Папка, в которой в Grafana будет отображаться ваш dashboard
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
...
```

## CustomPrometheusRules

Ресурс хранения PrometheusRule.  Подробнее [о добавлении пользовательских алертов](/modules/300-prometheus/faq.html#как-добавить-алерты-иили-recording-правила-для-вашего-проекта).

### Пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomPrometheusRules
metadata:
  name: my-rules
spec:
  groups:
  - name: cluster-state-alert.rules
    rules:
    - alert: CephClusterErrorState
      annotations:
        description: Storage cluster is in error state for more than 10m.
        summary: Storage cluster is in error state
        plk_markup_format: markdown
        plk_protocol_version: "1"
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```

## GrafanaAdditionalDatasource
Ресурс для подключения дополнительных datasource к Grafana.

Параметры ресурса подробно описаны в [документации к Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file). 

### Пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAdditionalDatasource
metadata:
  name: another-prometheus
spec:
  type: prometheus
  access: proxy
  url: https://another-prometheus.example.com/prometheus
  basicAuth: true
  basicAuthUser: foo
  jsonData:
    timeInterval: 30s
  secureJsonData:
    basicAuthPassword: bar
```

## PrometheusRemoteWrite

Ресурс для включения `remote_write` данных из локального Prometheus в отдельный longterm storage (например: [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)).

Таких ресурсов в кластере может быть любое количество. Custom Resource `PrometheusRemoteWrite` является глобальным (не namespaced-объект).

### Параметры
* `spec.url` — адрес по которому Prometheus будет отправлять данные.
  * Пример: `https://victoriametrics-test.domain.com/api/v1/write`
  * Обязательный параметр.
* `spec.basicAuth` — параметры базовой авторизации для отправки данных.
  * Необязательный параметр.
* `spec.writeRelabelConfigs` — параметры для relabel'инга данных для отпрваки (например удалить лишние метрики, или произвести релейбл данных). [Спецификация данного параметра](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#relabelconfig).
  * Необязательный параметр.

### Примеры

Пример минимального ресурса:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

Пример расширенного ресурса:
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
