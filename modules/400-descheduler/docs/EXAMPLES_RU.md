---
title: "Модуль descheduler: примеры"
---

## Пример custom resource

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: example
spec:
  deschedulerPolicy:
    # Укажите параметры, применяющиеся ко всем стратегиям.
    globalParameters:
      evictFailedBarePods: true
    strategies:
      # Включите конкретную стратегию, указав ее параметры.
      podLifeTime:
        enabled: true

      # Включите стратегию и укажите дополнительные параметры.
      removeDuplicates:
        enabled: true
        parameters:
          nodeFit: true
```

## Пример custom resource для NodeGroup (labelSelector узла)

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
