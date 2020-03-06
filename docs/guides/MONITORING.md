---
title: "prometheus Мониторинг приложений"
tags:
  - prometheus
type:
  - instruction
search: prometheus мониторинг, prometheus custom alert, prometheus кастомный алертинг
---

Мониторинг приложений
=====================

### Введение

Deckhouse поддерживает сбор метрик с многих распространненых приложений, а так же предоставляет необходимый минимальный набор alert'ов для Prometheus и dasbhaord'ов для Grafana.

Подробнее об этом описано в документации [модуля monitoring-applications](../../modules/340-monitoring-applications/README.md).

Если ни один из стандартных вариантов вам не подходит, следуйте дальнейшим инструкциям.

### Как собирать метрики с приложений в вашем проекте?

Чтобы организовать сбор метрик с приложения, поддержки которого нет в [модуле monitoring-applications](../../modules/340-monitoring-applications/README.md), необходимо:

* Чтобы у Pod или Service был проставлен label `prometheus.deckhouse.io/custom-target` с любым значением (значение определит имя в списке target'ов Prometheus).
    * В качестве значения label'а prometheus.deckhouse.io/custom-target стоит использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере. При этом, если приложение ставится в кластер больше одного раза (staging, testing, etc) или даже ставится несколько раз в один namespace — достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы namespace, pod и, если доступ осуществляется через Service, лейбл service. То есть это название, уникально идентифицирующее приложение в кластере, а не единичную его инсталляцию.
* Для указания порта, который необходимо скрейпить, используются ключевые имена портов - `http-metrics` для метрик, которые отдаются по HTTP и `https-metrics` для метрик, которые отдаются при помощи HTTPS.
    * Пример:
      ```yaml
      ports:
      - name: https-metrics
        containerPort: 443
      ```    
    * Если вы не имеете возможность указать имя порта (например, порт уже определен и назван другим именем), можно использовать аннотации для определения порта:
      ```yaml
      annotations:
        prometheus.deckhouse.io/port: "443"
        prometheus.deckhouse.io/tls: "true"  # если метрики отдаются по http, эту аннотацию указывать не нужно
      ```
* Если метрики отдаются по пути, отличном от `/metrics`, следует воспользоваться аннотацией `prometheus.deckhouse.io/path`.
* По умолчанию мы собираем данные только с Ready подов. Это поведение можно изменить, указав аннотацию `prometheus.deckhouse.io/allow-unready-pod` со значением "true".
    * Эта опция полезна в очень редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кеши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
* По умолчанию стоит ограничение на кол-во семплов, которые prometheus может собрать с вашего приложение - 1000 семплов. Это защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик и подвергает опасности работу всего мониторинга.
Если вы знаете, что вы делаете, есть возможность снять лимит при помощи аннотации `prometheus.deckhouse.io/sample-limit` со значением лимита, который вы хотите указать. Например "10000".

[Читайте подробнее](../../modules/300-prometheus/docs/PROMETHEUS_TARGETS_DEVELOPMENT.md) в документации по разработке target'ов Prometheus.

#### Пример: Service
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

#### Пример: Deployment:
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


### Как добавить дополнительные dashboard'ы в вашем проекте?

Есть два способа кастомизации dashboard'ов для Grafana.

1. Графана хранит данные персистентно. Все созданные или измененные через интерфейс grafana dashboard'ы будут сохранены.
2. Для реализации подхода infrastructure as a code можно использовать специальный ресурс - `GrafanaDashboardDefinition`.
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
**Важно!** Системные и добавленные через GrafanaDashboardDefinition dashboard'ы нельзя изменить через интерфейс Grafana. 

[Читайте подробнее](../../modules/300-prometheus/docs/GRAFANA_DASHBOARD_DEVELOPMENT.md) в документации по разработке графиков Grafana.

### Как добавить алерты и/или recording правила для вашего проекта?

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
        polk_flant_com_markup_format: markdown
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```

### Как обеспечить безопасный доступ к метрикам?
Для обеспечения безопасности настоятельно рекомендуем использовать **kube-rbac-proxy**. 

Подробнее процесс настройки описан [здесь](../../modules/300-prometheus/docs/PROMETHEUS_TARGETS_DEVELOPMENT.md).

### Как добавить дополнительный alertmanager?

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
