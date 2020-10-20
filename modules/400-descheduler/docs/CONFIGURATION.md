---
title: "Модуль descheduler: конфигурация"
---

Обязательных настроек нет.

## Параметры

* `removePodsViolatingNodeAffinity` — включить данную политику.
  * По-умолчанию включено (`true`).
* `removePodsViolatingInterPodAntiAffinity` — включить данную политику.
  * По-умолчанию включено (`true`).
* `removeDuplicates` — включить данную политику.
  * По-умолчанию выключено (`false`).
* `lowNodeUtilization` — включить данную политику.
  * По-умолчанию выключено (`false`).
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
  * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
  * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
  * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
  * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Примеры

```yaml
descheduler: |
  removePodsViolatingNodeAffinity: false
  removeDuplicates: true
  lowNodeUtilization: true
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
