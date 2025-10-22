---
title: "Модуль prometheus-metrics-adapter: настройки"
search: autoscaler, HorizontalPodAutoscaler 
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Модуль работает, если включен модуль `prometheus`. В общем случае конфигурации не требуется.

## Параметры

* `highAvailability` — ручное включение/отключение режима отказоустойчивости. По умолчанию режим отказоустойчивости определяется автоматически. Смотри [подробнее](/products/kubernetes-platform/documentation/v1/reference/api/global.html#параметры) про режим отказоустойчивости для модулей.
