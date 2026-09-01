---
description: Создание, настройка и эксплуатация PostgreSQL с помощью модуля managed-postgres.
title: "Managed PostgreSQL"
permalink: ru/user/managed-services/postgres/
lang: ru
relatedLinks:
  - title: "Частые вопросы"
    url: "faq.html"
---

Модуль `managed-postgres` позволяет создавать и настраивать PostgreSQL с помощью ресурса Postgres. Пользователь задаёт требуемую конфигурацию, а модуль создаёт и поддерживает экземпляры PostgreSQL с учётом PostgresClass, который определяет доступные параметры и ограничения. PostgresClass создаёт и настраивает администратор кластера.

В руководстве используются два примера:

- `app-postgres` — [основной пример](#основной-пример-создание-postgres) для создания и эксплуатации PostgreSQL: ресурсы, режим `Cluster`, репликация, пользователи, базы данных, параметры PostgreSQL, TLS и наблюдаемость;
- `snapshot-pg` — отдельный пример для создания и восстановления снимков, поскольку для него требуется StorageClass с поддержкой CSI-снимков.

{% alert level="info" %}
В примерах используются два worker-узла, чтобы экземпляры PostgreSQL в режиме `Cluster` могли размещаться на разных узлах. Кластер находится в одной зоне `default`.
{% endalert %}

## Проверка доступных ресурсов

Перед созданием Postgres проверьте доступные ресурсы worker-узлов. Это позволяет подобрать значения CPU и памяти для примера с учётом реальной загрузки кластера.

Сначала посмотрите список узлов:

```shell
d8 k get nodes -o wide
```

В примере доступны два worker-узла.

Пример вывода:

```console
NAME       STATUS   ROLES    AGE   VERSION
worker-1   Ready    worker   25d   v1.34.9
worker-2   Ready    worker   43m   v1.34.9
```

Проверьте занятые ресурсы первого worker-узла:

```shell
d8 k describe node worker-1 | grep -A 5 "Allocated resources"
```

Пример вывода:

```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests          Limits
  --------           --------          ------
  cpu                1104m (28%)       500m (12%)
  memory             4096854330 (53%)  390Mi (5%)
```

Проверьте второй worker-узел:

```shell
d8 k describe node worker-2 | grep -A 5 "Allocated resources"
```

Пример вывода:

```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests      Limits
  --------           --------      ------
  cpu                472m (12%)    500m (12%)
  memory             1004Mi (13%)  256Mi (3%)
```

## Проверка хранилища

Перед созданием Postgres проверьте доступные StorageClass и выберите класс хранилища, в котором будут размещаться данные PostgreSQL:

```shell
d8 k get storageclass
```

Пример вывода тестового стенда:

<!-- markdownlint-disable MD031 -->
```console
NAME                   PROVISIONER            RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION
local                  csi.dvp.deckhouse.io   Delete          WaitForFirstConsumer   true
replicated (default)   csi.dvp.deckhouse.io   Delete          WaitForFirstConsumer   true
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Параметр [`spec.instance.persistentVolumeClaim.storageClassName`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-persistentvolumeclaim-storageclassname) задаётся только при создании Postgres и не может быть изменён позднее.

## Основной пример: создание Postgres

Создайте неймспейс:

```shell
d8 k create namespace postgres
```

Для создания PostgreSQL используется ресурс Postgres. В нём указываются PostgresClass, ресурсы экземпляров, режим работы, топология и репликация, логические базы данных и пользователи, параметры PostgreSQL, TLS и наблюдаемость. Ниже эти настройки рассматриваются отдельно на основном примере `app-postgres`.

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  name: app-postgres
  namespace: postgres
spec:
  postgresClassName: default

  configuration:
    maxConnections: 120

  instance:
    cpu:
      cores: 1
      coreFraction: 50
    memory:
      size: 1Gi
    persistentVolumeClaim:
      size: 10Gi
      storageClassName: replicated

  type: Cluster
  cluster:
    topology: Ignored
    replication: Consistency

  users:
    - name: app-rw
      role: rw
      storeCredsToSecret: app-postgres-rw

  databases:
    - name: app

  tls:
    mode: K8s

  observability: Enabled
```

Сохраните манифест в `postgres.yaml` и примените его:

```shell
d8 k apply -f postgres.yaml
```

Проверьте состояние созданного Postgres:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

После завершения развёртывания основные условия должны перейти в `True` — что означает каждое условие, см. в разделе [«Проверка состояния»](#проверка-состояния).

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME           AVAILABLE   CONFIGURATIONVALID   LASTVALIDCONFIGURATIONAPPLIED   SCALEDTOLASTVALIDCONFIGURATION   DATABASESSYNCED   USERSSYNCED
app-postgres   True        True                 True                            True                             True              True
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Далее параметры `app-postgres` разбираются в разделах [«Выбор PostgresClass»](#выбор-postgresclass), [«Настройка ресурсов Postgres»](#настройка-ресурсов-postgres), [«Выбор режима работы»](#выбор-режима-работы), [«Настройка топологии и режима репликации»](#настройка-топологии-и-режима-репликации), [«Создание логической базы данных и пользователя»](#создание-логической-базы-данных-и-пользователя), [«Настройка параметров PostgreSQL»](#настройка-параметров-postgresql), [«Настройка TLS»](#настройка-tls) и [«Настройка наблюдаемости»](#настройка-наблюдаемости). Если требуется изменить параметр, измените соответствующий фрагмент `postgres.yaml` и примените тот же файл повторно.

## Выбор PostgresClass

Параметр [`spec.postgresClassName`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-postgresclassname) определяет PostgresClass, который задаёт доступные параметры и ограничения для Postgres. Посмотреть доступные в кластере PostgresClass можно командой:

```shell
d8 k get postgresclass
```

Пример вывода:

```console
NAME      AGE
default   13d
```

В данном примере используется PostgresClass `default` со стандартными ограничениями. Если выбран другой PostgresClass, его ограничения можно посмотреть в конфигурации:

```shell
d8 k get postgresclass <CLASS_NAME> -o yaml
```

Где `<CLASS_NAME>` — имя выбранного PostgresClass.

{% alert level="warning" %}
При выборе PostgresClass учитывайте допустимые значения CPU, памяти и `coreFraction`, доступные топологии и параметры PostgreSQL, разрешённые для переопределения. Если конфигурация Postgres не соответствует ограничениям выбранного класса, API отклонит её при применении.
{% endalert %}

Настройки и ограничения PostgresClass описаны [в разделе «Ограничение ресурсов CPU и памяти»](/admin/configuration/managed-services/postgres/#ограничение-ресурсов-cpu-и-памяти), [разделе «Управление отказоустойчивостью через зоны доступности»](/admin/configuration/managed-services/postgres/#управление-отказоустойчивостью-через-зоны-доступности) и [разделе «Автоматическая проверка настроек PostgreSQL»](/admin/configuration/managed-services/postgres/#автоматическая-проверка-настроек-postgresql).

### Ограничения размещения

PostgresClass также может определять правила размещения экземпляров PostgreSQL с помощью `nodeSelector`, `nodeAffinity` и `tolerations`. Эти правила применяются автоматически при выборе класса и не указываются в ресурсе Postgres.

## Настройка ресурсов Postgres

Для каждого экземпляра PostgreSQL можно задать количество CPU, долю гарантированного CPU и объём памяти.

Параметр [`spec.instance`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance) определяет ресурсы каждого экземпляра PostgreSQL.

В примере за ресурсы и хранилище отвечает этот фрагмент:

```yaml
spec:
  instance:
    cpu:
      cores: 1
      coreFraction: 50
    memory:
      size: 1Gi
```

В примере экземпляру выделяется одно ядро CPU и `1Gi` памяти. Параметр [`coreFraction`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-cpu-corefraction) определяет отношение CPU request к CPU limit. Для `cores: 1` и `coreFraction: 50` модуль сформировал:

Пример вывода:

```console
limits.cpu:   1
requests.cpu: 500m
```

Подробнее — [в разделе «Ограничение ресурсов CPU и памяти»](/admin/configuration/managed-services/postgres/#ограничение-ресурсов-cpu-и-памяти).

### Изменение ресурсов существующего Postgres

Ресурсы Postgres можно изменять повторным применением манифеста, если новые значения разрешены выбранным PostgresClass. Сначала узнайте текущие значения ресурсов командой:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o custom-columns='NAME:.metadata.name,CPU_REQUEST:.spec.containers[0].resources.requests.cpu,MEMORY_REQUEST:.spec.containers[0].resources.requests.memory'
```

Пример вывода:

```console
NAME                       CPU_REQUEST   MEMORY_REQUEST
d8ms-pg-app-postgres-1     500m          1Gi
d8ms-pg-app-postgres-2     500m          1Gi
```

Можно сразу применить изменения как для памяти, так и для CPU, но для наглядности сначала увеличьте память с `1Gi` до `2Gi`:

```yaml
spec:
  instance:
    memory:
      size: 2Gi
```

Примените изменённый манифест:

```shell
d8 k apply -f postgres.yaml
```

После завершения обновления CPU request останется `500m`, а memory request экземпляров изменится на `2Gi`.

Пример вывода:

```console
NAME                       CPU_REQUEST   MEMORY_REQUEST
d8ms-pg-app-postgres-1     500m          2Gi
d8ms-pg-app-postgres-2     500m          2Gi
```

Затем измените `coreFraction` с `50` на `100`, оставив `cores: 1`:

```yaml
spec:
  instance:
    cpu:
      cores: 1
      coreFraction: 100
```

Повторно примените манифест:

```shell
d8 k apply -f postgres.yaml
```

После завершения обновления CPU request экземпляров изменится с `500m` на `1`, а memory request останется `2Gi`.

Пример вывода:

```console
NAME                       CPU_REQUEST   MEMORY_REQUEST
d8ms-pg-app-postgres-1     1             2Gi
d8ms-pg-app-postgres-2     1             2Gi
```

Таким образом, у работающего Postgres можно изменять память и `coreFraction` в пределах, разрешённых выбранным PostgresClass.

### Проверка ограничения CPU и памяти через PostgresClass

Значения CPU и памяти должны соответствовать ограничениям выбранного PostgresClass. Если указанные ресурсы не соответствуют допустимым значениям или их сочетаниям, API отклонит конфигурацию.

PostgresClass `default` не подходит для наглядной проверки ограничений. Поэтому в этом примере используется отдельный PostgresClass `check`, который разрешает для `1–2` CPU память от `512Mi` до `2Gi` с шагом `512Mi`.

При `cores: 1` и `coreFraction: 50` значение памяти `700Mi` не соответствует установленному шагу, поэтому манифест будет отклонён:

```yaml
spec:
  postgresClassName: check
  instance:
    cpu:
      cores: 1
      coreFraction: 50
    memory:
      size: 700Mi
```

Примените манифест:

```shell
d8 k apply -f postgres.yaml
```

API отклонит ресурс. Пример вывода:

```console
spec.instance.memory.size: Invalid value: 734003200: memory setting does not fit Step 536870912 of the selected PostgresClass
```

## Выбор режима работы

От выбора режима работы зависит состав экземпляров PostgreSQL: `Cluster` создаёт основной экземпляр и реплики, состав которых зависит от выбранного режима репликации. А `Standalone` — один экземпляр PostgreSQL без реплик.

### Режим Cluster

Для работы с основным экземпляром и репликами используется режим `Cluster`, который задаётся параметром [`spec.type`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-type).

```yaml
spec:
  type: Cluster
  cluster:
    topology: Ignored
    replication: Consistency
```

Режим репликации и его параметры настраиваются отдельно. Доступные режимы и примеры их использования описаны [в разделе «Настройка репликации»](#настройка-репликации).

### Режим Standalone

Режим `Standalone` используется для запуска PostgreSQL с одним экземпляром без репликации. В отличие от режима `Cluster`, для него не используются параметры топологии и репликации.

Чтобы использовать этот режим, укажите:

```yaml
spec:
  type: Standalone
```

После создания Postgres будет запущен один экземпляр PostgreSQL. Проверьте созданные экземпляры PostgreSQL:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Пример вывода:

```console
NAME                     STATUS    NODE
d8ms-pg-app-postgres-1   Running   worker-1
```

Проверьте сервисы Kubernetes, созданные для подключения к PostgreSQL:

```shell
d8 k get svc -n postgres | grep app-postgres
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
d8ms-pg-app-postgres-r    ClusterIP   10.223.234.52    <none>   5432/TCP
d8ms-pg-app-postgres-ro   ClusterIP   10.223.70.248    <none>   5432/TCP
d8ms-pg-app-postgres-rw   ClusterIP   10.223.120.250   <none>   5432/TCP
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Проверьте, на какие экземпляры направлены сервисы, через эндпоинты:

```shell
d8 k get endpoints -n postgres | grep app-postgres
```

Пример вывода:

```console
d8ms-pg-app-postgres-r    10.112.2.31:5432   42h
d8ms-pg-app-postgres-ro   <none>             42h
d8ms-pg-app-postgres-rw   10.112.2.31:5432   42h
```

Сервисы `-r` и `-rw` направляют подключения на единственный экземпляр PostgreSQL. Сервис `-ro` также создаётся, но не имеет эндпоинта, поскольку в режиме `Standalone` отсутствуют реплики.

## Настройка топологии и режима репликации

В режиме `Cluster` топология определяет размещение экземпляров PostgreSQL по узлам и зонам доступности. Она позволяет управлять тем, где будут размещены экземпляры, чтобы учитывать требования к отказоустойчивости PostgreSQL.

### Настройка топологии

Параметр [`spec.cluster.topology`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-topology) определяет размещение экземпляров PostgreSQL по узлам и зонам доступности.

Поддерживаются следующие значения:

- `Ignored` — размещение выполняется по стандартным правилам планирования Kubernetes с разнесением экземпляров по разным узлам;
- `Zonal` — экземпляры размещаются в пределах одной из разрешённых зон;
- `TransZonal` — экземпляры размещаются в разных зонах доступности.

Доступные значения топологии и зоны определяются выбранным PostgresClass. Для `Zonal` и `TransZonal` инфраструктура кластера должна предоставлять соответствующие зоны доступности. Подробнее — [в разделе «Управление отказоустойчивостью через зоны доступности»](/admin/configuration/managed-services/postgres/#управление-отказоустойчивостью-через-зоны-доступности).

#### Размещение без выбора зоны

При `topology: Ignored` размещением экземпляров управляет планировщик Kubernetes. Режим обеспечивает разнесение экземпляров по разным узлам без дополнительных настроек со стороны пользователя. В [основном примере](#основной-пример-создание-postgres) используется этот режим:

```yaml
spec:
  cluster:
    topology: Ignored
```

Проверьте размещение экземпляров командой:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Экземпляры должны находиться на разных узлах.

Пример вывода:

```console
NAME                       STATUS    NODE
d8ms-pg-app-postgres-1     Running   worker-1
d8ms-pg-app-postgres-2     Running   worker-2
```

#### Размещение в одной зоне

При `topology: Zonal` для размещения Postgres выбирается одна из зон, разрешённых выбранным PostgresClass. Все экземпляры кластера размещаются в пределах этой зоны.

```yaml
spec:
  cluster:
    topology: Zonal
```

Для использования `Zonal` узлы должны иметь лейбл `topology.kubernetes.io/zone` со значением соответствующей зоны. Зона должна быть разрешена выбранным PostgresClass.

Например, если два доступных узла относятся к зоне `default`:

Пример вывода:

```console
NAME       ZONE
worker-1   default
worker-2   default
```

Экземпляры могут быть размещены следующим образом.

Пример вывода:

```console
NAME                       STATUS    NODE
d8ms-pg-app-postgres-1     Running   worker-1
d8ms-pg-app-postgres-2     Running   worker-2
```

В этом примере оба экземпляра размещены в зоне `default`.

### Настройка репликации

В режиме `Cluster` репликация обеспечивает передачу данных с основного экземпляра PostgreSQL на реплики. Режим репликации задаётся в [`spec.cluster.replication`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-replication).

Поддерживаются следующие режимы:

- `Availability` — основной экземпляр и одна асинхронная реплика;
- `Consistency` — основной экземпляр и одна синхронная реплика;
- `ConsistencyAndAvailability` — основной экземпляр, одна синхронная и одна асинхронная реплика.

#### Проверка режима репликации

Состояние репликации проверяется через представление `pg_stat_replication` на основном экземпляре. Эту процедуру используйте для любого режима `Cluster`, в том числе после смены `spec.cluster.replication`.

Сначала определите текущий основной экземпляр:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"
```

Затем выполните запрос:

```shell
d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -c \
  "SELECT application_name, state, sync_state FROM pg_stat_replication;"
```

Ожидаемые значения `sync_state` зависят от режима:

| Режим | Число реплик | Ожидаемый `sync_state` |
| :-- | :-- | :-- |
| `Availability` | 1 | `async` |
| `Consistency` | 1 | `quorum` |
| `ConsistencyAndAvailability` | 2 | `quorum` и `async` |

Значение `state: streaming` означает, что реплика получает изменения от основного экземпляра. Во время смены режима роли экземпляров могут меняться, поэтому каждый раз определяйте основной экземпляр заново через `status.targetPrimary`, а не по номеру Pod.

#### Режим Availability

Режим `Availability` создаёт основной экземпляр PostgreSQL и одну асинхронную реплику.

Чтобы использовать этот режим, укажите:

```yaml
spec:
  type: Cluster
  cluster:
    topology: Zonal
    replication: Availability
```

После создания Postgres запускаются два экземпляра PostgreSQL:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Пример вывода:

```console
NAME                     READY   STATUS
d8ms-pg-app-postgres-1   1/1     Running
d8ms-pg-app-postgres-2   1/1     Running
```

Проверьте режим репликации, как описано [в разделе «Проверка режима репликации»](#проверка-режима-репликации). Для `Availability` ожидается одна реплика со `sync_state = async`:

```console
     application_name      |   state   | sync_state
---------------------------+-----------+------------
 d8ms-pg-app-postgres-2    | streaming | async
(1 row)
```

Распределение сервисов между основным экземпляром и репликой можно проверить через EndpointSlice:

```shell
d8 k get endpointslice -n postgres | grep app-postgres
```

Пример вывода:

```console
d8ms-pg-app-postgres-r-v8kcv    IPv4   5432   10.112.2.249,10.112.2.155
d8ms-pg-app-postgres-ro-696bp   IPv4   5432   10.112.2.155
d8ms-pg-app-postgres-rw-8nx8s   IPv4   5432   10.112.2.249
```

Сервис `-rw` направляет подключения на основной экземпляр, `-ro` — на реплику, а `-r` — на оба экземпляра.

#### Режим Consistency

Режим `Consistency`, используемый в [основном примере](#основной-пример-создание-postgres), создаёт основной экземпляр PostgreSQL и одну синхронную реплику:

```yaml
spec:
  type: Cluster
  cluster:
    topology: Ignored
    replication: Consistency
```

После создания Postgres запускаются два экземпляра PostgreSQL:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Пример вывода:

```console
NAME                       STATUS    NODE
d8ms-pg-app-postgres-1     Running   worker-1
d8ms-pg-app-postgres-2     Running   worker-2
```

Проверьте режим репликации, как описано [в разделе «Проверка режима репликации»](#проверка-режима-репликации). Для `Consistency` ожидается одна реплика со `sync_state = quorum`:

```console
      application_name      |   state   | sync_state
----------------------------+-----------+------------
 d8ms-pg-app-postgres-2     | streaming | quorum
(1 row)
```

Дополнительно работу синхронной репликации можно проверить по фактической передаче данных. Определите основной экземпляр так же, как [в разделе «Проверка режима репликации»](#проверка-режима-репликации):

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"
```

Создайте на основном экземпляре контрольную таблицу и добавьте запись:

```shell
d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -c "
    CREATE TABLE consistency_check (
      id integer PRIMARY KEY,
      value text
    );
    INSERT INTO consistency_check VALUES (1, 'replicated');
  "
```

Определите реплику:

```shell
REPLICA="$(d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | \
  grep -v "^${PRIMARY}$" | head -n1)"
```

Проверьте наличие записи непосредственно на реплике:

```shell
d8 k exec -n postgres "$REPLICA" -- \
  psql -U postgres -d postgres -c \
  "SELECT pg_is_in_recovery(), * FROM consistency_check;"
```

Пример вывода:

```console
 pg_is_in_recovery | id |   value
-------------------+----+------------
 t                 |  1 | replicated
(1 row)
```

Значение `pg_is_in_recovery() = t` показывает, что запрос выполнен на реплике. Наличие строки `replicated` подтверждает передачу данных с основного экземпляра на синхронную реплику.

#### Режим ConsistencyAndAvailability

Режим `ConsistencyAndAvailability` создаёт основной экземпляр PostgreSQL, одну синхронную и одну асинхронную реплику.

Чтобы использовать этот режим, укажите:

```yaml
spec:
  type: Cluster
  cluster:
    topology: Zonal
    replication: ConsistencyAndAvailability
```

После создания Postgres запускаются три экземпляра PostgreSQL:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Пример вывода:

```console
NAME                     READY   STATUS
d8ms-pg-app-postgres-1   1/1     Running
d8ms-pg-app-postgres-2   1/1     Running
d8ms-pg-app-postgres-3   1/1     Running
```

Проверьте режим репликации, как описано [в разделе «Проверка режима репликации»](#проверка-режима-репликации). Для `ConsistencyAndAvailability` ожидаются две реплики — со `sync_state = quorum` и `sync_state = async`:

```console
     application_name      |   state   | sync_state
---------------------------+-----------+------------
 d8ms-pg-app-postgres-2    | streaming | quorum
 d8ms-pg-app-postgres-3    | streaming | async
(2 rows)
```

### Изменение режима репликации существующего кластера

Режим репликации можно изменить у уже существующего Postgres в режиме `Cluster`. Для этого измените `spec.cluster.replication` в манифесте `app-postgres` и примените его повторно.

Например, чтобы перейти с `Availability` на `Consistency`, укажите:

```yaml
spec:
  cluster:
    replication: Consistency
```

Примените изменения:

```shell
d8 k apply -f postgres.yaml
```

Во время обновления `ScaledToLastValidConfiguration` может временно перейти в `False`. После завершения обновления условия ресурса должны вернуться в `True`.

Проверьте новый режим, как описано [в разделе «Проверка режима репликации»](#проверка-режима-репликации). После перехода на `Consistency` реплика должна работать в синхронном режиме:

```console
d8ms-pg-app-postgres-1 | streaming | quorum
```

При обратном переходе на `Availability` та же проверка должна показывать асинхронную репликацию:

```console
d8ms-pg-app-postgres-2 | streaming | async
```

При переходе на `ConsistencyAndAvailability` число экземпляров увеличивается с двух до трёх. Проверьте запущенные экземпляры:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

После завершения обновления проверка `pg_stat_replication` должна показывать синхронную и асинхронную реплики:

```console
     application_name      |   state   | sync_state
---------------------------+-----------+------------
 d8ms-pg-app-postgres-3    | streaming | async
 d8ms-pg-app-postgres-2    | streaming | quorum
(2 rows)
```

При обратном переходе с `ConsistencyAndAvailability` на `Consistency` число экземпляров уменьшается с трёх до двух, а оставшаяся реплика работает в режиме `streaming | quorum`.

## Создание логической базы данных и пользователя

В [основном примере](#основной-пример-создание-postgres) создаются пользователь `app-rw` и логическая база данных `app`:

```yaml
spec:
  users:
    - name: app-rw
      role: rw
      storeCredsToSecret: app-postgres-rw

  databases:
    - name: app
```

После применения манифеста дождитесь синхронизации пользователей и баз данных. Состояния `USERSSYNCED` и `DATABASESSYNCED` должны иметь значение `True`:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

### Пользователь PostgreSQL

Учётные данные пользователя сохраняются в Secret, указанном в [`storeCredsToSecret`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-users-storecredstosecret).

Проверьте созданный Secret:

```shell
d8 k get secret app-postgres-rw -n postgres
```

Пример вывода:

```console
NAME              TYPE                       DATA
app-postgres-rw   kubernetes.io/basic-auth   4
```

Secret содержит параметры, необходимые для подключения:

```text
app-dsn
host
password
username
```

Получите параметры подключения следующим образом:

```shell
echo "host: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.host}' | base64 --decode)"
echo "username: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.username}' | base64 --decode)"
echo "password: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.password}' | base64 --decode)"
echo "app-dsn: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.app-dsn}' | base64 --decode)"
```

Пример вывода:

```console
host: d8ms-pg-app-postgres-rw
username: app-rw
password: <PASSWORD>
app-dsn: postgresql://app-rw:<PASSWORD>@d8ms-pg-app-postgres-rw:5432/app
```

Где `<PASSWORD>` — пароль пользователя из Secret.

Значения из Secret можно использовать для настройки подключения приложения или PostgreSQL-клиента.

{% alert level="info" %}
Для подключения приложений используйте Secret, имя которого указано в `storeCredsToSecret`. Внутренние Secret с именами вида `d8ms-pg-...` для этого использовать не следует.
{% endalert %}

### Декларативное управление пользователями

Список пользователей в [`spec.users`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-users) описывает требуемое состояние PostgreSQL. При изменении списка модуль синхронизирует роли пользователей и связанные с ними Secret.

Например, удалите пользователя `app-rw` из манифеста:

```yaml
spec:
  users: []
```

Примените изменённый манифест:

```shell
d8 k apply -f postgres.yaml
```

После завершения синхронизации условие `USERSSYNCED` должно вернуться в `True`:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

Проверьте отсутствие роли непосредственно в PostgreSQL:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"

d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -Atc \
  "SELECT rolname FROM pg_roles WHERE rolname = 'app-rw';"
```

Команда не должна вернуть имя роли.

При выполнении `d8 k exec` может отображаться служебное сообщение о выборе контейнера:

```console
Defaulted container "postgres" out of: postgres, bootstrap-controller (init)
```

Логическая база данных `app`, оставшаяся в [`spec.databases`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-databases), при удалении пользователя не удаляется. Проверьте её наличие командой:

```shell
d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -Atc \
  "SELECT datname FROM pg_database WHERE datname = 'app';"
```

Ожидаемый вывод:

```console
app
```

{% alert level="warning" %}
Удаление пользователя из `spec.users` приводит к удалению соответствующей роли PostgreSQL. Перед удалением убедитесь, что пользователь больше не используется приложениями.
{% endalert %}

Чтобы создать пользователя заново, добавьте его обратно в `spec.users`:

```yaml
spec:
  users:
    - name: app-rw
      role: rw
      storeCredsToSecret: app-postgres-rw
```

После повторного применения манифеста модуль создаст роль PostgreSQL и Secret заново.

### Логическая база данных

Логические базы данных, которые должен создать и поддерживать модуль, задаются в `spec.databases`:

```yaml
spec:
  databases:
    - name: app
```

После создания базы условие `DATABASESSYNCED` должно иметь значение `True`.

{% alert level="warning" %}
Удаление базы данных из `spec.databases` приводит к удалению соответствующей логической базы данных PostgreSQL вместе с её данными.
{% endalert %}

## Подключение к PostgreSQL

После создания Postgres модуль создаёт сервисы `-r`, `-ro` и `-rw`, которые используются для подключения к экземплярам PostgreSQL в зависимости от их роли:

- `-rw` — основной экземпляр;
- `-ro` — реплики;
- `-r` — все доступные экземпляры.

Для наглядности в примере `app-postgres` создаются следующие сервисы:

```text
NAME                      TYPE        PORT(S)
d8ms-pg-app-postgres-r    ClusterIP   5432/TCP
d8ms-pg-app-postgres-ro   ClusterIP   5432/TCP
d8ms-pg-app-postgres-rw   ClusterIP   5432/TCP
```

По умолчанию эти сервисы имеют тип `ClusterIP` и доступны внутри кластера. Учётные данные и параметры подключения пользователя сохраняются в Secret, указанном в `storeCredsToSecret`.

Подключаться к PostgreSQL можно как из приложений внутри кластера, так и из внешней сети. Для внешнего подключения сервис необходимо дополнительно опубликовать.

### Подключение из кластера

Для подключения из кластера используйте соответствующий сервис PostgreSQL и учётные данные из Secret пользователя. В [основном примере](#основной-пример-создание-postgres) приложение с правами на запись подключается к сервису `d8ms-pg-app-postgres-rw` от имени пользователя `app-rw` к базе данных `app`.

Для проверки подключения не требуется устанавливать `psql` на control-plane-узел. Для этого можно использовать временный клиентский Pod:

```shell
d8 k run postgres-client \
  -n postgres \
  --rm -it \
  --restart=Never \
  --image=postgres:17 \
  --env="PGPASSWORD=$(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.password}' | base64 --decode)" \
  -- \
  psql \
    -h d8ms-pg-app-postgres-rw \
    -U app-rw \
    -d app \
    -c 'SELECT current_database(), session_user, current_user;'
```

Пример вывода:

```console
 current_database | session_user | current_user
------------------+--------------+--------------
 app              | app-rw       | rw
(1 row)
```

`session_user` показывает пользователя, от имени которого выполнено подключение (`app-rw`), а `current_user` — действующую роль прав (`rw`).

### Внешнее подключение к PostgreSQL

Для работы с PostgreSQL из внешней сети можно использовать графические клиенты и другие приложения, поддерживающие подключение к PostgreSQL. Для этого сервис PostgreSQL должен быть доступен извне кластера, а клиенту необходимо указать адрес сервера, порт, базу данных и учётные данные пользователя.

{% alert level="info" %}
В этом разделе внешнее подключение рассматривается на примере DBeaver. Аналогичным образом можно использовать другие PostgreSQL-клиенты и приложения.
{% endalert %}

В примере подключение выполняется к созданной ранее базе данных `app` от имени пользователя `app-rw` в Postgres `app-postgres`.

#### Публикация PostgreSQL для внешнего доступа

Способ публикации зависит от сетевой инфраструктуры кластера. В этом примере внешний балансировщик нагрузки принимает подключения к `<EXTERNAL_IP>:5432` и перенаправляет их на `NodePort` `30001` узла кластера. Отдельный Service направляет этот трафик на основной экземпляр PostgreSQL.

Не изменяйте созданный модулем сервис `d8ms-pg-app-postgres-rw`. Создайте отдельный Service для внешнего доступа:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: app-postgres-external
  namespace: postgres
spec:
  type: NodePort
  selector:
    cnpg.internal.managed.deckhouse.io/cluster: d8ms-pg-app-postgres
    cnpg.internal.managed.deckhouse.io/instanceRole: primary
  ports:
    - name: postgres
      protocol: TCP
      port: 5432
      targetPort: 5432
      nodePort: 30001
```

Примените манифест:

```shell
d8 k apply -f app-postgres-external.yaml
```

Проверьте созданный Service:

```shell
d8 k get svc app-postgres-external -n postgres -o wide
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                    TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE   SELECTOR
app-postgres-external   NodePort   10.223.111.45   <none>        5432:30001/TCP   4s    cnpg.internal.managed.deckhouse.io/cluster=d8ms-pg-app-postgres,cnpg.internal.managed.deckhouse.io/instanceRole=primary
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

На внешнем балансировщике нагрузки настройте приём TCP-соединений на порту `5432` и перенаправление на `NodePort` `30001` узла кластера. В текущем примере создана такая схема:

```text
<EXTERNAL_IP>:5432
        |
внешний балансировщик нагрузки
        |
<NODE_IP>:30001
        |
NodePort
        |
primary PostgreSQL :5432
```

{% alert level="warning" %}
При публикации PostgreSQL во внешней сети убедитесь, что доступ к порту базы данных ограничен только доверенными источниками. Для этого могут использоваться файрвол, списки разрешённых IP-адресов, VPN и другие средства сетевой инфраструктуры. Не рекомендуется оставлять PostgreSQL доступным из Интернета без ограничений.
{% endalert %}

Перед проверкой внешнего подключения можно убедиться, что созданный `NodePort` направляет трафик на основной экземпляр PostgreSQL. Для этого получите пароль пользователя:

```shell
PGPASSWORD="$(d8 k get secret app-postgres-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode)"
```

Запустите временный клиентский Pod и подключитесь через IP узла и `NodePort`:

```shell
d8 k run nodeport-test \
  -n postgres \
  --rm -i \
  --restart=Never \
  --image=postgres:17 \
  --env="PGPASSWORD=$PGPASSWORD" \
  -- \
  psql \
    -h <NODE_IP> \
    -p 30001 \
    -U app-rw \
    -d app \
    -c "SELECT current_database(), pg_is_in_recovery(), inet_server_addr();"
```

Пример успешного результата:

```console
 current_database | pg_is_in_recovery | inet_server_addr
------------------+-------------------+------------------
 app              | f                 | <POD_IP>
(1 row)
```

Значение `pg_is_in_recovery = f` подтверждает, что соединение направлено на основной экземпляр PostgreSQL.

#### Подключение по IP-адресу

Подключение непосредственно по IP-адресу технически возможно и может использоваться, например, для проверки внешней доступности PostgreSQL. Для постоянного подключения рекомендуется использовать DNS-имя и TLS с проверкой сертификата сервера, как описано далее.

Получите пароль пользователя:

```shell
d8 k get secret app-postgres-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode; echo
```

Создайте в DBeaver подключение PostgreSQL и укажите:

```text
Host:     <EXTERNAL_IP>
Port:     5432
Database: app
Username: app-rw
Password: <PASSWORD>
```

Где `<PASSWORD>` — пароль из Secret `app-postgres-rw`.

После подключения откройте SQL Editor и выполните:

```sql
SELECT
    current_database(),
    session_user,
    inet_server_addr(),
    inet_server_port(),
    pg_is_in_recovery();
```

На проверенном стенде запрос вернул:

<!-- markdownlint-disable MD031 -->
```console
 current_database | session_user | inet_server_addr | inet_server_port | pg_is_in_recovery
------------------+--------------+------------------+------------------+-------------------
 app              | app-rw       | <POD_IP>         |             5432 | f
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Значение `pg_is_in_recovery = f` подтверждает подключение к основному экземпляру PostgreSQL.

#### Подключение с проверкой TLS

Для постоянного внешнего подключения рекомендуется использовать TLS с проверкой сертификата сервера.

В примере для `app-postgres` используется режим `K8s`, поэтому TLS-сертификаты PostgreSQL выпускаются автоматически. Серверный сертификат подписан `cluster-selfsigned-ca`.

Сохраните автоматически созданный серверный сертификат в файл, чтобы определить DNS-имя из SAN:

```shell
d8 k get secret d8ms-pg-app-postgres-server-cert \
  -n postgres \
  -o jsonpath='{.data.tls\.crt}' | \
  base64 --decode > /tmp/app-postgres-server.crt
```

Посмотрите сведения о сертификате и его Subject Alternative Name (SAN):

```shell
openssl x509 \
  -in /tmp/app-postgres-server.crt \
  -noout \
  -subject -issuer -dates -ext subjectAltName
```

Для `app-postgres` сертификат содержит DNS-имя сервиса `-rw`:

```console
d8ms-pg-app-postgres-postgres-rw.<EXTERNAL_IP>.sslip.io
```

При использовании режима `verify-full` клиент проверяет соответствие имени сервера сертификату, поэтому для подключения используйте DNS-имя из SAN.

Получите CA-сертификат:

```shell
d8 k get secret selfsigned-ca-key-pair \
  -n d8-cert-manager \
  -o jsonpath='{.data.tls\.crt}' | \
  base64 --decode > /tmp/app-postgres-ca.crt
```

Проверьте цепочку доверия:

```shell
openssl verify \
  -CAfile /tmp/app-postgres-ca.crt \
  /tmp/app-postgres-server.crt
```

Пример успешного результата:

```console
/tmp/app-postgres-server.crt: OK
```

Перенесите CA-сертификат на компьютер, с которого выполняется подключение. Например, если к узлу кластера доступен SSH, скопируйте сертификат с помощью `scp`:

```shell
scp user@<NODE_IP>:/tmp/app-postgres-ca.crt ~/app-postgres-ca.crt
```

В DBeaver укажите параметры подключения:

```text
Host:     d8ms-pg-app-postgres-postgres-rw.<EXTERNAL_IP>.sslip.io
Port:     5432
Database: app
Username: app-rw
Password: <PASSWORD>
```

Где `<PASSWORD>` — пароль из Secret `app-postgres-rw`.

В настройках SSL укажите CA-сертификат и режим `verify-full`:

```text
CA Certificate: <CA_CERT_PATH>
SSL mode:       verify-full
```

Где `<CA_CERT_PATH>` — путь к файлу `app-postgres-ca.crt`.

После подключения выполните:

```sql
SELECT
    current_database(),
    session_user,
    inet_server_addr(),
    inet_server_port(),
    pg_is_in_recovery();
```

Успешное выполнение запроса и значение `pg_is_in_recovery = f` подтверждают подключение к основному экземпляру PostgreSQL.

Если при использовании `verify-full` вместо DNS-имени из SAN указать IP-адрес `<EXTERNAL_IP>`, проверка имени сервера завершится ошибкой:

```console
The hostname <EXTERNAL_IP> could not be verified by hostnameverifier PgjdbcHostnameVerifier.
```

Таким образом, для подключения с `verify-full` используйте DNS-имя, указанное в SAN серверного сертификата.

## Настройка параметров PostgreSQL

Параметры PostgreSQL можно изменять через [`spec.configuration`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-configuration), если выбранный PostgresClass разрешает их переопределение.

Возможность изменения параметра определяется настройками PostgresClass:

- параметр должен быть разрешён для переопределения;
- значение параметра должно соответствовать установленным правилам проверки.

Если параметр запрещён для изменения или его значение не соответствует ограничениям, API отклонит применение ресурса Postgres.

### Изменение разрешённого параметра

В [основном примере](#основной-пример-создание-postgres) `app-postgres` использует PostgresClass `default`, который разрешает изменять параметр `maxConnections`.

Измените значение:

```yaml
spec:
  configuration:
    maxConnections: 100
```

Примените изменения:

```shell
d8 k apply -f postgres.yaml
```

После завершения обновления проверьте применённое значение непосредственно в PostgreSQL:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"

d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -c \
  "SHOW max_connections;"
```

Пример вывода:

```console
 max_connections
-----------------
 100
```

Параметр был изменён, так как он разрешён для переопределения выбранным PostgresClass.

### Ограничения на переопределение параметров

PostgresClass может ограничивать список параметров PostgreSQL, которые пользователь может изменять через `spec.configuration`.

Например, если PostgresClass разрешает переопределять только:

```yaml
overridableConfiguration:
  - maxConnections
  - sharedBuffers
  - walKeepSize
```

попытка изменить параметр, отсутствующий в этом списке, будет отклонена.

Примените, например:

```yaml
spec:
  configuration:
    workMem: 16Mi
```

```shell
d8 k apply -f postgres.yaml
```

API вернёт ошибку. Пример вывода:

```console
Configuration field workmem restricted to override by administrator in selected postgresClass
```

В этом случае конфигурация Postgres не изменится, так как параметр запрещён выбранным PostgresClass.

### Проверка значений параметров

Помимо списка разрешённых параметров, PostgresClass может задавать правила проверки допустимых значений.

Например, если для `maxConnections` установлено ограничение:

```text
configuration.maxConnections >= 100
```

следующее изменение будет отклонено:

```yaml
spec:
  configuration:
    maxConnections: 50
```

Примените манифест:

```shell
d8 k apply -f postgres.yaml
```

API вернёт ошибку. Пример вывода:

```console
Rule: configuration.maxConnections >= 100
```

Существующий Postgres продолжит работать с последней успешно применённой конфигурацией.

## Настройка TLS

Параметр [`spec.tls`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-tls) определяет способ управления TLS-сертификатами PostgreSQL. Поддерживаются режимы `CertManager`, `CustomCertificate` и `K8s`.

Для использования сертификатов, выпускаемых cert-manager, укажите режим `CertManager`:

```yaml
spec:
  tls:
    mode: CertManager
    certManager:
      clusterIssuerName: postgres-ca
```

Соответствующий Issuer или ClusterIssuer должен быть предварительно подготовлен. Административные зависимости описаны [в разделе «Зависимости для отдельных функций»](/admin/configuration/managed-services/postgres/#зависимости-для-отдельных-функций).

Чтобы использовать существующие сертификаты из Secret, выберите режим `CustomCertificate`:

```yaml
spec:
  tls:
    mode: CustomCertificate
    customCertificate:
      serverCASecret: postgres-ca
      serverTLSSecret: postgres-tls
```

### Режим K8s

В режиме `K8s` сертификаты для PostgreSQL выпускаются автоматически:

```yaml
spec:
  tls:
    mode: K8s
```

После перехода ресурса в готовое состояние модуль создаёт Secret с CA, серверным и репликационным сертификатами.

Проверьте использование TLS на стороне PostgreSQL через представление `pg_stat_ssl`. Определите основной экземпляр так же, как [в разделе «Проверка режима репликации»](#проверка-режима-репликации), и выполните запрос:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"

d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -c "
    SELECT
      a.pid,
      a.usename,
      a.client_addr,
      a.client_port,
      s.ssl,
      s.version,
      s.cipher
    FROM pg_stat_activity a
    LEFT JOIN pg_stat_ssl s USING (pid)
    WHERE a.usename = 'app-rw';
  "
```

Для TLS-соединения поле `ssl` имеет значение `t`, а в `version` и `cipher` отображаются используемые версия TLS и шифр.

Настройка клиентского подключения с проверкой сертификата сервера описана [в разделе «Подключение с проверкой TLS»](#подключение-с-проверкой-tls).

## Настройка наблюдаемости

Для Postgres можно включить мониторинг с алертами, полностью отключить мониторинг или оставить мониторинг без алертов. Режим наблюдаемости задаётся параметром [`spec.observability`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-observability).

В [основном примере](#основной-пример-создание-postgres) включены мониторинг и алерты:

```yaml
spec:
  observability: Enabled
```

Чтобы полностью отключить мониторинг, используйте:

```yaml
spec:
  observability: Disabled
```

Чтобы сохранить мониторинг, но отключить алерты, используйте:

```yaml
spec:
  observability: EnabledWithoutAlerts
```

Проверьте применённый режим по лейблам Pod:

```shell
d8 k get pod -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o json | \
  jq '.items[].metadata.labels | with_entries(select(.key | test("observability|prometheus")))'
```

Значение лейбла `observability.deckhouse.io/servicemonitoring` зависит от выбранного режима:

```text
Enabled                → enabled
Disabled               → disabled
EnabledWithoutAlerts   → no-alerts
```

При включённом мониторинге вывод также содержит лейбл:

```console
"prometheus.deckhouse.io/custom-target": "managed-postgres"
```

## Резервное копирование и восстановление

Для создания снимков используется ресурс PostgresSnapshot. StorageClass, в котором размещён Postgres, должен использовать CSI-драйвер с поддержкой snapshots, а в кластере должен быть доступен соответствующий VolumeSnapshotClass.

В [основном примере](#основной-пример-создание-postgres) используется StorageClass `replicated`, для которого провайдер в рассматриваемой конфигурации не поддерживает создание снимков. Поэтому для демонстрации используется отдельный StorageClass `snapshot-local` на `sds-local-volume` с LVM Thin.

Проверьте доступные классы снимков:

```shell
d8 k get volumesnapshotclass
```

Для `snapshot-local` доступен следующий класс снимков:

<!-- markdownlint-disable MD031 -->
```console
NAME                              DRIVER                           DELETIONPOLICY
sds-local-volume-snapshot-class   local.csi.storage.deckhouse.io   Delete
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

### Создание снимка

Для проверки создайте отдельный Postgres `snapshot-pg` в StorageClass `snapshot-local`:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  name: snapshot-pg
  namespace: postgres
spec:
  postgresClassName: default
  instance:
    cpu:
      cores: 1
      coreFraction: 50
    memory:
      size: 1Gi
    persistentVolumeClaim:
      size: 2Gi
      storageClassName: snapshot-local
  type: Standalone
  users:
    - name: snapshot-rw
      role: rw
      storeCredsToSecret: snapshot-pg-rw
  databases:
    - name: snapshotdb
```

Чтобы наглядно проверить восстановление данных на момент создания снимка, используйте контрольную таблицу: строку `BEFORE_SNAPSHOT` добавьте до создания снимка, а `AFTER_SNAPSHOT` — после.

Создайте контрольную таблицу и запишите первую строку:

```shell
PGPASSWORD="$(d8 k get secret snapshot-pg-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode)"

d8 k run snapshot-client \
  -n postgres \
  --rm -i \
  --restart=Never \
  --image=postgres:17 \
  --env="PGPASSWORD=$PGPASSWORD" \
  -- \
  psql \
    -h d8ms-pg-snapshot-pg-rw \
    -U snapshot-rw \
    -d snapshotdb \
    -c "
      CREATE TABLE snapshot_check (
        id integer PRIMARY KEY,
        value text NOT NULL
      );
      INSERT INTO snapshot_check VALUES (1, 'BEFORE_SNAPSHOT');
      SELECT * FROM snapshot_check;
    "
```

Пример вывода:

```console
 id |      value
----+-----------------
  1 | BEFORE_SNAPSHOT
(1 row)
```

Создайте ресурс PostgresSnapshot:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresSnapshot
metadata:
  name: snapshot-pg-backup
  namespace: postgres
spec:
  postgresName: snapshot-pg
```

Примените манифест:

```shell
d8 k apply -f snapshot-pg-backup.yaml
```

Проверьте состояние снимка:

```shell
d8 k get postgressnapshot snapshot-pg-backup -n postgres \
  -o jsonpath='{.status.phase}{"\n"}'
```

После успешного создания снимка команда возвращает:

```console
completed
```

Проверьте созданный VolumeSnapshot:

```shell
d8 k get volumesnapshot -n postgres
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                         READYTOUSE   SOURCEPVC               RESTORESIZE   SNAPSHOTCLASS
d8ms-pg-snapshot-pg-backup   true         d8ms-pg-snapshot-pg-1   2Gi           sds-local-volume-snapshot-class
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

`READYTOUSE=true` подтверждает готовность снимка к восстановлению.

После завершения создания снимка добавьте в исходную базу вторую контрольную строку:

```shell
PGPASSWORD="$(d8 k get secret snapshot-pg-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode)"

d8 k run snapshot-client \
  -n postgres \
  --rm -i \
  --restart=Never \
  --image=postgres:17 \
  --env="PGPASSWORD=$PGPASSWORD" \
  -- \
  psql \
    -h d8ms-pg-snapshot-pg-rw \
    -U snapshot-rw \
    -d snapshotdb \
    -c "
      INSERT INTO snapshot_check VALUES (2, 'AFTER_SNAPSHOT');
      SELECT * FROM snapshot_check ORDER BY id;
    "
```

Пример вывода:

```console
 id |      value
----+-----------------
  1 | BEFORE_SNAPSHOT
  2 | AFTER_SNAPSHOT
(2 rows)
```

### Восстановление из PostgresSnapshot

Для восстановления создайте новый ресурс Postgres и укажите созданный PostgresSnapshot в [`spec.dataSource.objectRef`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-datasource-objectref). Исходный Postgres удалять не требуется:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  name: snapshot-pg-restored
  namespace: postgres
spec:
  dataSource:
    objectRef:
      kind: PostgresSnapshot
      name: snapshot-pg-backup
  type: Standalone
  instance:
    cpu:
      cores: 1
      coreFraction: 50
    memory:
      size: 1Gi
    persistentVolumeClaim:
      size: 2Gi
      storageClassName: snapshot-local
```

Поля `type` и `instance` необходимо указывать явно — они не наследуются от исходного Postgres. После этого примените манифест:

```shell
d8 k apply -f snapshot-pg-restored.yaml
```

Дождитесь готовности восстановленного Postgres:

```shell
d8 k get postgres snapshot-pg-restored -n postgres -o wide -w
```

После запуска восстановленного PostgreSQL проверьте контрольную таблицу:

```shell
PGPASSWORD="$(d8 k get secret snapshot-pg-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode)"

d8 k run snapshot-restore-check \
  -n postgres \
  --rm -i \
  --restart=Never \
  --image=postgres:17 \
  --env="PGPASSWORD=$PGPASSWORD" \
  -- \
  psql \
    -h d8ms-pg-snapshot-pg-restored-rw \
    -U snapshot-rw \
    -d snapshotdb \
    -c "SELECT * FROM snapshot_check ORDER BY id;"
```

Пример вывода:

```console
 id |      value
----+-----------------
  1 | BEFORE_SNAPSHOT
(1 row)
```

Наличие только `BEFORE_SNAPSHOT` подтверждает, что восстановлено состояние базы данных на момент создания снимка.

## Проверка состояния

Состояние сервиса отражается в `status.conditions` ресурса Postgres.

Для краткой проверки используйте:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

Основные условия:

| Условие | Что показывает |
| :-- | :-- |
| `ConfigurationValid` | Конфигурация прошла проверки связанного PostgresClass |
| `LastValidConfigurationApplied` | Последняя валидная конфигурация применена |
| `ScaledToLastValidConfiguration` | Экземпляры соответствуют последней валидной конфигурации |
| `Available` | Сервис доступен |
| `UsersSynced` | Пользователи синхронизированы |
| `DatabasesSynced` | Логические базы данных синхронизированы |

Во время изменения ресурсов или параметров PostgreSQL часть условий может временно иметь значение `False`, в то время как `Available` остаётся `True`.

Для наблюдения за изменением состояния:

```shell
d8 k get postgres app-postgres -n postgres -o wide -w
```

Для просмотра подробностей:

```shell
d8 k get postgres app-postgres -n postgres -o yaml
```

Если сервис не переходит в готовое состояние, диагностика — [в разделе «Частые вопросы»](faq.html).