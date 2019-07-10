Модуль antiopa
==============

Модуль не устанавливает, но настраивает Antiopa.

Конфигурация
------------

### Что нужно настроить?

Ничего!

### Параметры


* `logLevel` — уровень логирования Antiopa: `Debug`, `Info` или `Error`. По-умолчанию `Info`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
    * **Внимание!!!** Если вы укажете ошибочный параметр и kube-scheduler не найдет места для Antiopa — нужно будет поправить не только значение в конфиге, но и deployment `antiopa`: `kubectl -n antiopa edit deploy/antiopa`.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
    * **Внимание!!!** Если вы укажете ошибочный параметр и kube-scheduler не найдет места для Antiopa — нужно будет поправить не только значение в конфиге, но и deployment `antiopa`: `kubectl -n antiopa edit deploy/antiopa`.

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
