---
title: "Модуль descheduler: примеры"
---

## Настройка интервала перераспределения подов

Чтобы настроить, как часто модуль `descheduler` будет запускать цикл перераспределения подов, используйте параметр [`deschedulingInterval`](configuration.html#parameters-deschedulinginterval).

Например, чтобы запускать `descheduler` каждые 5 минут, укажите `deschedulingInterval: Frequent`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: descheduler
spec:
  enabled: true
  settings:
    deschedulingInterval: Frequent
```

Поддерживаются следующие значения:

- `Frequent` — запуск каждые 5 минут. Подходит для кластеров, где важно быстрее перераспределять поды;
- `Moderate` — запуск каждые 15 минут (по умолчанию);
- `Rare` — запуск каждые 30 минут. Подходит для кластеров, где важно минимизировать количество перераспределений подов.

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
