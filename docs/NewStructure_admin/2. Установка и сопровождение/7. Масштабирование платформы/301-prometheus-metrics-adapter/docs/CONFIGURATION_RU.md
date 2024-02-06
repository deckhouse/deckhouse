---
title: "Модуль prometheus-metrics-adapter: настройки"
search: autoscaler, HorizontalPodAutoscaler 
---

{% include module-bundle.liquid %}

Модуль работает, если включен модуль `prometheus`. В общем случае конфигурации не требуется.

## Параметры

* `highAvailability` — ручное включение/отключение режима отказоустойчивости. По умолчанию режим отказоустойчивости определяется автоматически. Смотри [подробнее](../../deckhouse-configure-global.html#параметры) про режим отказоустойчивости для модулей.
