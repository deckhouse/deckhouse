---
title: "Модуль descheduler: примеры"
---

## Пример стратегии LowNodeUtilization

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: lownodeutilization
spec:
  strategies:
    lowNodeUtilization:
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

## Пример стратегии HighNodeUtilization

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: highnodeutilization
spec:
  strategies:
    highNodeUtilization:
      thresholds:
        cpu: 50
        memory: 50
```
