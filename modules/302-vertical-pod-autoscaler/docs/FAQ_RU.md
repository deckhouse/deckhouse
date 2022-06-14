---
title: "Модуль vertical-pod-autoscaler: FAQ"
---

## Как посмотреть рекомендации Vertical Pod Autoscaler?

После создания custom resource [VerticalPodAutoscaler](cr.html#verticalpodautoscaler) посмотреть рекомендации VPA можно следующим образом:

```shell
kubectl describe vpa my-app-vpa
```

В секции `status` будут такие параметры:
- `Target` — Количество ресурсов, которое будет оптимальным для Pod'а (в пределах resourcePolicy).
- `Lower Bound` — Минимальное рекомендуемое количество ресурсов для более или менее (но не гарантированно) хорошей работы приложения.
- `Upper Bound` — Максимальное рекомендуемое количество ресурсов. Скорее всего ресурсы выделенные сверх этого значения идут в мусорку и совсем никогда не нужны приложению.
- `Uncapped Target` — Рекомендуемое количество ресурсов в самый последний момент, т.е. данное значение считается на основе самых крайних метрик, не смотря на историю ресурсов за весь период.

## Как Vertical Pod Autoscaler работает с лимитами?

### Пример 1

У нас есть VPA объект:

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

И есть Pod с такими ресурсами:

```yaml
resources:
  limits:
    cpu: 2
  requests:
    cpu: 1
```

В данном случае если контейнер будет потреблять, к примеру 1 CPU целиком и VPA порекомендует данному контейнеру 1.168 CPU, то вычисляется ratio между requets и limits. В данном случае он будет равен 100%.
В этом случае, при пересоздании Pod'а VPA модифицирует Pod и проставит такие ресурсы:

```yaml
resources:
  limits:
    cpu: 2336m
  requests:
    cpu: 1168m
```

### Пример 2

У нас есть VPA объект:

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

И есть Pod с такими ресурсами:

```yaml
resources:
  limits:
    cpu: 1
  requests:
    cpu: 750m
```

В данном случае соотношение requests и limits будет равным 25% и если VPA порекомендует для контейнера 1.168 CPU, то VPA изменит ресурсы контейнера таким образом:

```yaml
resources:
  limits:
    cpu: 1557m
  requests:
    cpu: 1168m
```

Если вам необходимо ограничить максимальное количество ресурсов, которое может быть заданно для limits контейнера, то необходимо использовать в спецификации объекта VPA: `maxAllowed` или использовать [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) объект Kubernetes.
