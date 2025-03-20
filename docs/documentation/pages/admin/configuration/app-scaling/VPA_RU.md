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
  - **Auto** (по умолчанию) —  изменяет ресурсы без пересоздания подов.
  - **Recreate** — VPA может изменять ресурсы у работающих подов, перезапуская их. В случае одного пода (replicas: 1) это приведет к недоступности сервиса на время перезапуска. VPA не пересоздает поды, если они были созданы без контроллера.

- Простая генерация рекомендаций без изменения ресурсов:
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

1. Через команду deckhouse-controller (в поде `d8-system/deckhouse`):

   ```console
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable vertical-pod-autoscaler
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

- Target — рекомендуемое количество ресурсов для пода.
- Lower Bound — минимальное рекомендуемое количество ресурсов для приложения.
- Upper Bound — максимальное рекомендуемое количество ресурсов для приложения.
- Uncapped Target — значение, основанное на крайних метриках без учета истории.

## Настройка VPA

1. Создайте конфигурации модуля VPA.

   Для настройки VPA  нужно создать файл конфигурации для модуля. Пример конфигурации:

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
         node-role/example: ""
       tolerations:
       - key: dedicated
         operator: Equal
         value: example
      ```

1. Примените файл конфигурации для VPA с помощью `kubectl apply -f <имя файла конфигурации>`.

## Работа VPA с лимитами

VPA управляет запрашиваемыми ресурсами (requests) контейнера, но не управляет ограничениями (limits), если явно не указано в его политике.

VPA рассчитывает рекомендуемые значения на основе данных о потреблении ресурсов контейнером. Это поведение может повлиять на соотношение requests и limits:

- Если запрашиваемые ресурсы и лимиты равны, то VPA изменяет только ресурсы, оставляя лимиты неизменными.
- Если лимиты не указаны, VPA обновляет только ресурсы.
- Если лимиты заданы, но не управляются VPA, соотношение ресурсов/лимитов может измениться.

1. Пример 1. В кластере имеется:

   - Объект VPA:

     ```yaml
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

   - Под с ресурсами:

     ```yaml
     resources:
     limits:
       cpu: 2
     requests:
       cpu: 1
     ```

     Если контейнер будет потреблять 1 CPU, VPA порекомендует 1,168 CPU. В данном случае, соотношение между запросами и лимитами будет равно 100%. При пересоздании пода VPA изменит ресурсы на следующие:

     ```yaml
     resources:
     limits:
       cpu: 2336m
     requests:
       cpu: 1168m
     ```

1. Пример 2. В кластере имеется:

   - VPA:

     ```yaml
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

   - Под с ресурсами:

     ```yaml
     resources:
     limits:
       cpu: 1
     requests:
       cpu: 750m
     ```

     В данном случае соотношение реквестов и лимитов будет 25%. Если VPA порекомендует 1,168 CPU, ресурсы контейнера будут изменены на:

     ```yaml
     resources:
      limits:
        cpu: 1557m
      requests:
        cpu: 1168m
     ```

Если не ограничивать ресурсы, VPA может выставить слишком большие ресурсы, что может привести к проблемам.

Для предотвращения этого можно:

- Использовать параметр `maxAllowed` в VPA:

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

- Настроить [`Limit Range`](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) в кластере:

  ```yaml
  apiVersion: autoscaling.k8s.io/v1
  kind: LimitRange
  metadata:
    name: my-app-vpa
  spec:
    limits:
    - default:
        cpu: 2
        memory: 4Gi
      defaultRequest:
        cpu: 500m
        memory: 256Mi
      type: Container
  ```

## Мониторинг VPA в Grafana

Для эффективного управления ресурсами с помощью Vertical Pod Autoscaler (VPA) рекомендуется использовать [Grafana-дашборды](ссылка на раздел о графане). Они позволяют отслеживать текущий статус VPA, его настройки, а также процент подов, на которых он активирован.

Grafana предоставляет несколько уровней детализации информации о VPA. Основные дашборды:

- Main / Namespace — отображает информацию об использовании VPA на уровне пространства имен.
- Main / Namespace / Controller — детализирует данные VPA для конкретных контроллеров.
- Main / Namespace / Controller / Pod — самый детальный уровень, отображает информацию о каждом отдельном поде.

При мониторинге VPA в Grafana используются следующие ключевые столбцы:

- VPA type — показывает текущее значение `updatePolicy.updateMode`, которое определяет режим работы VPA для данного пода и отображается в дашбордах:
  - Main / Namespace
  - Main / Namespace / Controller
  - Main / Namespace / Controller / Pod

- VPA % (Процент подов с включенным VPA) — показывает процент подов в пространстве имен, на которых включен VPA. Позволяет быстро определить, сколько подов в кластере автоматически масштабируются с помощью VPA.

## Примеры настройки VPA

1. Пример минимального ресурса VerticalPodAutoscaler:

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

1. Пример полного ресурса VerticalPodAutoscaler:

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
