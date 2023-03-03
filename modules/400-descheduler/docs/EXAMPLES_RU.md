---
title: "Модуль descheduler: примеры"
---

## Пример CR

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: example
spec:
  deschedulerPolicy:
    # укажите параметры, применяющиеся ко всем стратегиям
    globalParameters:
      evictFailedBarePods: true
    strategies:
      # включите конкретную стратегию, указав её параметры
      podLifeTime:
        enabled: true

      # включите стратегию и укажите дополнительные параметры
      removeDuplicates:
        enabled: true
        parameters:
          nodeFit: true
```

## Пример CR для NodeGroup (labelSelector ноды)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: example-specific-ng
spec:
  deploymentTemplate:
    nodeSelector:
      node.deckhouse.io/group: worker
```
