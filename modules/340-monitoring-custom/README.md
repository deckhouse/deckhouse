---
title: "Модуль monitoring-custom"
tags:
  - prometheus
  - monitoring
  - custom
type:
  - instruction
search: prometheus
---
Модуль monitoring-custom
========================

Модуль расширяет возможности модуля [prometheus](../300-prometheus/README.md) по мониторингу приложений пользователей.

* У модуля нет никаких параметров для настройки.
* Модуль включен по умолчанию, если включен модуль `prometheus`. Для отключения модуля необходимо добавить в конфигурацию dechkouse:
  ```yaml
  monitoringCustomEnabled: "false"
  ```
### Как собирать метрики с приложений в вашем проекте?

1. Необходимо поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или Pod. В значении указать имя приложения.
2. Указать порту, с которого необходимо собирать метрики, имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.
Если это не возможно, предлагается воспользоваться двумя аннотациями: `prometheus.deckhouse.io/port: номер_порта` для указания порта и `prometheus.deckhouse.io/tls: "true"`, если сбор метрик будет проходить по HTTPS.
3. Указать дополнительные аннотации для более тонкой настройки:
    * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`)
    * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready).
    * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 1000).

Подробнее о том, как мониторить приложения, можно ознакомиться [здесь](../../docs/guides/MONITORING.md).
