Модуль Prometheus Pushgateway
=======

Данный модуль устанавливает в кластер [Prometheus Pushgateway](https://github.com/prometheus/pushgateway). Он предназначен для приема метрик от приложения и отдачи их Prometheus.

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения добавьте в CM `antiopa`:

```yaml
data:
  prometheusPushgatewayEnabled: "true"
```

### Параметры

* `instances` — данный параметр содержит список PushGateway-ев для каждого из которых будет создан отдельный PushGateway.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфигурации

```yaml
prometheusPushgatewayEnabled: "true"
prometheusPushgateway: |
  instances:
  - first
  - second
  - another
```

### О Prometheus Pushgateway

* [Когда стоит использовать](https://prometheus.io/docs/practices/pushing/)
* [Как использовать](https://prometheus.io/docs/instrumenting/pushing/).

#### Пример работы с PushGateway:

Адрес PushGateway: `http://first.kube-prometheus-pushgateway:9091`.

##### Отправка метрики через curl:

```shell
# echo "test_metric 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/app
```

Через 30 секунд (после скрейпа данных) метрики будут доступны в Prometheus:

```
test_metric{instance="10.244.1.155:9091",job="app",pushgateway="first"} 3.14
```

**Важно!** Значение job должно быть уникальным в Prometheus, чтобы не поломать существующие графики и алерты. Получить список всех занятых job можно следующим запросом: `count({__name__=~".+"}) by (job)`.

##### Удаление всех метрик группы `{instance="10.244.1.155:9091",job="app"}` через curl:

```shell
# curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/app/instance/10.244.1.155:9091
```

Т.к. PushGateway хранит полученные метрики в памяти, **при рестарте pod-а все метрики будут утеряны**.
