---
title: "Модуль operator-prometheus"
description: "Установка и управление системой мониторинга в кластере Deckhouse Kubernetes Platform."
---

Модуль устанавливает [prometheus operator](https://github.com/coreos/prometheus-operator), который позволяет создавать и автоматизированно управлять инсталляциями [Prometheus](https://prometheus.io/).

<!-- Исходник картинок: https://docs.google.com/drawings/d/1KMgawZD4q7jEYP-_g6FvUeJUaT3edro_u6_RsI3ZVvQ/edit -->

Функции устанавливаемого оператора:

- определяет следующие кастомные ресурсы:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов
  - `Alertmanager` — определяет кластер *Alertmanager*'ов
  - `PrometheusRule` — определяет список *Prometheus rules*
- следит за этими ресурсами и:
  - генерирует `StatefulSet` с самим *Prometheus* и необходимые для его работы конфигурационные файлы, сохраняя их в `Secret`;
  - следит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в `Secret`.

## Prometheus

### Что делает Prometheus?

В целом, сервер Prometheus делает две ключевых вещи — **собирает метрики** и **выполняет правила**:

* Для каждого *target'а* (цель для мониторинга), каждый `scrape_interval`, делает HTTP запрос на этот *target*, получает в ответ метрики в [своем формате](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md#text-format-details), которые сохраняет к себе в базу
* Каждый `evaluation_interval` обрабатывает *rules*, на основании чего:
  * или шлет алерты
  * или записывает (себе же в базу) новые метрики (результат выполнения *rule'а*)

### Как настраивается Prometheus?

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

### Где Prometheus берет список *target'ов*?

* В целом Prometheus работает следующим образом:

  ![Работа Prometheus](images/targets.png)

  * **(1)** Prometheus читает секцию конфигурации `scrape_configs`, согласно которой настраивает свой внутренний механизм Service Discovery
  * **(2)** Механизм Service Discovery взаимодействует с API Kubernetes (в основном — получает endpoint`ы)
  * **(3)** На основании происходящего в Kubernetes механизм Service Discovery обновляет Targets (список *target'ов*)
* В `scrape_configs` указан список *scrape job'ов* (внутреннее понятие Prometheus), каждый из которых определяется следующим образом:

  ```yaml
  scrape_configs:
    # Общие настройки
  - job_name: d8-monitoring/custom/0    # просто название scrape job'а, показывается в разделе Service Discovery
    scrape_interval: 30s                  # как часто собирать данные
    scrape_timeout: 10s                   # таймаут на запрос
    metrics_path: /metrics                # path, который запрашивать
    scheme: http                          # http или https
    # Настройки service discovery
    kubernetes_sd_configs:                # означает, что target'ы мы получаем из Kubernetes
    - api_server: null                    # означает, что адрес API-сервера использовать из переменных окружения (которые есть в каждом Pod'е)
      role: endpoints                     # target'ы брать из endpoint'ов
      namespaces:
        names:                            # искать endpoint'ы только в этих namespace'ах
        - foo
        - baz
    # Настройки "фильтрации" (какие enpoint'ы брать, а какие нет) и "релейблинга" (какие лейблы добавить или удалить, на все получаемые метрики)
    relabel_configs:
    # Фильтр по значению label'а prometheus_custom_target (полученного из связанного с endpoint'ом service'а)
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: .+                           # подходит любой НЕ пустой лейбл
      action: keep
    # Фильтр по имени порта
    - source_labels: [__meta_kubernetes_endpointslice_port_name]
      regex: http-metrics                 # подходит, только если порт называется http-metrics
      action: keep
    # Добавляем label job, используем значение label'а prometheus_custom_target у service'а, к которому добавляем префикс "custom-"
    #
    # Лейбл job это служебный лейбл Prometheus:
    #    * он определяет название группы, в которой будет показываться target на странице targets
    #    * и конечно же он будет у каждой метрики, полученной у этих target'ов, чтобы можно было удобно фильтровать в rule'ах и dashboard'ах
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: (.*)
      target_label: job
      replacement: custom-$1
      action: replace
    # Добавляем label namespace
    - source_labels: [__meta_kubernetes_namespace]
      regex: (.*)
      target_label: namespace
      replacement: $1
      action: replace
    # Добавляем label service
    - source_labels: [__meta_kubernetes_service_name]
      regex: (.*)
      target_label: service
      replacement: $1
      action: replace
    # Добавляем label instance (в котором будет имя Pod'а)
    - source_labels: [__meta_kubernetes_pod_name]
      regex: (.*)
      target_label: instance
      replacement: $1
      action: replace
  ```

* Таким образом, Prometheus сам отслеживает:
  * добавление и удаление Pod'ов (при добавлении/удалении Pod'ов Kubernetes изменяет endpoint'ы, а Prometheus это видит и добавляет/удаляет *target'ы*)
  * добавление и удаление сервисов (точнее endpoint'ов) в указанных namespace'ах
* Изменение конфигурации требуется в следующих случаях:
  * нужно добавить новый scrape config (обычно — новый вид сервисов, которые надо мониторить)
  * нужно изменить список namespace'ов

## Prometheus Operator

### Что делает Prometheus Operator?

* С помощью механизма CRD (Custom Resource Definitions) определяет четыре кастомных ресурса:
  * [prometheus](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#prometheus) — определяет инсталляцию (кластер) Prometheus
  * [servicemonitor](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#servicemonitor) — определяет, как "мониторить" (собирать метрики) набор сервисов
  * [alertmanager](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#alertmanager) — определяет кластер Alertmanager'ов
  * [prometheusrule](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#prometheusrule) — определяет список Prometheus rules
* Следит за ресурсами `prometheus` и генерирует для каждого:
  * StatefulSet (с самим Prometheus'ом)
  * Secret с `prometheus.yaml` (конфиг Prometheus'а) и `configmaps.json` (конфиг для `prometheus-config-reloader`)
* Следит за ресурсами `servicemonitor` и `prometheusrule` и на их основании обновляет конфиги (`prometheus.yaml` и `configmaps.json`, которые лежат в секрете).

### Что в Pod'е с Prometheus'ом?

![Что в Pod Prometheus](images/pod.png)

* Три контейнера:
  * `prometheus` — сам Prometheus
  * `prometheus-config-reloader` — [обвязка](https://github.com/coreos/prometheus-operator/tree/master/cmd/prometheus-config-reloader), которая:
    * следит за изменениями `prometheus.yaml` и, при необходимости, вызывает reload конфигурации Prometheus'у (специальным HTTP-запросом, см. [подробнее ниже](#как-обрабатываются-service-monitorы))
    * следит за PrometheusRule'ами (см. [подробнее ниже](#как-обрабатываются-кастомные-ресурсы-с-ruleами)) и по необходимости скачивает их и перезапускает Prometheus
  * `kube-rbac-proxy` — serves as an authentication and authorization proxy server based on RBAC for accessing Prometheus metrics.
* Pod использует несколько volume, из которых три — ключевые для работы Prometheus:
  * config — примонтированный secret (два файла: `prometheus.yaml` и `configmaps.json`). Подключен в оба контейнера.
  * rules — `emptyDir`, который наполняет `prometheus-config-reloader`, а читает `prometheus`. Подключен в оба контейнера, но в `prometheus` в режиме read only.
  * data — данные Prometheus. Подмонтирован только в `prometheus`.

### Как обрабатываются Service Monitor'ы?

![Как обрабатываются Service Monitor'ы](images/servicemonitors.png)

1. Prometheus Operator читает (а также следит за добавлением/удалением/изменением) Service Monitor'ы (какие именно Service Monitor'ы — указано в самом ресурсе `prometheus`, см. подробней [официальную документацию](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#prometheusspec)).
1. Для каждого Service Monitor'а, если в нем НЕ указан конкретный список namespace'ов (указано `any: true`), Prometheus Operator вычисляет (обращаясь к API Kubernetes) список namespace'ов, в которых есть Service'ы (подходящие под указанные в Service Monitor'е label'ы).
1. На основании прочитанных ресурсов `servicemonitor` (см. [официальную документацию](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#servicemonitorspec)) и на основании вычисленных namespace'ов Prometheus Operator генерирует часть конфигурации (секцию `scrape_configs`) и сохраняет конфиг в соответствующий Secret.
1. Штатными средствами самого Kubernetes данные из секрета прилетают в Pod (файл `prometheus.yaml` обновляется).
1. Изменение файла замечает `prometheus-config-reloader`, который по HTTP отправляет запрос Prometheus'у на перезагрузку.
1. Prometheus перечитывает конфиг и видит изменения в scrape_configs, которые обрабатывает уже согласно своей логике работы (см. подробнее выше).

### Как обрабатываются кастомные ресурсы с *rule'ами*?

![Как обрабатываются кастомные ресурсы с rule'ами](images/rules.png)

1. Prometheus Operator следит за PrometheusRule'ами (подходящими под указанный в ресурсе `prometheus` `ruleSelector`).
1. Если появился новый (или был удален существующий) PrometheusRule — Prometheus Operator обновляет `prometheus.yaml` (а дальше срабатывает логика в точности соответствующая обработке Service Monitor'ов, которая описана выше).
1. Как в случае добавления/удаления PrometheusRule'а, так и при изменении содержимого PrometheusRule'а, Prometheus Operator обновляет ConfigMap `prometheus-main-rulefiles-0`.
1. Штатными средствами самого Kubernetes данные из ConfigMap прилетают в Pod
1. Изменение файла замечает `prometheus-config-reloader`, который:
   - скачивает изменившиеся ConfigMap'ы в директорию rules (это `emptyDir`)
   - по HTTP отправляет запрос Prometheus'у на перезагрузку
1. Prometheus перечитывает конфиг и видит изменившиеся *rule'ы*.
