---
title: "Модуль vertical-pod-autoscaler: custom resources"
---

## VerticalPodAutoscaler

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
