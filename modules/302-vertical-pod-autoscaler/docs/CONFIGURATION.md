---
title: "Модуль vertical-pod-autoscaler: настройки"
search: autoscaler
---

По умолчанию — **включен** в кластерах начиная с версии 1.11. В общем случае конфигурации не требуется.

## Параметры

У модуля есть только настройки `nodeSelector/tolerations`:
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Примеры
```yaml
verticalPodAutoscaler: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

VPA работает не с контроллером пода, а с самим подом — измеряя и изменяя параметры его контейнеров. Вся настройка происходит с помощью custom resource [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler).
