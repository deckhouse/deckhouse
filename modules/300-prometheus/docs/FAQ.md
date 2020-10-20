---
title: "Prometheus-мониторинг: FAQ"
type:
  - instruction
search: prometheus мониторинг, prometheus custom alert, prometheus кастомный алертинг
---

## Как собирать метрики с приложений в вашем проекте?

> Описываемый функционал реализуется модулем `monitoring-custom`, который включен по умолчанию, если включен модуль `prometheus`.

Чтобы организовать сбор метрик с приложения, поддержки которого нет в [модуле monitoring-applications](/modules/340-monitoring-applications/), необходимо:

1. Поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или Pod. Значение лейбла определит имя в списке target'ов Prometheus.
    * В качестве значения label'а prometheus.deckhouse.io/custom-target стоит использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере. При этом, если приложение ставится в кластер больше одного раза (staging, testing, etc) или даже ставится несколько раз в один namespace — достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы namespace, pod и, если доступ осуществляется через Service, лейбл service. То есть это название, уникально идентифицирующее приложение в кластере, а не единичную его инсталляцию.
2. Указать порту, с которого необходимо собирать метрики, имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно. Если это не возможно (например, порт уже определен и назван другим именем), то необходимо воспользоваться аннотациями: `prometheus.deckhouse.io/port: номер_порта` для указания порта и `prometheus.deckhouse.io/tls: "true"`, если сбор метрик будет проходить по HTTPS.
    * Пример 1:

      ```yaml
      ports:
      - name: https-metrics
        containerPort: 443
      ```

    * Пример 2:

      ```yaml
      annotations:
        prometheus.deckhouse.io/port: "443"
        prometheus.deckhouse.io/tls: "true"  # если метрики отдаются по http, эту аннотацию указывать не нужно
      ```

3. (Не обязательно) Указать дополнительные аннотации для более тонкой настройки.
    * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`)
    * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в очень редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кеши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
    * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 1000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Эту аннотацию надо вешать на тот же ресурс, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.

### Пример: Service
```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
  annotations:
    prometheus.deckhouse.io/port: "8061"                   # по умолчанию будет использоваться порт сервиса с именем http-metrics или https-metrics
    prometheus.deckhouse.io/path: "/my_app/metrics"        # по умолчанию /metrics
    prometheus.deckhouse.io/allow-unready-pod: "true"      # по умолчанию НЕ ready поды игнорируются
    prometheus.deckhouse.io/sample-limit: "5000"           # по умолчанию принимается не больше 1000 метрик от одного пода
spec:
  ports:
  - name: my-app
    port: 8060
  - name: http-metrics
    port: 8061
    targetPort: 8061
  selector:
    app: my-app
```

### Пример: Deployment:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
        prometheus.deckhouse.io/custom-target: my-app
      annotations:
        prometheus.deckhouse.io/sample-limit: "5000"  # по умолчанию принимается не больше 1000 метрик от одного пода
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```

## Как добавить дополнительные dashboard'ы в вашем проекте?

Добавление пользовательских dashboard'ов для Grafana в deckhouse реализовано при помощи подхода infrastructure as a code.
Чтобы ваш dashboard появился в Grafana, необходимо создать в кластере специальный ресурс — [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition).

Пример:
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
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```
**Важно!** Системные и добавленные через [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) dashboard нельзя изменить через интерфейс Grafana.

[Читайте подробнее](/modules/300-prometheus/grafana_dashboard_development.html) в документации по разработке графиков Grafana.

## Как добавить алерты и/или recording правила для вашего проекта?

Для добавления алертов существует специальный ресурс — `CustomPrometheusRules`.

Параметры:

`groups` — единственный параметр, в котором необходимо описать группы алертов. Структура групп полностью совпадает с [аналогичной в prometheus-operator](https://github.com/coreos/prometheus-operator/blob/ed9e365370603345ec985b8bfb8b65c242262497/Documentation/api.md#rulegroup).

Пример:
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
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```
### Как подключить дополнительные Datasource для Grafana?
Для подключения дополнительных datasource'ов к Grafana добавлен специальный ресурс - `GrafanaAdditionalDatasource`.

Параметры ресурса подробно описаны в [документации к Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file).

Пример:
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

## Как обеспечить безопасный доступ к метрикам?
Для обеспечения безопасности настоятельно рекомендуем использовать **kube-rbac-proxy**.

Подробнее процесс настройки описан [здесь](/modules/300-prometheus/prometheus_targets_development.html).

## Как добавить дополнительный alertmanager?

Создать сервис с лейблом `prometheus.deckhouse.io/alertmanager: main`, который указывает на ваш Alertmanager.

Опциональные аннотации:
* `prometheus.deckhouse.io/alertmanager-path-prefix` — префикс, который будет добавлен к HTTP-запросам.
  * По-умолчанию — "/".

**Важно!** На данный момент поддерживается только plain HTTP схема.

Пример:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-alertmanager
  namespace: my-monitoring
  labels:
    prometheus.deckhouse.io/alertmanager: main
  annotations:
    prometheus.deckhouse.io/alertmanager-path-prefix: /myprefix/
spec:
  type: ClusterIP
  clusterIP: None
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  selector:
    app: my-alertmanager
```
**Важно!!** если вы создаете Endpoints для Service вручную (например для использования внешнего alertmanager'а), обязательно указывать имя порта (name) и в Service, и в Endpoints.
