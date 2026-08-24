---
title: "Классы приоритета"
permalink: ru/user/configuration/app-scaling/priority-classes.html
description: "Использование классов приоритета в Deckhouse Kubernetes Platform: примеры настройки, демонстрация вытеснения и практическая диагностика."
lang: ru
---

Класс приоритета задаёт, какие поды важнее при нехватке ресурсов на узле. Можно назначить класс в манифесте, проверить вытеснение и понять, почему под застрял в `Pending`.

## Использование класса приоритета в поде

В Deckhouse Kubernetes Platform (DKP) уже есть набор классов приоритета. В этом примере используется предустановленный класс `production-medium`.

Создайте файл `test-pod.yaml`, чтобы запустить под с классом `production-medium`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-priority-pod
spec:
  priorityClassName: production-medium
  containers:
  - name: nginx
    image: nginx
```

Примените манифест в кластере:

```shell
d8 k apply -f test-pod.yaml
```

Проверьте, что под получил нужный приоритет:

```shell
d8 k describe pod test-priority-pod | grep Priority
```

Пример вывода:

```console
Priority:             6000
Priority Class Name:  production-medium
```

{% alert level="warning" %}
Параметр `priorityClassName` нельзя изменить у работающего пода. Это поле является неизменяемым. Единственный способ изменить приоритет — удалить под и создать его заново с новым классом.
{% endalert %}

## Использование класса приоритета в Deployment

Класс приоритета можно указать в шаблоне Deployment. В этом случае все поды, созданные этим Deployment, наследуют указанный класс приоритета.

Создайте файл `deployment-with-priority.yaml`, чтобы развернуть приложение с предустановленным в DKP классом `production-high` (значение `9000`, [в разделе «Доступные классы приоритета»](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#доступные-классы-приоритета)):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      priorityClassName: production-high
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 80
```

Примените манифест в кластере:

```shell
d8 k apply -f deployment-with-priority.yaml
```

Проверьте приоритеты созданных подов:

```shell
d8 k get pods -l app=my-app -o custom-columns=NAME:.metadata.name,CLASS:.spec.priorityClassName,PRIORITY:.spec.priority
```

Пример вывода:

```console
NAME                      CLASS             PRIORITY
my-app-7d8f9b5c4f-2xq9t   production-high   9000
my-app-7d8f9b5c4f-4p7k2   production-high   9000
my-app-7d8f9b5c4f-9w5r8   production-high   9000
```

## Пошаговая демонстрация вытеснения

В этом примере показано, как планировщик вытесняет под с более низким приоритетом, когда на узле не хватает ресурсов для нового пода с более высоким приоритетом. Сначала на worker-узле запускается под с классом `develop` (`1000`), который занимает большую часть свободных ресурсов. Затем создаётся под с классом `production-high` (`9000`) с такими же `requests` — места на узле уже нет, и планировщик вытесняет под с низким приоритетом в пользу нового.

В примерах вместо `worker-0` укажите имя своего worker-узла.

{% alert level="info" %}
Пример рассчитан на один worker-узел с 4 CPU и 8 Gi памяти. Если узлов несколько, поды могут разместиться на разных узлах, и вытеснения не будет. Подберите `requests` так, чтобы под с низким приоритетом занял большую часть свободных ресурсов узла.
{% endalert %}

Проверьте список узлов в кластере:

```shell
d8 k get nodes
```

Пример вывода:

```console
NAME        STATUS   ROLES                  AGE   VERSION
master-0    Ready    control-plane,master   14d   v1.34.9
worker-0    Ready    worker                 14d   v1.34.9
```

Проверьте ресурсы на worker-узле:

```shell
d8 k describe node worker-0 | grep -E "Capacity|Allocatable|Allocated" -A 5
```

Пример вывода:

```console
Capacity:
  cpu:                4
  memory:             8174932Ki
Allocatable:
  cpu:                3800m
  memory:             7174932Ki
Allocated resources:
  cpu:                1200m (31%)
  memory:             2Gi (28%)
```

Создайте под с низким приоритетом, который займёт значительную часть свободных ресурсов, а затем под с высоким приоритетом, которому не хватит оставшегося места.

Создайте файл `low-priority-pod.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: low-priority-pod
spec:
  priorityClassName: develop
  containers:
  - name: app
    image: nginx
    resources:
      requests:
        cpu: "2"
        memory: "4Gi"
```

Примените манифест:

```shell
d8 k apply -f low-priority-pod.yaml
```

Дождитесь, пока под перейдёт в статус `Running`:

```shell
d8 k get pods low-priority-pod
```

Пример вывода:

```console
NAME               READY   STATUS    RESTARTS   AGE
low-priority-pod   1/1     Running   0          10s
```

Создайте файл `high-priority-pod.yaml`, чтобы запустить под с высоким приоритетом, который запросит больше ресурсов, чем осталось свободно на узле:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: high-priority-pod
spec:
  priorityClassName: production-high
  containers:
  - name: app
    image: nginx
    resources:
      requests:
        cpu: "2"
        memory: "4Gi"
```

Примените манифест:

```shell
d8 k apply -f high-priority-pod.yaml
```

Проверьте статус подов:

```shell
d8 k get pods | grep priority
```

Пример вывода:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   1/1     Running   0          5s
```

Под `high-priority-pod` находится в статусе `Running`. Под `low-priority-pod` может отсутствовать в выводе, так как планировщик вытеснил его за считанные секунды. В некоторых случаях можно увидеть `low-priority-pod` в статусе `Pending`.

Проверьте события вытеснения:

```shell
d8 k get events -A --field-selector reason=Preempted
```

Пример вывода:

```console
NAMESPACE   LAST SEEN   TYPE     REASON      OBJECT                 MESSAGE
default     68s         Normal   Preempted   pod/low-priority-pod   Preempted by pod d9d25b95-4a7d-4214-8a30-8ce1fd616f67 on node worker-0
```

## Разделение окружений по приоритетам

Неймспейс сам по себе не влияет на вытеснение: планировщик сравнивает только классы приоритета подов, независимо от того, в каких неймспейсах они находятся. Чтобы защитить production-нагрузку, в тестовых и develop-окружениях указывайте более низкий класс приоритета (`develop`, `staging`), а в production — более высокий (`production-low`, `production-medium`, `production-high`).

## Защита Stateful-приложений

Stateful-приложения (с сохранением состояния, например базы данных, очереди сообщений) хранят данные в памяти или в постоянных томах (PVC). Их защита требует особого подхода, так как внезапное уничтожение пода без корректного завершения работы может повредить данные, а массовое вытеснение реплик приводит к потере доступности сервиса.

Подробнее о механизмах защиты от вытеснения смотрите [в разделе «Защита от вытеснения»](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#защита-от-вытеснения). В подразделе «Демонстрация защиты Stateful-приложения» показан практический сценарий, в котором эти механизмы работают вместе.

Для защиты Stateful-приложений в данном примере используется комбинация трёх механизмов:

- Высокий `priorityClassName` — в примере у StatefulSet указан `production-medium` (`6000`). Для реальных критичных Stateful-приложений обычно берут `production-high` (`9000`) и выше, см. [раздел «Доступные классы приоритета»](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#доступные-классы-приоритета). Так поды реже становятся кандидатами на вытеснение.
- PodDisruptionBudget (PDB) гарантирует минимальное количество работающих реплик (например, `minAvailable: 5`).
- Параметр `terminationGracePeriodSeconds` задаёт время на запись данных на диск и закрытие транзакций перед завершением пода (рекомендуется 30–60 секунд).

### Демонстрация защиты Stateful-приложения

В этом примере разворачивается учебное Stateful-приложение: ему назначается класс `production-medium` (`6000`) вместе с PodDisruptionBudget и `terminationGracePeriodSeconds`, чтобы ограничить масштаб вытеснения и дать подам время корректно завершить работу. Затем создаётся под `emergency-task` с более высоким классом `production-high` (`9000`) и большим запросом памяти — чтобы смоделировать критическую нехватку ресурсов на узле и проверить, как срабатывают механизмы защиты.

#### Шаг 1. Создание защищённого StatefulSet с PDB

Создайте файл `stateful-protect.yaml`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mock-stateful
spec:
  serviceName: mock-stateful
  replicas: 7
  selector:
    matchLabels:
      app: mock-stateful
  template:
    metadata:
      labels:
        app: mock-stateful
    spec:
      priorityClassName: production-medium
      terminationGracePeriodSeconds: 30
      containers:
      - name: app
        image: busybox
        command:
        - sh
        - -c
        - |
          trap 'echo ">>> НАЧАЛО: Сохранение данных на диск..."; sleep 10; echo ">>> КОНЕЦ: Данные сохранены, выход."' TERM
          echo "Приложение запущено и работает..."
          while true; do sleep 1; done
        resources:
          requests:
            cpu: "100m"
            memory: "256Mi"
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: mock-stateful-pdb
spec:
  minAvailable: 5
  selector:
    matchLabels:
      app: mock-stateful
```

Примените конфигурацию:

```shell
d8 k apply -f stateful-protect.yaml
```

Дождитесь статуса `Running` для всех 7 подов:

```shell
d8 k get pods -l app=mock-stateful -w
```

#### Шаг 2. Симуляция нехватки ресурсов

Создайте файл `emergency-task.yaml`, чтобы запустить под с предустановленным классом `production-high` (значение `9000`), который выше, чем у Stateful-приложения (`production-medium`, значение `6000`):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: emergency-task
spec:
  priorityClassName: production-high
  containers:
  - name: task
    image: busybox
    command: ["sleep", "infinity"]
    resources:
      requests:
        cpu: "1"
        memory: "5Gi"
```

Примените манифест:

```shell
d8 k apply -f emergency-task.yaml
```

#### Шаг 3. Наблюдение за работой механизмов защиты

Проверьте статус подов:

```shell
d8 k get pods | grep -E 'mock-stateful|emergency-task'
```

Ожидаемый пример вывода (процесс вытеснения):

```console
NAME                      READY   STATUS        RESTARTS   AGE
emergency-task            0/1     Pending       0          5s
mock-stateful-0           1/1     Terminating   0          55s
mock-stateful-1           1/1     Terminating   0          53s
mock-stateful-2           1/1     Running       0          51s
mock-stateful-3           1/1     Running       0          49s
mock-stateful-4           1/1     Running       0          47s
mock-stateful-5           1/1     Running       0          45s
mock-stateful-6           1/1     Terminating   0          43s
```

{% alert level="warning" %}
Логи вытесняемых подов доступны только пока под находится в статусе `Terminating`. Чтобы увидеть процесс корректного завершения работы, необходимо успеть выполнить команду `d8 k logs` до полного удаления пода.
{% endalert %}

Проверьте логи завершающегося пода:

```shell
d8 k logs mock-stateful-0 --tail=20
```

Ожидаемый пример вывода:

```console
Приложение запущено и работает...
>>> НАЧАЛО: Сохранение данных на диск...
>>> КОНЕЦ: Данные сохранены, выход.
```

### Как работают механизмы защиты в критической ситуации

Поскольку приоритет созданного [в «Демонстрации защиты Stateful-приложения»](#демонстрация-защиты-stateful-приложения) пода `emergency-task` (`9000`, класс `production-high`) выше, чем у Stateful-приложения (`6000`, класс `production-medium`), и других кандидатов на вытеснение нет, планировщик вынужден выбрать Stateful-приложение для вытеснения. При этом срабатывают защитные механизмы:

1. В данном случае PodDisruptionBudget пытается ограничить масштаб ущерба, но поскольку запрос экстремально велик, планировщик нарушает PDB, хоть и пытается минимизировать количество удаляемых подов.
1. Параметр `terminationGracePeriodSeconds` гарантирует сохранность данных, даже в случае полного вытеснения.

## Эксплуатация и диагностика

В этом разделе приведены практические команды для проверки состояния подов и событий планировщика. Описание причин проблем и возможных действий на уровне кластера приведено [в административном разделе «Эксплуатация и диагностика»](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#эксплуатация-и-диагностика).

[В «Пошаговой демонстрации вытеснения»](#пошаговая-демонстрация-вытеснения) под `high-priority-pod` переходит в `Running`. В следующих подразделах тот же под разбирается в двух случаях, когда он остаётся в `Pending`: не хватает CPU или памяти, и планировщик не находит поды для вытеснения.

### Под не запускается из-за нехватки ресурсов

Если в кластере нет свободных ресурсов и вытеснение невозможно, под останется в состоянии `Pending`.

Проверьте статус пода:

```shell
d8 k get pod high-priority-pod
```

Пример вывода:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   0/1     Pending   0          2m
```

Проверьте события пода:

```shell
d8 k describe pod high-priority-pod | grep -A10 "Events:"
```

Ищите в разделе `Events` сообщения вида:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  2m    default-scheduler  0/3 nodes are available: 2 Insufficient cpu, 1 Insufficient memory. preemption: 0/3 nodes are available: 3 Preemption is not helpful for scheduling.
```

Что означают эти сообщения:

- `Insufficient cpu` или `Insufficient memory` — на узлах не хватает запрошенных ресурсов, соответственно CPU и память.
- `Preemption is not helpful for scheduling` — вытеснение существующих подов не освободит достаточно ресурсов (например, все поды имеют равный или более высокий приоритет).

Проверьте доступные ресурсы на узлах:

```shell
d8 k describe nodes | grep -A 5 "Allocated resources"
```

Пример вывода:

```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests    Limits
  --------           --------    ------
  cpu                3800m (95%)  4200m (105%)
  memory             7Gi (87%)    8Gi (100%)
```

Возможные решения:

- Уменьшите `requests` в манифесте пода.
- Добавьте ресурсы в кластер (новые узлы или увеличение существующих).
- Удалите неиспользуемые поды.

### Практическая диагностика невозможности вытеснения

Если вытеснение подов с низким приоритетом не происходит, хотя ресурсы свободны, проверьте события с причиной `FailedPreemption`:

```shell
d8 k get events -A --field-selector reason=FailedPreemption --sort-by='.metadata.creationTimestamp'
```

Пример вывода:

```console
NAMESPACE   LAST SEEN   TYPE      REASON             OBJECT                    MESSAGE
default     30s         Warning   FailedPreemption   pod/high-priority-pod     no preemption victims found for pod
```

Сообщение `no preemption victims found for pod` обычно значит, что на узле нет подов с более низким приоритетом — их нельзя вытеснить ради нового пода. Разбор такого случая — [в разделе «Практическая проверка отсутствия подходящих подов»](#практическая-проверка-отсутствия-подходящих-подов).

Под может не запуститься и по причинам, не связанным с классом приоритета: например, нет подходящих узлов из-за taint и tolerations. Тогда в событиях будет `FailedScheduling` с сообщениями вроде `untolerated taint(s)`, а не `FailedPreemption`.

Возможные действия на уровне кластера смотрите [в разделе «Эксплуатация и диагностика»](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#эксплуатация-и-диагностика).

### Практическая проверка отсутствия подходящих подов

В этом примере используется состояние кластера [после «Демонстрации защиты Stateful-приложения»](#демонстрация-защиты-stateful-приложения). Под `mock-stateful-0` имеет приоритет `6000`. На целевом узле остальные поды `mock-stateful` имеют такой же приоритет, `emergency-task` — более высокий (`9000`), а системные поды — ещё более высокий. Поэтому планировщик не может вытеснить ни один из них ради запуска `mock-stateful-0`: подходящих подов с более низким приоритетом на узле нет.

Если в событиях есть сообщение `No preemption victims found for incoming pod`, проверьте приоритеты подов на узле:

```shell
d8 k get pods --all-namespaces -o wide --field-selector spec.nodeName=worker-0 -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,PRIORITY:.spec.priority --sort-by=.spec.priority
```

Пример вывода:

```console
NAME                                                            NODE       PRIORITY
mock-stateful-5                                                 worker-0   6000
mock-stateful-2                                                 worker-0   6000
mock-stateful-3                                                 worker-0   6000
mock-stateful-4                                                 worker-0   6000
emergency-task                                                  worker-0   9000
multitenancy-manager-5968799d76-ktjgl                           worker-0   2000000000
csi-node-s82x4                                                  worker-0   2000001000
agent-wqzxq                                                     worker-0   2000001000
early-oom-6tkzg                                                 worker-0   2000001000
safe-agent-updater-ntrzq                                        worker-0   2000001000
kubernetes-api-proxy-worker-0                                   worker-0   2000001000
node-exporter-cddjm                                             worker-0   2000001000
oom-kills-exporter-pplfk                                        worker-0   2000001000
```

Проверьте точную причину, почему `mock-stateful-0` не смог вытеснить другие поды:

```shell
d8 k describe pod mock-stateful-0 | grep -A10 "Events:"
```

Пример сообщения из событий пода:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  13s   default-scheduler  0/2 nodes are available: preemption: 0/2 nodes are available: 1 No preemption victims found for incoming pod
```

| Сообщение | Значение |
|-----------|----------|
| `0/2 nodes are available` | В кластере есть 2 узла, но ни один не подходит для размещения пода. |
| `1 No preemption victims found for incoming pod` | На узле с нехваткой памяти нет подов с более низким приоритетом, которые можно было бы вытеснить. |

Что делать на уровне кластера, смотрите [в разделе «Эксплуатация и диагностика»](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#эксплуатация-и-диагностика).

### Практический пример «Лимит подов на узле»

Даже если освободились CPU и память, лимит на максимальное количество подов на узле может помешать запуску пода с высоким приоритетом.

Проверьте лимит подов на узле:

```shell
d8 k describe node worker-0 | grep pods -A2
```

Пример вывода:

```console
Capacity:
  pods:  120
```

Проверьте текущее количество подов на узле:

```shell
d8 k get pods --all-namespaces -o wide | grep worker-0 | wc -l
```

Пример вывода:

```console
64
```

Сейчас на узле 64 пода из 120. Свободное место ещё есть.

Создайте файл `pod-filler.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pod-filler
spec:
  replicas: 110
  selector:
    matchLabels:
      app: filler
  template:
    metadata:
      labels:
        app: filler
    spec:
      priorityClassName: develop
      containers:
      - name: filler
        image: busybox
        command: ["sleep", "infinity"]
        resources:
          requests:
            cpu: "1m"
            memory: "5Mi"
```

Примените манифест:

```shell
d8 k apply -f pod-filler.yaml
```

Дождитесь, пока Deployment заполнит узел:

```shell
d8 k get pods -l app=filler -o wide | grep worker-0 | wc -l
```

Создайте файл `high-priority-limit-pod.yaml`, чтобы запустить под с высоким приоритетом:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: high-priority-limit-pod
spec:
  priorityClassName: production-high
  containers:
  - name: app
    image: nginx
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"
```

Примените манифест:

```shell
d8 k apply -f high-priority-limit-pod.yaml
```

Проверьте статус пода с высоким приоритетом:

```shell
d8 k get pod high-priority-limit-pod
```

Пример вывода:

```console
NAME                      READY   STATUS    RESTARTS   AGE
high-priority-limit-pod   0/1     Pending   0          11s
```

Посмотрите причину в событиях:

```shell
d8 k describe pod high-priority-limit-pod | grep -A10 "Events:"
```

Пример вывода:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  11s   default-scheduler  0/2 nodes are available: 1 Too many pods, 1 node(s) had untolerated taint(s).
```

Проверьте, что вытеснения не произошло:

```shell
d8 k get events -A --field-selector reason=Preempted
```

Пример вывода (пусто):

```console
No resources found
```

Суть проблемы: лимит подов на узле уже достигнут, поэтому даже под с высоким приоритетом не может запуститься. Вытеснение существующих подов не помогает, так как количество подов останется прежним (вытесненный под будет заменён новым).

### Полезные команды для мониторинга приоритетов

Посчитайте количество подов по классам приоритетов:

```shell
d8 k get pods -A -o jsonpath='{range .items[*]}{.spec.priorityClassName}{"\n"}{end}' | sort | uniq -c | sort -rn
```

Ожидаемый пример вывода:

```console
     34 cluster-medium
     30 cluster-low
     26 system-cluster-critical
     18 system-node-critical
      6 production-high
```

Просмотрите события вытеснения во всех неймспейсах:

```shell
d8 k get events -A --field-selector reason=Preempted -o custom-columns=NAMESPACE:.metadata.namespace,POD:.involvedObject.name,MESSAGE:.message
```

Ожидаемый пример вывода:

```console
NAMESPACE          POD                                                 MESSAGE
d8-chrony          chrony-master-9wzbl                                 Preempted by pod ac651aed-... on node master-0
d8-console         backend-58f9989c9d-4svjw                            Preempted by pod ac651aed-... on node master-0
d8-monitoring      prometheus-main-0                                   Preempted by pod 91f6e071-... on node worker-0
default            log-collector-dlxpv                                 Preempted by pod 91f6e071-... on node worker-0
```

{% alert level="info" %}
События хранятся ограниченное время (обычно около часа). Если вытеснения давно не было, эти команды могут ничего не вернуть — повторите демонстрацию вытеснения и выполните команды сразу после неё.
{% endalert %}

Проверьте, какие поды вытеснялись чаще всего:

```shell
d8 k get events -A --field-selector reason=Preempted -o jsonpath='{range .items[*]}{.involvedObject.name}{"\n"}{end}' | sort | uniq -c | sort -rn | head -10
```

Ожидаемый пример вывода:

```console
      2 prometheus-main-0
      2 memcached-0
      1 user-api-77494dc777-jzp7p
      1 upmeter-dex-authenticator-7f54c8dfb4-wwv22
      1 upmeter-dex-authenticator-7f54c8dfb4-wqfn5
      1 upmeter-dex-authenticator-7f54c8dfb4-h6784
      1 upmeter-dex-authenticator-7f54c8dfb4-bxsvk
      1 upmeter-dex-authenticator-7f54c8dfb4-28fgt
      1 upmeter-agent-4chrw
      1 status-dex-authenticator-786c6cc554-mfsdw
```
