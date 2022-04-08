---
title: "Модуль Prometheus Pushgateway: настройки"
---

Данный модуль устанавливает в кластер [Prometheus Pushgateway](https://github.com/prometheus/pushgateway). Он предназначен для приема метрик от приложения и отдачи их Prometheus.

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  prometheusPushgatewayEnabled: "true"
  prometheusPushgateway: |
    instances:
    - example
```

## Параметры

<!-- SCHEMA -->
