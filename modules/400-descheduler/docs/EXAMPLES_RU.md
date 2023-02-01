---
title: "Модуль descheduler: примеры"
---

## Example CR

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
