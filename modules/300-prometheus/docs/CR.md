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

## CustomPrometheusRules

Ресурс хранения PrometheusRule.  Подробнее [о добавлении пользовательских алертов](/modules/300-prometheus/faq.html#как-добавить-алерты-иили-recording-правила-для-вашего-проекта).

## GrafanaAdditionalDatasource
Ресурс для подключения дополнительных datasource к Grafana.

Параметры ресурса подробно описаны в [документации к Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file). 

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
