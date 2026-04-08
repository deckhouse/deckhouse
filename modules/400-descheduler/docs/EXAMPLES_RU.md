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
    deschedulingInterval: "Frequent"
```

Доступные пресеты:
- `Frequent` — каждые 5 минут;
- `Moderate` — каждые 15 минут (по умолчанию);
- `Rare` — каждые 30 минут.

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
