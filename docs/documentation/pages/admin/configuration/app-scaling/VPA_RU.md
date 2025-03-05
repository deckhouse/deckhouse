---
title: "Вертикальное масштабирование"
permalink: ru/admin/configuration/app-scaling/vpa.html
lang: ru
---

## Вертикальное масштабирование (VPA)

Vertical Pod Autoscaler (VPA) — это сервис, который помогает автоматически настраивать resource requests для контейнеров, когда точные значения этих параметров неизвестны. При использовании VPA и включении соответствующего режима работы, resource requests устанавливаются на основе фактического потребления ресурсов, полученных из Prometheus. Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, не внося изменений автоматически.

VPA поддерживает следующие режимы работы:

- **Auto** (по умолчанию) — режимы Auto и Recreate выполняют одинаковую задачу.
- **Recreate** — в этом режиме VPA может изменять ресурсы у работающих подов, перезапуская их. В случае одного пода (replicas: 1) это приведет к недоступности сервиса на время перезапуска. В этом режиме VPA не пересоздает поды, если они были созданы без контроллера.
- **Initial** — ресурсы подов изменяются только при их создании, но не в процессе работы.
- **Off** — VPA не меняет автоматически ресурсы. В этом случае можно просмотреть рекомендации по ресурсам, которые предоставляет VPA (с помощью команды `kubectl describe vpa`).

Ограничения VPA:

- Обновление ресурсов работающих подов — это экспериментальная функция. При изменении resource requests пода, под пересоздается, что может привести к его запуску на другом узле.
- VPA не рекомендуется использовать совместно с HPA для CPU и памяти в данный момент. Однако VPA можно применять с HPA для custom/external metrics.
- VPA реагирует на большинство событий out-of-memory, но не гарантирует реакцию (подробности нужно искать в документации).
- Производительность VPA не была протестирована на крупных кластерах.
- Рекомендации VPA могут превышать доступные ресурсы кластера, что может привести к тому, что поды окажутся в состоянии Pending.
- Использование нескольких VPA-ресурсов для одного пода может вызвать неопределенное поведение.
- При удалении VPA или отключении его (режим Off) изменения, внесенные VPA, сохраняются в последнем измененном виде. Это может привести к путанице, когда в Helm указаны одни ресурсы, в контроллере — другие, но на самом деле у подов будут другие ресурсы, что создаст впечатление, что они появились «непонятно откуда».

Важно! При использовании VPA рекомендуется настраивать Pod Disruption Budget.

VPA состоит из 3 компонентов:

- Recommender — мониторит настоящее (делая запросы в Metrics API, который реализован в модуле prometheus-metrics-adapter) и прошлое потребление ресурсов (делая запросы в Trickster перед Prometheus) и предоставляет рекомендации по CPU и памяти для контейнеров.
- Updater — проверяет, что у подов с VPA выставлены корректные ресурсы, если нет — убивает эти поды, чтобы контроллер пересоздал поды с новыми resource requests.
- Admission Plugin — задает resource requests при создании новых подов (контроллером или из-за активности Updater’а).

При изменении ресурсов компонентом Updater это происходит с помощью Eviction API, поэтому учитываются Pod Disruption Budget для обновляемых подов.

### Рекомендации VPA

После создания ресурса VerticalPodAutoscaler посмотреть рекомендации VPA можно с помощью команды:

```console
kubectl describe vpa my-app-vpa
```

В секции status будут такие параметры:

- Target — количество ресурсов, которое будет оптимальным для пода (в пределах resourcePolicy);
- Lower Bound — минимальное рекомендуемое количество ресурсов для более или менее (но не гарантированно) хорошей работы приложения;
- Upper Bound — максимальное рекомендуемое количество ресурсов. Скорее всего, ресурсы, выделенные сверх этого значения, идут в мусорку и совсем никогда не нужны приложению;
- Uncapped Target — рекомендуемое количество ресурсов в самый последний момент, то есть данное значение считается на основе самых крайних метрик, не смотря на историю ресурсов за весь период.

### Настройка VPA

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

1. Примените файл конфигурации для VPA с помощью `kubectl apply -f <имя файла>`.

### Работа VPA с лимитами

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

     Если вам необходимо ограничить максимальное количество ресурсов, которое может быть заданно для лимитов контейнера, необходимо использовать в спецификации VPA-объекта `maxAllowed` или использовать [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) объект Kubernetes.

### Примеры настройки VPA

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
