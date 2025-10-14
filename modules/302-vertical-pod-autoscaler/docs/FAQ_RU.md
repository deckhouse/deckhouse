---
title: "Модуль vertical-pod-autoscaler: FAQ"
---

## Как посмотреть рекомендации Vertical Pod Autoscaler?

После создания кастомного ресурса [VerticalPodAutoscaler](cr.html#verticalpodautoscaler) посмотреть рекомендации VPA можно следующим образом:

```shell
d8 k describe vpa my-app-vpa
```

В секции `status` отобразятся параметры:

- `Target` — количество ресурсов, которое будет оптимальным для пода (в пределах resourcePolicy);
- `Lower Bound` — минимальное рекомендуемое количество ресурсов для более или менее (но не гарантированно) хорошей работы приложения;
- `Upper Bound` — максимальное рекомендуемое количество ресурсов. Скорее всего, ресурсы, выделенные сверх этого значения, идут в мусорку и совсем никогда не нужны приложению;
- `Uncapped Target` — рекомендуемое количество ресурсов в самый последний момент, то есть данное значение считается на основе самых крайних метрик, не смотря на историю ресурсов за весь период.

## Как Vertical Pod Autoscaler работает с лимитами?

### Пример 1

В примере представлен VPA-объект:

```yaml
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: test2
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: test2
  updatePolicy:
    updateMode: "Initial"
```

В VPA-объекте представлен под с ресурсами:

```yaml
resources:
  limits:
    cpu: 2
  requests:
    cpu: 1
```

Если контейнер использует весь CPU, и VPA рекомендует этому контейнеру 1.168 CPU, то отношение между запросами и ограничениями будет равно 100%.
В этом случае при пересоздании пода VPA модифицирует его и проставит такие ресурсы:

```yaml
resources:
  limits:
    cpu: 2336m
  requests:
    cpu: 1168m
```

### Пример 2

В примере представлен VPA-объект:

```yaml
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: test2
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: test2
  updatePolicy:
    updateMode: "Initial"
```

В VPA-объекте представлен под с ресурсами:

```yaml
resources:
  limits:
    cpu: 1
  requests:
    cpu: 750m
```

Если отношение запросов и ограничений равно 25%, и VPA рекомендует 1.168 CPU для контейнера, VPA изменит ресурсы контейнера следующим образом:

```yaml
resources:
  limits:
    cpu: 1557m
  requests:
    cpu: 1168m
```

Если необходимо ограничить максимальное количество ресурсов, которые могут быть выделены для ограничений контейнера, нужно использовать в спецификации объекта VPA `maxAllowed` или использовать [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) объекта Kubernetes.
