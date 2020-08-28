---
title: "Модуль vertical-pod-autoscaler: Custom resources"
---

## VerticalPodAutoscaler

### Параметры
- `spec.targetRef`:
  - `apiVersion` — API версия объекта;
  - `kind` — тип объекта;
  - `name` — имя объекта.
- (не обязательно) `spec.updatePolicy.updateMode`: `Auto`, `Recreate`, `Initial`, `Off` (по умолчанию — `Auto`)
- (не обязательно) `resourcePolicy.containerPolicies` для конкретных контейнеров:
    - `containerName` — имя контейнера;
    - `mode` — `Auto` или `Off`, для включения или отключения работы автоскейлинга в указанном контейнере;
    - `minAllowed` — минимальная граница `cpu` и `memory` для контейнера;
    - `maxAllowed` — максимальная граница `cpu` и `memory` для контейнера.

### Примеры
#### Пример полного CR VerticalPodAutoscaler

```yaml
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: hamster
      minAllowed:
        memory: 100Mi
        cpu: 120m
      maxAllowed:
        memory: 300Mi
        cpu: 350m
      mode: Auto
```

#### Пример минимального CR VerticalPodAutoscaler

```yaml
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: StatefulSet
    name: my-app
```
