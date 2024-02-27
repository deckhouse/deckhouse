---
title: "Модуль operator-prometheus"
---

Модуль устанавливает [prometheus operator](https://github.com/coreos/prometheus-operator), который позволяет создавать и автоматизированно управлять инсталляциями [Prometheus](https://prometheus.io/).

<!-- Исходник картинок: https://docs.google.com/drawings/d/1KMgawZD4q7jEYP-_g6FvUeJUaT3edro_u6_RsI3ZVvQ/edit -->

Функциии `prometheus operator`:
* определение следующих custom ресурсов:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов
  - `Alertmanager` — определяет кластер *Alertmanager*'ов
  - `PrometheusRule` — определяет список *Prometheus rules*
* наблюдение за ресурсами, где:
  - генерирует `StatefulSet` с самим *Prometheus* и необходимые для его работы конфигурационные файлы, сохраняя их в `Secret`;
  - следит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в `Secret`.

## Prometheus

### Что делает Prometheus?

Сервер Prometheus — **собирает метрики** и **выполняет правила**:
* Для каждой *target* (цель для мониторинга), каждый `scrape_interval` делает HTTP запрос на *target* и получает в ответ метрики в [своем формате](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-format-details), которые сохраняет в свою базу.
* Каждый `evaluation_interval` обрабатывает *rules*, на основании которых шлет алерты или записывает (в свою базу) новые метрики (результат выполнения *rule'а*).

### Как настраивается Prometheus?

* У сервера Prometheus присутствуют *config* и *rule files* (файлы с правилами)
* В `config` имеются следующие секции:
  - `scrape_configs` — настройки поиска *target'ов* (целей для мониторинга, см. подробней следующий раздел).
  - `rule_files` — список директорий, в которых лежат *rule'ы*, которые необходимо загружать:

     ```yaml
     rule_files:
     - /etc/prometheus/rules/rules-0/*
     - /etc/prometheus/rules/rules-1/*
     ```

  * `alerting` — настройки поиска *Alert Manager'ов*, в которые попадают алерты. Секция похожа на `scrape_configs`, но результатом ее работы является список *endpoint'ов*, в которые Prometheus высылает алерты.

### Где Prometheus берет список target?

* Prometheus работает следующим образом:

  ![Работа Prometheus](../../images/200-operator-prometheus/targets.png)

  - **(1)** Prometheus читает секцию конфигурационного файла `scrape_configs`, по которой настраивает собственный внутренний механизм Service Discovery.
  - **(2)** Механизм Service Discovery взаимодействует с API Kubernetes (в основном — получает endpoint`ы).
  - **(3)** На основании событий, происходящих в Kubernetes, механизм Service Discovery обновляет список Targets (список *target'ов*).

* В `scrape_configs` указан список *scrape job'ов* (внутреннее понятие Prometheus), каждый определяется следующим образом:

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

* Prometheus отслеживает:
  * добавление и удаление подов (при добавлении/удалении подов Kubernetes изменяет endpoint, а Prometheus добавляет/удаляет *target*).
  * добавление и удаление сервисов (endpoint'ов) в указанных namespace.
* Изменение конфига требуется в следующих случаях:
  * для добавления нового scrape config (новый вид сервисов, которые мониторятся)
  * для изменения namespace'ов.

## Prometheus Operator

### Что делает Prometheus Operator?

* С помощью механизма CRD (Custom Resource Definitions) определяет четыре custom ресурса:
  - [prometheus](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheus) — определяет инсталляцию (кластер) Prometheus.
  - [servicemonitor](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#servicemonitor) — определяет, каким образом собирать метрики набора сервисов.
  - [alertmanager](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#alertmanager) — определяет кластер Alertmanager'ов (Alertmanager не используется, так как метрики отправляются непосредственно в [madison](https://madison.flant.com/).
  - [prometheusrule](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheusrule) — определяет список Prometheus rules.
* Следит за ресурсами `prometheus` и генерирует для каждого:
  - StatefulSet (с самим Prometheus'ом).
  - Secret с `prometheus.yaml` (конфиг Prometheus'а) и `configmaps.json` (конфиг для `prometheus-config-reloader`).
* Следит за ресурсами `servicemonitor` и `prometheusrule` и на их основании обновляет конфигурационные файлы (`prometheus.yaml` и `configmaps.json`, которые лежат в секрете).

### Что находится в поде с Prometheus?

![Что находится в под Prometheus](../../images/200-operator-prometheus/pod.png)

* Два контейнера:
  1. `prometheus` — сам Prometheus.
  2. `prometheus-config-reloader` — [обвязка](https://github.com/coreos/prometheus-operator/tree/master/cmd/prometheus-config-reloader), которая:
    - следит за изменениями `prometheus.yaml` и, при необходимости, вызывает reload конфигурации Prometheus (специальным HTTP-запросом, см. [подробнее ниже](#как-обрабатываются-service-monitorы))
    - следит за PrometheusRule (см. [подробнее ниже](#как-обрабатываются-custome-resources-с-ruleами)) и по необходимости скачивает их и перезапускает Prometheus.
* Под использует три volume:
  1. config — примонтированный secret (два файла: `prometheus.yaml` и `configmaps.json`). Подключен в оба контейнера.
  2. rules — `emptyDir`, который наполняет `prometheus-config-reloader`, а читает `prometheus`. Подключен в оба контейнера, но в `prometheus` в режиме read only.
  3. data — данные Prometheus. Подмонтирован только в `prometheus`.

### Как обрабатывается Service Monitor?

![Как обрабатывается Service Monitor](../../images/200-operator-prometheus/servicemonitors.png)

1. Prometheus Operator читает (а также следит за добавлением/удалением/изменением) Service Monitor (за каждым конкретным Service Monitor — это указано в самом ресурсе `prometheus`, см. подробней [официальную документацию](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheusspec)).
2. Для каждого Service Monitor, если в нем не указан конкретный список namespace (указано `any: true`), Prometheus Operator вычисляет (обращаясь к API Kubernetes) список namespace, в которых присутствуют сервисы (подходящие под указанные в Service Monitor лэйблы).
3.  На основании прочитанных ресурсов `servicemonitor` (см. [официальную документацию](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#servicemonitorspec)) и на основании вычисленных namespace Prometheus Operator генерирует часть конфигурационного файла (секцию `scrape_configs`) и сохраняет конфигурационный файл в соответствующий секрет.
4.  Штатными средствами самого Kubernetes данные из секрета попадают в под (файл `prometheus.yaml` обновляется).
5.  Изменение файла отмечается `prometheus-config-reloader`, который по HTTP отправляет запрос Prometheus на перезагрузку.
6.  Prometheus перечитывает конфигурационный файл и замечает изменения в scrape_configs, которые их обрабатывает по собственной логике (см. подробнее выше).

### Как обрабатываются Custome Resources с rule?

![Как обрабатываются Custome Resources с rule](../../images/200-operator-prometheus/rules.png)

1. Prometheus Operator следит за PrometheusRule'ами (подходящими под указанный в ресурсе `prometheus` `ruleSelector`).
2. Если появился новый PrometheusRule (или был удален существующий) — Prometheus Operator обновляет `prometheus.yaml` (также обрабатывает его по логике совпадающей с обработкой Service Monitor'ов, которая описана выше).
3. Как в случае добавления/удаления PrometheusRule'а, так и при изменении содержимого PrometheusRule'а, Prometheus Operator обновляет ConfigMap `prometheus-main-rulefiles-0`.
4.  Штатными средствами самого Kubernetes данные из ConfigMap помещаются в поде.
5.  Изменение файла отслеживает `prometheus-config-reloader`, который:
 * скачивает изменившиеся ConfigMap'ы в директорию rules (это `emptyDir`);
 * по HTTP отправляет запрос Prometheus'у на перезагрузку.
6. Prometheus перечитывает конфигурационный файл и отмечает изменившиеся *rules*.
