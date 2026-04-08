---
title: "Модуль descheduler: примеры"
---

## Настройка интервала запуска descheduler

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: descheduler
spec:
  enabled: true
  settings:
    deschedulingInterval: "frequent"
```

Доступные пресеты:
- `frequent` — каждые 5 минут;
- `moderate` — каждые 15 минут (по умолчанию);
- `rare` — каждые 30 минут.

## Пример стратегии LowNodeUtilization

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization
spec:
  strategies:
    lowNodeUtilization:
      enabled: true
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

## Пример стратегии HighNodeUtilization

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: high-node-utilization
spec:
  strategies:
    highNodeUtilization:
      enabled: true
      thresholds:
        cpu: 50
        memory: 50
```
