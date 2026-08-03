---
title: "Модуль deckhouse: примеры"
description: "Практические примеры создания и использования PriorityClass, механизм вытеснения подов и диагностика связанных проблем."
---

## Примеры создания и использования PriorityClass

PriorityClass — сопоставляет имя класса приоритета с целочисленным значением. Чем выше значение, тем выше приоритет пода. Этот механизм используется планировщиком для принятия решений о размещении подов и их вытеснении в условиях нехватки ресурсов на узлах.

При нехватке ресурсов на узле планировщик вытесняет поды с меньшим приоритетом, чтобы освободить место для подов с большим приоритетом.

В Deckhouse Kubernetes Platform (DKP) предустановлен набор классов приоритета. Полный перечень, рекомендуемое назначение классов и ограничения по использованию смотрите в разделе [Priority Classes](./#priority-classes).

{% alert level="info" %}
Не создавайте пользовательские классы со значениями выше 1 000 000, чтобы не нарушить работу критически важных системных компонентов.
{% endalert %}

### Создание PriorityClass

Создайте файл `my-priority.yaml` со следующим содержимым — PriorityClass с именем `my-app-critical` и приоритетом `8000`:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: my-app-critical
value: 8000
globalDefault: false
description: "Приоритет для критичных приложений"
```

Примените манифест в кластере:

```shell
d8 k apply -f my-priority.yaml
```

Проверьте создание ресурса:

```shell
d8 k get priorityclass my-app-critical
```

Пример вывода:

```console
NAME              VALUE   GLOBAL-DEFAULT   AGE   PREEMPTIONPOLICY
my-app-critical   8000    false            7s    PreemptLowerPriority
```

В манифесте PriorityClass используются следующие поля:

- `metadata.name` — имя класса. Указывается в подах в параметре `priorityClassName`;
- `value` — числовой приоритет. Чем выше значение, тем выше приоритет;
- `globalDefault` — определяет, будет ли этот класс использоваться по умолчанию для подов без явного `priorityClassName`;
- `description` — описание класса.

{% alert level="warning" %}
Будьте осторожны с параметром `globalDefault: true`. Если установить его для пользовательского класса, все поды в кластере без явного приоритета получат это значение, что может привести к непредсказуемому вытеснению системных компонентов.
{% endalert %}

### Использование PriorityClass в поде

Создайте файл `test-pod.yaml`, чтобы запустить под с классом `my-app-critical` из раздела [Создание PriorityClass](#создание-priorityclass):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-priority-pod
spec:
  priorityClassName: my-app-critical
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
Priority:             8000
Priority Class Name:  my-app-critical
```

{% alert level="warning" %}
Параметр `priorityClassName` нельзя изменить у работающего пода. Это поле является неизменяемым. Единственный способ изменить приоритет — удалить под и создать его заново с новым классом.
{% endalert %}

### Использование PriorityClass в Deployment

PriorityClass можно указать в шаблоне Deployment. В этом случае все поды, созданные этим Deployment, наследуют указанный класс приоритета.

Создайте файл `deployment-with-priority.yaml`, чтобы развернуть приложение с предустановленным в DKP классом `production-high` (значение `9000`, раздел [Priority Classes](./#priority-classes)):

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

## Механизм вытеснения

### Принцип работы

Решение о вытеснении принимается планировщиком исключительно на основе числового значения приоритета. Тип рабочей нагрузки (Deployment, StatefulSet, DaemonSet или обычный под) не имеет значения — планировщик сравнивает только числа: под с большим значением приоритета может вытеснить под с меньшим, чтобы освободить ресурсы на узле.

При равных приоритетах вытеснение не происходит. Если новый под имеет такой же приоритет, как и существующие поды на узле, планировщик не будет вытеснять их — новый под останется в статусе `Pending` до появления свободных ресурсов.

### Пошаговая демонстрация вытеснения

В примерах ниже вместо `worker-0` укажите имя своего worker-узла.

{% alert level="info" %}
Поведение зависит от количества worker-узлов и может отличаться от приведённого в примере: демонстрация рассчитана на кластер с одним worker-узлом. Если worker-узлов несколько, поды могут быть распределены по разным узлам, и вытеснение на одном узле может не произойти.
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

В этом примере на узле свободно примерно 2600m CPU и 5Gi памяти. Создайте под с низким приоритетом, который займёт значительную часть этих ресурсов, а затем под с высоким приоритетом, которому не хватит оставшегося места.

{% alert level="info" %}
Значения `requests` в этом примере (2 CPU и 4Gi памяти) подобраны для узла с 4 CPU и 8Gi памяти, где уже занято около 30% ресурсов. Если в кластере узлы мощнее или слабее, адаптируйте эти значения так, чтобы под с низким приоритетом занял примерно 70–80% свободных ресурсов узла.
{% endalert %}

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
d8 k get events --field-selector reason=Preempted
```

Пример вывода:

```console
LAST SEEN   TYPE     REASON      OBJECT                 MESSAGE
68s         Normal   Preempted   pod/low-priority-pod   Preempted by pod d9d25b95-4a7d-4214-8a30-8ce1fd616f67 on node worker-0
```

## Защита от вытеснения

Единственный способ защитить под от вытеснения — назначить ему достаточно высокий приоритет. Однако даже при высоком приоритете есть риск быть вытесненным, если в кластере появится под с ещё более высоким приоритетом.

Для снижения рисков при вытеснении используется комбинация трёх механизмов. Эти механизмы универсальны и применимы к любым критичным приложениям:

1. Высокий `priorityClassName` — делает под менее вероятным кандидатом на вытеснение.
1. PodDisruptionBudget (PDB) — ограничивает количество одновременно удаляемых реплик. При вытеснении PDB работает как рекомендация (в отличие от плановых работ, таких как `d8 k drain`, где PDB является жёстким ограничением).
1. `terminationGracePeriodSeconds` — даёт приложению время на сброс буферов и закрытие соединений перед принудительным удалением. Это последняя линия обороны, гарантирующая целостность данных даже при нарушении PDB.

{% alert level="info" %}
Без параметра `terminationGracePeriodSeconds` мгновенное завершение работы привело бы к потере данных, которые не были сохранены на диск, повреждению базы данных и необходимости длительной проверки файловой системы при следующем запуске. Корректное завершение работы позволяет новым подам перемонтировать PVC и восстановиться из сохранённого состояния.
{% endalert %}

Подробнее о практическом применении этих механизмов для защиты Stateful-приложений смотрите в разделе [Защита Stateful-приложений](#защита-stateful-приложений).

## Архитектурные сценарии использования

### Разделение окружений по приоритетам

Разделение окружений только по неймспейсам не защищает критичные сервисы от нехватки ресурсов на уровне всего кластера. Использование классов приоритета позволяет выстроить чёткую иерархию потребления ресурсов.

Предположим, что узел кластера полностью загружен фоновой задачей из окружения разработки. В этот момент в production-окружении требуется масштабировать критичный сервис. Благодаря разнице в приоритетах (`9000` у `production-high` против `1000` у `develop`) планировщик автоматически вытеснит задачу из окружения разработки, чтобы освободить ресурсы для критичного production-сервиса.

{% alert level="info" %}
Без изменения приоритетов обратная ситуация — вытеснение production-сервиса задачами разработки — невозможна.
{% endalert %}

В окружении разработки создайте файл `dev-data-processor.yaml`, чтобы запустить фоновую задачу с предустановленным классом `develop`:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: dev-data-processor
spec:
  schedule: "0 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          priorityClassName: develop
          containers:
          - name: processor
            image: busybox:latest
            command: ["sleep", "infinity"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
              limits:
                cpu: "2"
                memory: "4Gi"
          restartPolicy: OnFailure
```

Примените манифест:

```shell
d8 k apply -f dev-data-processor.yaml
```

{% alert level="info" %}
Команда `command: ["sleep", "infinity"]` используется как учебная заглушка. Она заставляет контейнер работать бесконечно, гарантированно удерживая запрошенные ресурсы, что необходимо для надёжной демонстрации нехватки ресурсов на узле без развёртывания реального приложения.
{% endalert %}

Создайте задачу вручную для немедленного запуска:

```shell
d8 k create job --from=cronjob/dev-data-processor manual-test
```

Проверьте, что под запустился:

```shell
d8 k get pods | grep manual-test
```

Пример вывода:

```console
manual-test-mh9f4                   1/1     Running   0          15h
```

В production-окружении создайте файл `prod-api.yaml` и запустите критичный сервис с высоким приоритетом, используя предустановленный класс `production-high`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prod-api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: prod-api
  template:
    metadata:
      labels:
        app: prod-api
    spec:
      priorityClassName: production-high
      containers:
      - name: api
        image: busybox:latest
        command: ["sleep", "infinity"]
        resources:
          requests:
            cpu: "2"
            memory: "4Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
```

Примените манифест:

```shell
d8 k apply -f prod-api.yaml
```

Проверьте события, чтобы убедиться, что вытеснение произошло:

```shell
d8 k get events --field-selector reason=Preempted --sort-by='.lastTimestamp'
```

Пример вывода:

```console
LAST SEEN   TYPE     REASON      OBJECT                  MESSAGE
15s         Normal   Preempted   pod/manual-test-tnksn   Preempted by pod 3205b347-705d-49b0-9a20-a1e56f51cb7e on node worker-0
```

## Защита Stateful-приложений

Stateful-приложения (с сохранением состояния, например базы данных, очереди сообщений) хранят данные в памяти или в постоянных томах (PVC). Их защита требует особого подхода, так как внезапное уничтожение пода без корректного завершения работы может повредить данные, а массовое вытеснение реплик приводит к потере доступности сервиса.

Для защиты Stateful-приложений в данном примере используется комбинация трёх механизмов из раздела [Защита от вытеснения](#защита-от-вытеснения):

- Высокий `priorityClassName` (рекомендуется `production-high` со значением `9000` и выше, раздел [Priority Classes](./#priority-classes)) делает поды менее вероятными кандидатами на вытеснение.
- PodDisruptionBudget гарантирует минимальное количество работающих реплик (например, `minAvailable: 5`).
- Параметр `terminationGracePeriodSeconds` задаёт время на запись данных на диск и закрытие транзакций перед завершением пода (рекомендуется 30–60 секунд).

### Демонстрация защиты Stateful-приложения

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
      priorityClassName: production-high
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

#### Шаг 2. Создание класса приоритета выше, чем у приложения

Создайте файл `super-critical-pc.yaml`:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: super-critical
value: 10000
description: "Максимальный приоритет для экстренных задач"
```

Примените его:

```shell
d8 k apply -f super-critical-pc.yaml
```

#### Шаг 3. Симуляция нехватки ресурсов

Создайте файл `emergency-task.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: emergency-task
spec:
  priorityClassName: super-critical
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

#### Шаг 4. Наблюдение за работой механизмов защиты

Проверьте статус подов:

```shell
d8 k get pods -l app=mock-stateful
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

Поскольку приоритет `emergency-task` (10000) выше, чем у Stateful-приложения (9000) и других кандидатов на вытеснение нет, планировщик вынужден выбрать Stateful-приложение для вытеснения. При этом срабатывают защитные механизмы, описанные в разделе [Защита от вытеснения](#защита-от-вытеснения):

1. В данном случае PodDisruptionBudget пытается ограничить масштаб ущерба, но поскольку запрос экстремально велик, планировщик нарушает PDB, хоть и пытается минимизировать количество удаляемых подов.
1. Параметр `terminationGracePeriodSeconds` гарантирует сохранность данных, даже в случае полного вытеснения.

## Отличия от других механизмов управления ресурсами

PriorityClass часто путают с другими механизмами управления ресурсами. Важно понимать их различия:

| Механизм | Назначение | Отличие от PriorityClass |
|----------|------------|--------------------------|
| Resource Quotas | Лимиты на потребление ресурсов в неймспейсе | Ограничивает суммарное потребление, не влияет на вытеснение подов. |
| Limit Ranges | Лимиты и запросы для подов по умолчанию | Задаёт минимальные и максимальные значения, не влияет на вытеснение. |

В отличие от этих механизмов, PriorityClass влияет именно на порядок вытеснения подов при нехватке ресурсов, а не на ограничение потребления.

## Эксплуатация и диагностика

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

### Вытеснение не происходит

Если вытеснение подов с низким приоритетом не происходит, хотя ресурсы свободны, проверьте события с причиной `FailedPreemption`:

```shell
d8 k get events --field-selector reason=FailedPreemption --sort-by='.metadata.creationTimestamp'
```

Пример вывода:

```console
LAST SEEN   TYPE      REASON             OBJECT                    MESSAGE
30s         Warning   FailedPreemption   pod/high-priority-pod     no preemption victims found for pod
```

Возможные причины:

- Фрагментация ресурсов. Свободные ресурсы распределены по разным узлам, и ни один узел не может вместить новый под целиком даже после вытеснения.
- Отсутствие подходящих кандидатов. Все поды на узле имеют равный или более высокий приоритет, чем у нового пода, поэтому вытеснение невозможно по правилам планировщика.

#### Отсутствие подов с более низким приоритетом

Если под не запускается и в событиях видно сообщение `No preemption victims found for incoming pod`, проверьте приоритеты всех подов на узле:

```shell
d8 k get pods --all-namespaces -o wide --field-selector spec.nodeName=worker-0 -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,PRIORITY:.spec.priority --sort-by=.spec.priority
```

Пример вывода:

```console
NAME                                                            NODE       PRIORITY
mock-stateful-5                                                 worker-0   9000
mock-stateful-2                                                 worker-0   9000
mock-stateful-3                                                 worker-0   9000
mock-stateful-4                                                 worker-0   9000
emergency-task                                                  worker-0   10000
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

Решение: повысить приоритет целевого пода или освободить ресурсы на узле вручную.

#### Лимит подов на узле

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

Создайте файл `high-priority-pod.yaml`, чтобы запустить под с высоким приоритетом:

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
        cpu: "100m"
        memory: "256Mi"
```

Примените манифест:

```shell
d8 k apply -f high-priority-pod.yaml
```

Проверьте статус пода с высоким приоритетом:

```shell
d8 k get pod high-priority-pod
```

Пример вывода:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   0/1     Pending   0          11s
```

Посмотрите причину в событиях:

```shell
d8 k describe pod high-priority-pod | grep -A10 "Events:"
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
d8 k get events --field-selector reason=Preempted
```

Пример вывода (пусто):

```console
No resources found in default namespace.
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

{% alert level="info" %}
События хранятся ограниченное время (обычно около часа). Если вытеснения давно не было, эти команды могут ничего не вернуть — повторите демонстрацию вытеснения и выполните команды сразу после неё.
{% endalert %}

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

## Очистка созданных в примерах ресурсов

После прохождения примеров удалите созданные ресурсы. Команды ниже соответствуют разделам [Примеры создания и использования PriorityClass](#примеры-создания-и-использования-priorityclass), [Механизм вытеснения](#механизм-вытеснения), [Архитектурные сценарии использования](#архитектурные-сценарии-использования), [Защита Stateful-приложений](#защита-stateful-приложений) и [Эксплуатация и диагностика](#эксплуатация-и-диагностика).

```shell
# Ресурсы из раздела «Создание PriorityClass»
d8 k delete priorityclass my-app-critical

# Ресурсы из раздела «Использование PriorityClass в поде»
d8 k delete pod test-priority-pod

# Ресурсы из раздела «Использование PriorityClass в Deployment»
d8 k delete deployment my-app

# Ресурсы из раздела «Пошаговая демонстрация вытеснения»
d8 k delete pod low-priority-pod
d8 k delete pod high-priority-pod

# Ресурсы из раздела «Разделение окружений по приоритетам»
d8 k delete deployment prod-api
d8 k delete job manual-test
d8 k delete cronjob dev-data-processor

# Ресурсы из раздела «Защита Stateful-приложений»
d8 k delete statefulset mock-stateful
d8 k delete pdb mock-stateful-pdb
d8 k delete pod emergency-task
d8 k delete priorityclass super-critical

# Ресурсы из раздела «Лимит подов на узле»
d8 k delete deployment pod-filler
```
