Модуль operator-prometheus
==========================

Модуль устанавливает [prometheus operator](https://github.com/coreos/prometheus-operator).

Как работает вся связка Prometheus и Prometheus Operator можно посмотреть в документации по [внутреннему устройству](docs/INTERNALS.md).

Конфигурация
------------

### Что нужно настроить?

Ничего!

### Параметры

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
