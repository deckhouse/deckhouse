---
title: "Prometheus-мониторинг: FAQ"
type:
  - instruction
search: prometheus мониторинг, prometheus custom alert, prometheus кастомный алертинг
---


## Как собирать метрики с приложений, расположенных вне кластера?

1. Сконфигурировать Service, по аналогии с сервисом для [сбора метрик с вашего приложения](../../modules/340-monitoring-custom/#пример-service), но без указания параметра `spec.selector`.
1. Создать Endpoints для этого Service, явно указав в них `IP:PORT`, по которым ваши приложения отдают метрики.
> Важный момент: имена портов в Endpoints должны совпадать с именами этих портов в Service. 

### Пример:
Метрики приложения доступны без TLS, по адресу `http://10.182.10.5:9114/metrics`.
```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
spec:
  ports:
  - name: http-metrics
    port: 9114
---
apiVersion: v1
kind: Endpoints
metadata:
  name: my-app
  namespace: my-namespace
subsets:
  - addresses:
    - ip: 10.182.10.5
    ports:
    - name: http-metrics
      port: 9114
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
    httpMethod: POST
  secureJsonData:
    basicAuthPassword: bar
```

## Как обеспечить безопасный доступ к метрикам?
Для обеспечения безопасности настоятельно рекомендуем использовать **kube-rbac-proxy**.

## Как добавить дополнительный alertmanager?

Создать сервис с лейблом `prometheus.deckhouse.io/alertmanager: main`, который указывает на ваш Alertmanager.

Опциональные аннотации:
* `prometheus.deckhouse.io/alertmanager-path-prefix` — префикс, который будет добавлен к HTTP-запросам.
  * По умолчанию — "/".

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

## Как в alertmanager игнорировать лишние алерты?

Решение сводится к настройке маршрутизации алертов в вашем Alertmanager.

Потребуется: 
1. Завести получателя без параметров.
1. Смаршрутизировать лишние алерты в этого получателя. 

В `alertmanager.yaml` это будет выглядеть так:
```yaml
receivers:
- name: blackhole
  # Получатель определенный без параметров будет работать как "/dev/null".
- name: some-other-receiver
  # ...
route:
  routes:
  - match:
      alertname: DeadMansSwitch
    receiver: blackhole
  - match_re:
      service: ^(foo1|foo2|baz)$
    receiver: blackhole
  - receiver: some-other-receiver
```

С подробным описанием всех параметров можно ознакомиться в [официальной документации](https://prometheus.io/docs/alerting/latest/configuration/#configuration-file).

## Почему нельзя установить разный scrapeInterval для отдельных таргетов?

Наиболее [полный ответ](https://www.robustperception.io/keep-it-simple-scrape_interval-id) на этот вопрос даёт разработчик Prometheus Brian Brazil.
Если коротко, то разные scrapeInterval'ы принесут следующие проблемы:
* Увеличение сложности конфигурации
* Проблемы при написании запросов и создании графиков
* Короткие интервалы больше похожи на профилирование приложения, и, скорее всего, Prometheus не самый подходящий инструмент для этого

Наиболее разумное значение для scrapeInterval находится в диапазоне 10-60s.
