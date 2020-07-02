---
title: "Модуль vertical-pod-autoscaler"
permalink: /modules/302-vertical-pod-autoscaler/
search: autoscaler 
---

## Назначение

Vertical Pod Autoscaler ([VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)) — это инфраструктурный сервис, который позволяет не выставлять точные resource requests, если неизвестно, сколько ресурсов необходимо контейнеру для работы. При использовании VPA, и при включении соответствующего режима работы, resource requests выставляются автоматически на основе потребления ресурсов (полученных данных из prometheus).
Как вариант — возможно только получать рекомендации по ресурсам, без из автоматического изменения.

У VPA есть 3 режима работы:
- `"Auto"` (default) — в данный момент Auto и Recreate режимы работы делают одно и то же. Однако когда в kubernetes появится [pod inplace resource update](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/autoscaling/vertical-pod-autoscaler.md#in-place-updates), данный режим будет делать именно его.
- `"Recreate"` — данный режим разрешает VPA изменять ресурсы у запущенных подов, т.е. рестартить их при работе. В случае работы одного пода (replicas: 1) — это приведет к недоступности сервиса, на время рестарта. В данном режиме VPA не пересоздает поды, которые были созданы без контроллера.
- `"Initial"` — VPA изменяет ресурсы подов только при создании подов, но не во время работы.
- `"Off"` — VPA не изменяет автоматически никакие ресурсы. В данном случае, если есть VPA c таким режимом работы, мы можем посмотреть, какие ресурсы рекомендует поставить VPA (kubectl describe vpa <vpa-name>)

Ограничения VPA:
- Обновление ресурсов запущенных подов это экспериментальная фича VPA. Каждый раз, когда VPA обновляет `resource requests` пода, под пересоздается. Соответственно под может быть создан на другой ноде.
- VPA **не должен использоваться с [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) по cpu и memory** в данный момент. Однако VPA можно использовать с HPA на custom/external metrics.
- VPA реагирует почти на все `out-of-memory` event, но не гарантирует реакцию (почему так — выяснить из документации не удалось).
- Производительность VPA не тестировалась на огромных кластерах.
- Рекомендации VPA могут превышать доступные ресурсы в кластере, что **может приводить к подам в состоянии pending**.
- Использование нескольких VPA ресурсов над одним подом может привести к неопределенному поведению.
- В случае удаления VPA или его "выключения" (режим `Off`), изменения внесенные ранее VPA не сбрасываются, а остаются в последнем измененном значении. Из-за этого может возникнуть путаница, что в Helm будут описаны одни ресурсы, при этом в контроллере тоже будут описаны одни ресурсы, но реально у подов ресурсы будут совсем другие и может сложиться впечатление, что они взялись "непонятно откуда".

***ВАЖНО***

При использовании VPA настоятельно рекомендуем использовать [Pod Disruption Budget](https://fox.flant.com/docs/kb/blob/master/qa/pod-disruption-budget.md).

##  Конфигурация

Работает только в кластерах начиная с версии 1.11.

### Настройка модуля

У модуля есть только настройки `nodeSelector/tolerations`:
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика]({{ site.baseurl }}/#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика]({{ site.baseurl }}/#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

Пример конфига:
```yaml
verticalPodAutoscaler: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

### Настройка работы VPA для пода

> VPA работает не с контроллером пода, а с самим подом — измеряя и изменяя параметры его контейнеров.

Создать ресурс `VerticalPodAutoscaler`, в котором:
- описать `spec.targetRef`:
  - `apiVersion` — API версия объекта;
  - `kind` — тип объекта;
  - `name` — имя объекта.
- (не обязательно) указать `spec.updatePolicy.updateMode` один из — `Auto`, `Recreate`, `Initial`, `Off` (не обязательно, по умолчанию — `Auto`)
- (не обязательно) указать `resourcePolicy.containerPolicies` для конкретных контейнеров:
    - `containerName` — имя контейнера;
    - `mode` — `Auto` или `Off`, для включения или отключения работы автоскейлинга в указанном контейнере;
    - `minAllowed` — указывается минимальная граница `cpu` и `memory` для контейнера;
    - `maxAllowed` — указывается максимальная граница `cpu` и `memory` для контейнера.

Смотри [примеры](#пример-использования-vertical-pod-autoscaler).

## Grafana dashboard

На досках:
- `Main / Namespace`, `Main / Namespace / Controller`, `Main / Namespace / Controller / Pod` — столбец `VPA type` показывает значение `updatePolicy.updateMode`;
- `Main / Namespaces` — столбец `VPA %` показывает процент подов с включенным VPA.

## Как работает

VPA состоит из 3х компонентов:
- `Recommender` — он мониторит настоящее (делая запросы в [Metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md), который у нас реализован силами [prometheus-metrics-adapter]({{ site.baseurl }}/modules/301-prometheus-metrics-adapter/)) и прошлое потребление ресурсов (делая запросы в trickster перед prometheus) и предоставляет рекомендации по cpu и memory для контейнеров.
- `Updater` — Проверяет, что у подов с VPA выставлены корректные ресурсы и если нет, — убивает эти поды, чтобы контроллер пересоздал поды с новыми resource requests.
- `Admission Plugin` — Он задает resource requests при создании новых подов (контроллером или из-за активности Updater'а).

При изменении ресурсов компонентом Updater это происходит с помощью [Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#the-eviction-api), поэтому учитываются `Pod Disruption Budget` для обновляемых подов.

### Архитектура VPA

![Архитектура VPA](https://raw.githubusercontent.com/kubernetes/community/master/contributors/design-proposals/autoscaling/images/vpa-architecture.png)

## Пример использования Vertical Pod Autoscaler

Пример полного VPA объекта со всеми параметрами:

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

Пример минимального VPA объекта:

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

После деплоя данного VPA мы можем посмотреть рекомендации VPA:

```shell
kubectl describe vpa my-app-vpa
```

В секции `status` будут такие параметры:
- `Target` — Количество ресурсов, которое будет оптимальным для пода (в пределах resourcePolicy).
- `Lower Bound` — Минимальное рекомендуемое количество ресурсов для более или менее (но не гарантированно) хорошей работы приложения.
- `Upper Bound` — Максимальное рекомендуемое количество ресурсов. Скорее всего ресурсы выделенные сверх этого значения идут в мусорку и совсем никогда не нужны приложению.
- `Uncapped Target` — Рекомендуемое количество ресурсов в самый последний момент, т.е. данное значение считается на основе самых крайних метрик, не смотря на историю ресурсов за весь период.

## Как Vertical Pod Autoscaler работает с лимитами

Теперь Vertical Pod Autoscaler модифицирует лимиты контейнеров. Рассмотрим работу на примерах:

### Пример 1

У нас есть VPA объект:

```yaml
---
apiVersion: autoscaling.k8s.io/v1beta2
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

В данном случае если контейнер будет потреблять, к примеру 1 CPU целиком и VPA порекомендует данному контейнеру 1.168 CPU, то вычисляется ratio между requets и limits. В данном случае он будет равен 100%.
В этом случае, при пересоздании пода VPA модифицирует под и проставит такие ресурсы:
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
apiVersion: autoscaling.k8s.io/v1beta2
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

В данном случае соотношение requests и limits будет равным 25% и если VPA порекомендует для контейнера 1.168 CPU, то VPA изменит ресурсы контейнера таким образом:
```yaml
resources:
  limits:
    cpu: 1557m
  requests:
    cpu: 1168m
```

Если вам необходимо ограничить максимальное количество ресурсов, которое может быть заданно для limits контейнера, то необходимо использовать в спеке объекта VPA: `maxAllowed` или использовать [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) объект Kubernetes.
