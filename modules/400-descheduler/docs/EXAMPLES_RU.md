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
    parameters:
      evictFailedBarePods: true
    strategies:
      # включите конкретную стратегию, указав её параметры
      podLifeTime:
        params:
          podLifeTime:
            maxPodLifeTimeSeconds: 86400
            podStatusPhases:
              - Pending

      # включите стратегию, но заполните её параметры автоматически значениями по умолчанию
      removeDuplicates: { }
```
