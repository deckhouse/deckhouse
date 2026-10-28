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
- `Uncapped Target` — значение target, рассчитанное только по потреблению ресурсов, без ограничений `resourcePolicy` (`minAllowed`, `maxAllowed`) и без `recommendationFactor`.

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

## Как заставить VPA рекомендовать больше или меньше измеренного?

VPA рассчитывает рекомендацию по измеренному потреблению ресурсов и добавляет собственный запас. Чтобы VPA рекомендовал больше или меньше, задайте поле `recommendationFactor` в политике контейнера объекта [VerticalPodAutoscaler](cr.html#verticalpodautoscaler). VPA умножает рассчитанную рекомендацию на указанное значение. Например, `1.2` увеличивает рекомендацию на 20%, а `0.87` уменьшает на 13%.

Задайте множители в объекте VerticalPodAutoscaler. Политика с `containerName: "*"` применяется ко всем контейнерам, у которых нет собственной политики:

```yaml
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: my-app
  resourcePolicy:
    containerPolicies:
    - containerName: "*"
      recommendationFactor:
        cpu: "1.2"
        memory: "0.87"
```

Значения `cpu` и `memory` задаются как множитель в виде строки, не больше двух знаков после запятой, например `"1.25"`. Допустимый диапазон: больше нуля и не больше `10`. Если ключ не указан, рекомендация для этого объекта не меняется.

VPA применяет множитель после вычисления собственного запаса и до применения ограничений `minAllowed` и `maxAllowed`, поэтому эти ограничения по-прежнему действуют на итоговую рекомендацию. В `status` объекта VPA поля `target`, `lowerBound` и `upperBound` уже содержат множитель. Поле `uncappedTarget` показывает target, рассчитанный без него.

Лимиты пересчитываются пропорционально запросам, как описано в разделе [Как Vertical Pod Autoscaler работает с лимитами?](#как-vertical-pod-autoscaler-работает-с-лимитами).

{% alert level="warning" %}
Множитель меньше `1` для `memory` повышает риск завершения контейнера по событию OOMKilled, потому что уменьшает запас, который VPA оставляет над измеренным потреблением.
{% endalert %}

При изменении множителя меняется рекомендация, поэтому компонент updater может обновить поды приложения по своим обычным правилам.

Для DaemonSet с `spec.scope` VPA хранит рекомендации в `status.groups` в компактном виде, который содержит только `target` для каждого контейнера. Значение без множителя там не показывается.
