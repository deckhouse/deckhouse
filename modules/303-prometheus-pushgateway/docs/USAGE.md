---
title: "Модуль Prometheus Pushgateway: Примеры работы"
---

{% raw %}

Адрес PushGateway: `http://first.kube-prometheus-pushgateway:9091`.

## Отправка метрики через curl:

```shell
# echo "test_metric 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/app
```

Через 30 секунд (после скрейпа данных) метрики будут доступны в Prometheus:

```
test_metric{instance="10.244.1.155:9091",job="app",pushgateway="first"} 3.14
```

**Важно!** Значение job должно быть уникальным в Prometheus, чтобы не поломать существующие графики и алерты. Получить список всех занятых job можно следующим запросом: `count({__name__=~".+"}) by (job)`.

## Удаление всех метрик группы `{instance="10.244.1.155:9091",job="app"}` через curl:

```shell
# curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/app/instance/10.244.1.155:9091
```

Т.к. PushGateway хранит полученные метрики в памяти, **при рестарте pod-а все метрики будут утеряны**.
{% endraw %}
