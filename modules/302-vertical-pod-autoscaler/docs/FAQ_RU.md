---
title: "Модуль vertical-pod-autoscaler: FAQ"
---

## Как посмотреть рекомендации Vertical Pod Autoscaler?

После создания custom resource [VerticalPodAutoscaler](cr.html#verticalpodautoscaler) посмотреть рекомендации VPA можно следующим образом:

```shell
kubectl describe vpa my-app-vpa
```

В секции `status` будут такие параметры:
- `Target` — количество ресурсов, которое будет оптимальным для пода (в пределах resourcePolicy);
- `Lower Bound` — минимальное рекомендуемое количество ресурсов для более или менее (но не гарантированно) хорошей работы приложения;
- `Upper Bound` — максимальное рекомендуемое количество ресурсов. Скорее всего, ресурсы, выделенные сверх этого значения, идут в мусорку и совсем никогда не нужны приложению;
- `Uncapped Target` — рекомендуемое количество ресурсов в самый последний момент, то есть данное значение считается на основе самых крайних метрик, не смотря на историю ресурсов за весь период.

## Как Vertical Pod Autoscaler работает с лимитами?

### Пример 1

У нас есть VPA-объект:

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

И есть под с такими ресурсами:

```yaml
resources:
  limits:
    cpu: 2
  requests:
    cpu: 1
```

В данном случае, если контейнер будет потреблять, к примеру, 1 CPU целиком и VPA порекомендует данному контейнеру 1,168 CPU, вычисляется ratio между реквестами и лимитами. В данном случае он будет равен 100%.
В этом случае при пересоздании пода VPA модифицирует его и проставит такие ресурсы:

```yaml
resources:
  limits:
    cpu: 2336m
  requests:
    cpu: 1168m
```

### Пример 2

У нас есть VPA-объект:

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

И есть под с такими ресурсами:

```yaml
resources:
  limits:
    cpu: 1
  requests:
    cpu: 750m
```

В данном случае соотношение реквестов и лимитов будет равным 25%, и если VPA порекомендует для контейнера 1,168 CPU, то VPA изменит ресурсы контейнера таким образом:

```yaml
resources:
  limits:
    cpu: 1557m
  requests:
    cpu: 1168m
```

Если вам необходимо ограничить максимальное количество ресурсов, которое может быть заданно для лимитов контейнера, необходимо использовать в спецификации VPA-объекта `maxAllowed` или использовать Limit Range объект Kubernetes.
