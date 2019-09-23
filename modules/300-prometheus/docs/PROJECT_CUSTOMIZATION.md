Кастомизация проекта
====================

### Как собирать метрики с кастомных приложений в конкретном проекте?

Для сбора custom метрик сделан [специальный ServiceMonitor](../templates/prometheus-targets/custom/service-monitor.yaml), так что все что нужно сделать — поправить уже существующий сервис (добавив необходимый label и порт) или создать дополнительный сервис (который будет использоваться исключительно для мониторинга, если необходимо).

* Должен быть сервис, у которого:
    * установлен label `prometheus-custom-target` (с любым значением)
    * задекларирован порт с названием `http-metrics` (или `https-metrics`, если нужно ограничить доступ к метрикам)
* Тип сервиса не имеет значения (это может быть и ClusterIP и `clusterIP: None`) — Prometheus использует только endpoints'ы (см. подробнее в [документации](../../200-operator-prometheus/docs/INTERNALS.md) о том, как все это работает).
* В качестве значения label'а `prometheus-custom-target` стоит использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере. При этом, если приложение ставится в кластер больше одного раза (staging, testing, etc) или даже ставится несколько раз в один namespace — достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы с `namespace` и `service`. То есть это название, уникально идентифицирующее приложение в кластере, а не единичную его инсталляцию.
* Порт `http-metrics` должен отвечать по HTTP (не HTTPS, а именно HTTP) по пути `/metrics`. Если никак не получается это сделать — можно подсадить sidecar контейнер с nginx, который будет делать необходимый rewrite.
* Если необходимо обеспечить безопасный доступ к метрикам, то нужно использовать порт `https-metrics` — он должен отвечать по HTTPS по пути `/metrics` и проверять сертификат клиента!!! Рекомендуется использовать sidecar контейнер с [kube-ca-auth-proxy](https://github.com/flant/kube-ca-auth-proxy) для проверки сертификата.
* После этих манипуляций, на странице Targets (`/prometheus/targets`), в группе `custom-0` или `custom-1` (для HTTP или HTTPS-метрик), среди прочих target-ов вы должны увидеть искомые: с метками `job="custom-<значение label'а prometheus-custom-target>"`. В них указаны IP-адреса всех pod'ов, на которые ссылается сервис. Если этого не произошло — придется разобраться [в устройстве Prometheus Operator](../../200-operator-prometheus/docs/INTERNALS.md).


[Читайте подробнее](PROMETHEUS_TARGETS_DEVELOPMENT.md) в документации по разработке target'ов Prometheus.

### Как добавлять кастомные dashboard'ы в конкретном проекте?

А очень просто! Любой dashboard, добавленный в `cm/grafana-dashboard-definitions-custom`, автоматически попадет в Grafana (в folder с названием Custom).
* Для custom dashboard'ов поддерживается только один cm (и пока это не является ограничением).
* Этот cm не создается автоматически (и не управляется antiop'ой), так что если его нет — его нужно просто создать: `kubectl -n kube-prometheus create cm grafana-dashboard-definitions-custom`.
* Любые изменения (в том числе и создание/удаление `cm/grafana-dashboard-definitions-custom`) подхватываются полностью автоматически, но требуется подождать около минуты (пока kubernetes зальет данные из cm в pod).
* Пример того, что нужно складывать в этот cm:

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: grafana-dashboard-definitions-custom
      namespace: kube-prometheus
    data:
      dashboard-name.json: |
        [
          "very long json",
          "with grafana dashboard definition"
        ]

    ```

* Сам dashboard создавать и править в Grafana, после чего [экспортировать в JSON](img/grafana_export.jpg) и сохранять в cm.

[Читайте подробнее](GRAFANA_DASHBOARD_DEVELOPMENT.md) в документации по разработке графиков Grafana.

### Как добавлять кастомные rule'ы в конкретном проекте?

А очень просто! Любой PrometheusRule объект с лейблами `component=rules` и `prometheus=main` в namespace `kube-prometheus` будет автоматически подхвачен prometheus'ом (см. [подробнее](../../200-operator-prometheus/docs/INTERNALS.md) о том, как это работает).
* Рекомендуется называть этот PrometheusRule `prometheus-rules-custom`. В названиях групп правил рекомендуется использовать или `custom.<имя файла>.<имя группы>` или просто `custom.<имя файла>`.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: PrometheusRule
    metadata:
      name: prometheus-rules-custom
      namespace: kube-prometheus
      labels:
        component: rules
        prometheus: main
    spec:
      groups:
      - name: custom.foo.xxx
        rules:
        - ...
        - ...
      - name: custom.foo.yyy
        rules:
        - ...
        - ...
      - name: custom.bar
        rules:
        - ...
        - ...
    ```
* Любые изменения (в том числе и добавление/удаление) этого PrometheusRule отрабатываются полностью автоматически, но требуется подождать около минуты (пока отработает Prometheus Operator и компания).

[Читайте подробнее](PROMETHEUS_RULES_DEVELOPMENT.md) в документации по разработке правил Prometheus.

### Как добавить кастомные конфиги scrape_configs, alert_relabel_configs, alertmanager_config?

Создать в неймспейсе `kube-prometheus` новый `Secret`:
* Рекомендуемое имя — prometheus-main-additional-configs-XXX.
* Лейбл — `additional-configs-for-prometheus=main`.
* Содержимое:
```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  labels:
    additional-configs-for-prometheus: main
  name: prometheus-main-additional-configs-example
data:
  alert-managers.yaml: "<base64>"
  alert-relabels.yaml: "<base64>"
  scrapes.yaml: "<base64>"
```

Указывать в секрете все три конфига необязательно, можно указать только нужные в данной ситуации.

Важно — при создании такого секрета, [хук](/modules/300-prometheus/hooks/additional_configs_render) соберёт все секреты с лейблом `additional-configs-for-prometheus=main` и смержит их в один главный секрет `prometheus-main-additional-configs`. Если в разных секретах попадутся одинаковые конфиги, он их конкатинирует.
