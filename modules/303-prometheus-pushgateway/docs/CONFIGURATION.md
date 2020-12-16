---
title: "Модуль Prometheus Pushgateway: настройки"
---

Данный модуль устанавливает в кластер [Prometheus Pushgateway](https://github.com/prometheus/pushgateway). Он предназначен для приема метрик от приложения и отдачи их Prometheus.

Модуль по умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  prometheusPushgatewayEnabled: "true"
  prometheusPushgateway: |
    instances:
    - example
```

## Параметры

* `instances` — данный параметр содержит список PushGateway-ев для каждого из которых будет создан отдельный PushGateway.
    * **Обязательный параметр**.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
