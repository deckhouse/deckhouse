---
title: "Модуль Prometheus Pushgateway: примеры"
---

## Пример настройки модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus-pushgateway
spec:
  version: 1
  enabled: true
  settings:
    instances:
    - first
    - second
    - another
```

Адрес PushGateway (из контейнера пода): `http://first.kube-prometheus-pushgateway:9091`.

## Отправка метрики

Пример отправки метрики через curl:

```shell
echo "test_metric{env="dev"} 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp
```

Через 30 секунд (после скрейпа данных) метрики будут доступны в Prometheus. Пример:

```text
test_metric{container="prometheus-pushgateway", env="dev", exported_job="myapp", 
    instance="10.244.1.155:9091", job="prometheus-pushgateway", pushgateway="prometheus-pushgateway", tier="cluster"} 3.14
```

{% alert %} Название job (в примере — `myapp`) будет доступно в Prometheus в лейбле `exported_job`, а не `job` (так как лейбл `job` уже занят в Prometheus, он переименовывается при приеме метрики от PushGateway).
{% endalert %}

{% alert %} Возможно, вам потребуется получить список всех имеющихся job для выбора уникального названия (чтобы не испортить существующие графики и алерты). Получить список всех имеющихся job можно следующим запросом: {% raw %}`count({__name__=~".+"}) by (job)`.{% endraw %}
{% endalert %}

## Удаление метрик

Пример удаления всех метрик группы `{instance="10.244.1.155:9091",job="myapp"}` через curl:

```shell
curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp/instance/10.244.1.155:9091
```

Так как PushGateway хранит полученные метрики в памяти, **при рестарте пода все метрики будут утеряны**.
