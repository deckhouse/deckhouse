Модуль prometheus-operator
==========================

Модуль устанавливает [prometheus operator](https://github.com/coreos/prometheus-operator).

Как работает вся связка Prometheus и Prometheus Operator можно посмотреть в документации по [внутреннему устройству](docs/INTERNALS.md).

Конфигурация
------------

### Что нужно настроить?

Ничего!

### Параметры

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/prometheus-operator":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"prometheus-operator"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
