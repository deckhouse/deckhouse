Модуль prometheus-operator
=======

Модуль устанавливает [prometheus operator](https://github.com/coreos/prometheus-operator).


Конфигурация
------------

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/system":""}`.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/system","operator":"Exists"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
