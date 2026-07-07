---
title: "Настройка системы сбора и хранения метрик"
permalink: ru/virtualization-platform/documentation/admin/platform-management/monitoring/prometheus.html
lang: ru
---

## Что делает Prometheus?

Prometheus собирает метрики и выполняет правила:

* Для каждого *target* (цели мониторинга) с заданной периодичностью `scrape_interval` Prometheus выполняет HTTP-запрос на этот *target*, получает в ответ метрики в [собственном формате](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md) и сохраняет их в свою базу данных.
* Каждый `evaluation_interval` обрабатывает правила (*rules*), на основании чего:
  * отправляет алерты;
  * или сохраняет новые метрики (результат выполнения правил) в свою базу данных.

## Как работает Prometheus?

Prometheus устанавливается модулем `prometheus-operator` DVP, который выполняет следующие функции:
- определяет следующие кастомные ресурсы:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов
  - `Alertmanager` — определяет кластер *Alertmanager*'ов
  - `PrometheusRule` — определяет список *Prometheus rules*
- следит за этими ресурсами, а также:
  - генерирует `StatefulSet` с самим *Prometheus*
  - создает секреты с необходимыми для работы Prometheus конфигурационными файлами (`prometheus.yaml` — конфигурация Prometheus, и `configmaps.json` — конфигурация для `prometheus-config-reloader`);
  - следит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в секреты.

Включить модуль можно с использованием следующего ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  settings:
    auth:
      password: xxxxxx
    retentionDays: 7
    storageClass: rbd
    nodeSelector:
      node-role/monitoring: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: monitoring
```

Полное описание всех настроек доступно [в документации модуля `prometheus`](/modules/prometheus/configuration.html).

## Как настраивается Prometheus?

* У сервера Prometheus есть *config* и есть *rule files* (файлы с правилами)
* В `config` имеются следующие секции:
  * `scrape_configs` — настройки поиска *target'ов* (целей для мониторинга, см. подробней следующий раздел).
  * `rule_files` — список директорий, в которых лежат *rule'ы*, которые необходимо загружать:

    ```yaml
    rule_files:
    - /etc/prometheus/rules/rules-0/*
    - /etc/prometheus/rules/rules-1/*
    ```

  * `alerting` — настройки поиска *Alert Manager'ов*, в которые слать алерты. Секция очень похожа на `scrape_configs`, только результатом ее работы является список *endpoint'ов*, в которые Prometheus будет слать алерты.

