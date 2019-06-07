Модуль antiopa
==========================

Модуль не устанавливает, но настраивает antiopa.

Конфигурация
------------

### Что нужно настроить?

Ничего!

### Параметры


* `logLevel` — уровень логирования antiopa: `Debug`, `Info` или `Error`. По-умолчанию `Info`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/antiopa":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
    * **Внимание!!!** Если вы укажете ошибочный параметр и kube-schedule'r не найдет места для antiopa — нужно будет поправить не только значение в конфиге, но и deployment antiopa: `kubectl -n antiopa edit deploy/antiopa`.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"antiopa"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига

```yaml
antiopa: |
  logLevel: Debug
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
