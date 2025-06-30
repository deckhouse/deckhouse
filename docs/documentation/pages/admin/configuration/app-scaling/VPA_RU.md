---
title: "Вертикальное масштабирование"
permalink: ru/admin/configuration/app-scaling/vpa.html
lang: ru
---

## Как работает вертикальное масштабирование (VPA)

Vertical Pod Autoscaler (VPA) позволяет автоматизировать управление ресурсами контейнеров и значительно повысить эффективность работы приложений. VPA полезен в ситуациях, когда заранее неизвестно, сколько ресурсов потребуется приложению. При использовании VPA и включении соответствующего режима работы, запрашиваемые ресурсы устанавливаются на основе фактического потребления ресурсов, полученных [от системы мониторинга](ссылка на раздел о мониторинге). Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, не внося изменений автоматически.

Если нагрузка на приложение меняется в зависимости от времени суток, запросов пользователей или других факторов, VPA автоматически подстраивает выделенные ресурсы. Это позволяет избежать падений из-за нехватки ресурсов или чрезмерного расходования CPU и памяти.

## Режимы работы VPA

VPA может работать в двух режимах:

- Автоматическое изменение запросов ресурсов:
  - **Auto** (по умолчанию) —  изменяет ресурсы без пересоздания подов. В текущих версиях Kubernetes этот режим ведёт себя так же, как и **Recreate**: при необходимости изменения ресурсов VPA перезапускает под. Однако в будущем, с появлением [in-place обновления ресурсов](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates), режим **Auto** будет использовать его — то есть изменять ресурсы без перезапуска пода.
  - **Recreate** — VPA может изменять ресурсы у работающих подов, перезапуская их. В случае одного пода (replicas: 1) это приведет к недоступности сервиса на время перезапуска. VPA не пересоздает поды, если они были созданы без контроллера.

- Только рекомендации, без изменения ресурсов:
  - **Initial** — ресурсы подов изменяются только при их создании, но не в процессе работы.
  - **Off** — VPA не меняет автоматически ресурсы.Однако, с его помощью можно просматривать рекомендуемые ресурсы с помощью команды `kubectl describe vpa`.

При использовании VPA и включении соответствующего режима, запрашиваемые ресурсы устанавливаются автоматически на основе данных из Prometheus. Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, но не изменяла их.

## Как включить или отключить VPA

Включить VPA можно следующими способами:

1. Через ресурс ModuleConfig (например, ModuleConfig/vertical-pod-autoscaler). Установите параметр `spec.enabled` в значение `true` или `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     enabled: true
   ```

1. Через команду `d8` (в поде `d8-system/deckhouse`):

   ```console
   d8 platform module enable vertical-pod-autoscaler
   ```

1. Через [веб-интерфейс Deckhouse](https://deckhouse.ru/products/kubernetes-platform/modules/console/stable/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `vertical-pod-autoscaler` и нажмите на него;
   - Включите тумблер «Модуль включен».

У модуля нет обязательных настроек, то есть можно включить его и не настраивать дополнительно. При этом он будет работать со значениями по умолчанию.

После создания ресурса VerticalPodAutoscaler посмотреть рекомендации VPA можно с помощью команды:

```console
kubectl describe vpa my-app-vpa
```

В секции `status` будут такие параметры:

- `Target` — рекомендуемое количество ресурсов для пода.
- `Lower Bound`— минимальное рекомендуемое количество ресурсов для приложения.
- `Upper Bound` — максимальное рекомендуемое количество ресурсов для приложения.
- `Uncapped Target`— значение, основанное на крайних метриках без учета истории.

## Настройка VPA

1. Создайте конфигурации модуля VPA.

   Для настройки VPA нужно создать файл конфигурации для модуля. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     version: 1
     enabled: true
     settings:
       nodeSelector:
         node-role/system: ""
       tolerations:
       - key: dedicated.deckhouse.io
         operator: Equal
         value: system
      ```

1. Примените файл конфигурации для VPA с помощью `kubectl apply -f <имя файла конфигурации>`.

Подробнее про настройку лимитов VPA [см. в разделе Использование](../../../user/configuration/app-scaling/vpa.html#работа-vpa-с-лимитами).

### Примеры настройки

#### Настройка модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: vertical-pod-autoscaler
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/system: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: system
```

#### Пример минимального ресурса VerticalPodAutoscaler

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: StatefulSet
    name: my-app
```

#### Пример полного ресурса VerticalPodAutoscaler

```yaml
apiVersion: autoscaling.k8s.io/v1
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

## Как Vertical Pod Autoscaler работает с лимитами

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
