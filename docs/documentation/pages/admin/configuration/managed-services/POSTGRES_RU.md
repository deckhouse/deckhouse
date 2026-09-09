---
title: "Managed PostgreSQL"
permalink: ru/admin/configuration/managed-services/postgres/
description: "Администрирование managed-сервиса PostgreSQL в Deckhouse Kubernetes Platform"
lang: ru
---

Сервис Managed PostgreSQL помогает разворачивать и поддерживать PostgreSQL в DKP. [Поддерживаемая версия PostgreSQL](/modules/managed-postgres/user_guide.html#%D0%BF%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%B8%D0%B2%D0%B0%D0%B5%D0%BC%D1%8B%D0%B5-%D0%B2%D0%B5%D1%80%D1%81%D0%B8%D0%B8-postgresql) — 17.6.

На этой странице описано, что и как может настраивать администратор кластера: управление ограничениями ресурсов и топологией, настройка проверки параметров, установка значений по умолчанию и привязка к узлам кластера.

## Включение сервиса

Чтобы включить в DKP сервис Managed PostgreSQL, выполните следующие шаги:

1. Убедитесь, что выполнены [требования к работе модуля `managed-postgres`](/modules/managed-postgres/configuration.html#требования).
1. [Включите модуль `managed-postgres`](/modules/managed-postgres/configuration.html#enable) в web-интерфейсе DKP или другими способами.
1. Создайте [PostgresClass](/modules/managed-postgres/cr.html#postgresclass-v1alpha1) с необходимыми настройками, которые будут использоваться пользователями при создании объектов Postgres. Также можно использовать PostgresClass `default`, создаваемый модулем при включении. PostgresClass `default` содержит базовые настройки, чтобы пользователи могли сразу после включения Managed PostgreSQL в DKP создавать базы данных. Для production-окружений рекомендуется создавать отдельные PostgresClass с явными настройками и ограничениями.

После включения модуля пользователи смогут сами создавать базы данных PostgreSQL. Пользовательские операции с сервисом описаны [в разделе «Использование» →
«Managed-сервисы» → «Managed PostgreSQL»](../../../user/managed-services/postgres.html).

## Зависимости для отдельных функций

Некоторые функции модуля требуют дополнительной настройки компонентов Deckhouse Kubernetes Platform или инфраструктуры кластера:

| Функция                                                  | Требование                                                                                                                                                                                 |
|----------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Резервное копирование (управляется [PostgresSnapshot](/modules/managed-postgres/cr.html#postgressnapshot)) | Включённый модуль [snapshot-controller](/modules/snapshot-controller/) и StorageClass с поддержкой снимков                                                                                                              |
| TLS через `cert-manager`                                 | Включённый модуль [cert-manager](/modules/cert-manager/) и настроенный ClusterIssuer или Issuer                                                                                            |  |
| Размещение на выделенных узлах                           | Лейблы на узлах (например, `node.deckhouse.io/group=pg`) и taints при необходимости. Подробнее — [в документации](../../../admin/configuration/platform-scaling/node/node-management.html) |

## Пример создания PostgresClass

Перед применением примера убедитесь, что в кластере достаточно ресурсов. Чтобы оценить доступные ресурсы, выполните:

```shell
d8 k describe node worker-0 | grep -A 5 "Allocated resources"
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests          Limits
  --------           --------          ------
  cpu                3176m (80%)       500m (12%)
  memory             8342837084 (71%)  6400Mi (57%)
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Ниже приведён пример PostgresClass `production-v1`, который можно использовать вместо PostgresClass `default` и адаптировать под требования к ресурсам, топологии, параметрам PostgreSQL и размещению на узлах. Отдельные параметры примера подробнее описаны в следующих разделах.

Создайте файл `postgresclass-production.yaml` со следующим содержимым:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresClass
metadata:
  name: production-v1
spec:
  # Ограничение ресурсов CPU и памяти.
  sizingPolicies:
    - cores:
        min: 1
        max: 2
      memory:
        min: 512Mi
        max: 2Gi
        step: 512Mi
      coreFractions:
        - 50
        - 100
    - cores:
        min: 3
        max: 4
      memory:
        min: 2Gi
        max: 4Gi
        step: 1Gi
      coreFractions:
        - 50
        - 100

  # Управление отказоустойчивостью.
  topology:
    allowedTopologies:
      - Ignored
      - Zonal
      - TransZonal
    defaultTopology: TransZonal
    allowedZones:
      - zone-a
      - zone-b
      - zone-c

  # Проверка настроек PostgreSQL.
  validations:
    - message: "Max connections should not be more than 300"
      rule: "configuration.maxConnections <= 300"
    - message: "Shared buffers should not be more than 25% of RAM"
      rule: "configuration.sharedBuffers < instance.memory.size / 4"
    - message: "walKeepSize can not be more than 1Gi"
      rule: "configuration.walKeepSize <= 1073741824"

  # Параметры по умолчанию и разрешения на переопределение.
  configuration:
    maxConnections: 200
    sharedBuffers: 1Gi
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize

  # Привязка к выделенным узлам.
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: "node.deckhouse.io/group"
              operator: "In"
              values:
                - "pg"
                - "postgres"
  nodeSelector:
    "node.deckhouse.io/group": "pg"
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

Примените манифест:

```shell
d8 k apply -f postgresclass-production.yaml
```

Проверьте созданный PostgresClass:

```shell
d8 k describe postgresclass production-v1
```

{% offtopic title="Пример вывода..." %}

```console
Name:         production-v1
Namespace:    
Labels:       <none>
Annotations:  <none>
API Version:  managed-services.deckhouse.io/v1alpha1
Kind:         PostgresClass
Metadata:
  Creation Timestamp:  2026-08-06T13:08:24Z
  Generation:          1
  Resource Version:    21610964
  UID:                 f2548e11-a23c-4bd1-b786-776bf1677a1d
Spec:
  Configuration:
    Max Connections:  200
    Shared Buffers:   1Gi
  Node Affinity:
    Required During Scheduling Ignored During Execution:
      Node Selector Terms:
        Match Expressions:
          Key:       node.deckhouse.io/group
          Operator:  In
          Values:
            pg
            postgres
  Node Selector:
    node.deckhouse.io/group:  pg
  Overridable Configuration:
    maxConnections
    sharedBuffers
    walKeepSize
  Sizing Policies:
    Core Fractions:
      50
      100
    Cores:
      Max:  2
      Min:  1
    Memory:
      Max:   2Gi
      Min:   512Mi
      Step:  512Mi
    Core Fractions:
      50
      100
    Cores:
      Max:  4
      Min:  3
    Memory:
      Max:   4Gi
      Min:   2Gi
      Step:  1Gi
  Tolerations:
    Effect:    NoSchedule
    Key:       primary-role
    Operator:  Equal
    Value:     pg
  Topology:
    Allowed Topologies:
      Ignored
      Zonal
      TransZonal
    Allowed Zones:
      zone-a
      zone-b
      zone-c
    Default Topology:  TransZonal
  Validations:
    Message:  Max connections should not be more than 300
    Rule:     configuration.maxConnections <= 300
    Message:  Shared buffers should not be more than 25% of RAM
    Rule:     configuration.sharedBuffers < instance.memory.size / 4
    Message:  walKeepSize can not be more than 1Gi
    Rule:     configuration.walKeepSize <= 1073741824
Events:       <none>
```

{% endofftopic %}

Пользователи могут использовать созданный PostgresClass, указывая его в параметре [postgresClassName](/modules/managed-postgres/stable/cr.html#postgres-v1alpha1-spec-postgresclassname) Postgres.

{% alert level="warning" %}
После создания PostgresClass его параметры нельзя изменить. Создайте новый PostgresClass с другим именем, чтобы использовать другие настройки.
{% endalert %}

## Изменение и удаление PostgresClass

Чтобы удалить старый класс, сначала проверьте, используется ли он существующими объектами Postgres. Выполните команду:

```shell
d8 k get postgres --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,CLASS:.spec.postgresClassName | grep production-v1
```

Если найдены объекты Postgres, использующие этот PostgresClass, учитывайте следующее:
- Базы данные, созданные на основе удалённого PostgresClass, продолжат работать. Их настройки остаются прежними, так как они были зафиксированы при создании.
- Пользователи не смогут создать новый объект Postgres со ссылкой на удалённый класс.

Если PostgresClass больше не используется его можно удалить.

Пример удаления PostgresClass `production-v1` через CLI:

```shell
d8 k delete postgresclass production-v1
```

Пример вывода:

```console
postgresclass.managed-services.deckhouse.io "production-v1" deleted
```

{% alert level="info" %}
При необходимости изменения настроек PostgresClass рекомендуется создавать новый с другим именем, а не пересоздавать PostgresClass с прежним именем.
{% endalert %}

## Ограничение ресурсов CPU и памяти

Для определения допустимых комбинаций CPU и памяти экземпляров PostgreSQL используются политики, в секции параметров [`spec.sizingPolicies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies). Администратор может задать несколько диапазонов ядер, для каждого из которых определены минимальная и максимальная память, а также шаг. Эти политики полезно применять, когда в одном кластере работают несколько независимых команд, создающих объекты Postgres без согласования с администратором. Политики предотвращают создание объектов Postgres с нереалистичными ресурсами и обеспечивают предсказуемую утилизацию узлов.

Выбор политики происходит по количеству ядер CPU. Параметр [`coreFractions`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-corefractions) определяет, какой процент от лимитов CPU (`limits`) составит гарантированный запрос (`requests`). Например, если администратор указал `coreFractions: [50, 100]`, пользователь при создании объекта Postgres может выбрать `coreFraction: 50` или `coreFraction: 100`.

При `coreFraction: 50`:

- `limits` CPU будут равны количеству запрошенных ядер (например, 4 ядра).
- `requests` CPU составят 50% от `limits` (2 ядра).

Планировщик гарантирует поду 2 ядра, но позволит использовать до 4, если они доступны. Это повышает плотность размещения подов на узлах.

При `coreFraction: 100`:

- `limits` и `requests` равны (4 ядра).
- Под получает гарантированное выделение ресурсов, но теряется возможность переиспользовать неиспользуемые ядра другими подами.

Ниже приведён фрагмент манифеста PostgresClass, с указанием допустимых комбинаций CPU и памяти. Полный вариант приведён [в разделе «Пример создания PostgresClass»](#пример-создания-postgresclass).

```yaml
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 2
      memory:
        min: 512Mi
        max: 2Gi
        step: 512Mi
      coreFractions:
        - 50
        - 100
```

В этом фрагменте для 1–2 ядер доступны значения памяти 512Mi, 1Gi, 1.5Gi и 2Gi.

Параметр `step` определяет шаг изменения объёма памяти. Пользователь может указать только то значение памяти, которое кратно `step`. Например, если `step: 512Mi`, допустимы значения 512Mi, 1Gi, 1.5Gi, 2Gi и так далее. Значение 700Mi будет отклонено, так как оно не кратно 512Mi.

Выбор памяти зависит от количества ядер. Если пользователь запросит 5 ядер, такой объект Postgres не будет создан, так как эта конфигурация не предусмотрена администратором.

{% alert level="warning" %}
В одном PostgresClass диапазоны `cores.min`–`cores.max` у разных политик не должны пересекаться.
{% endalert %}

Например, в PostgresClass `production-v1` первая политика не может покрывать 1–4 ядра, а вторая — 2–6 ядер, так как ядра 2–4 принадлежат обеим политикам. В разных PostgresClass, например `production-v1` и `production-v2`, диапазоны пересекаться могут.

Пользователь может увидеть доступные варианты и выбрать подходящий при создании объекта Postgres.

## Управление отказоустойчивостью через зоны доступности

Администратор может управлять отказоустойчивостью экземпляров PostgreSQL пользователей, задавая топологию их распределения по зонам доступности в секции параметров [`spec.topology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology). Это может быть полезно в production-окружениях с требованиями к отказоустойчивости уровня дата-центра.

Доступны три топологии:

- `Ignored` — стандартное планирование без привязки к зонам;
- `Zonal` — все экземпляры размещаются в одной зоне (минимальная задержка между репликами). Подходит для сред с низкой задержкой, где потеря зоны допустима;
- `TransZonal` — экземпляры распределяются по разным зонам (один основной экземпляр, одна синхронная реплика, одна асинхронная реплика). Защищает от падения целой зоны, но требует больше ресурсов.

Администратор может указать разрешённые варианты топологий ([`allowedTopologies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedtopologies)), топологию по умолчанию ([`defaultTopology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-defaulttopology)) и список доступных зон ([`allowedZones`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedzones)).

Ниже приведён фрагмент манифеста PostgresClass, с указанием параметров топологии. Полный вариант приведён [в разделе «Пример создания PostgresClass»](#пример-создания-postgresclass).

```yaml
spec:
  topology:
    allowedTopologies:
      - Zonal
      - TransZonal
    defaultTopology: TransZonal
    allowedZones:
      - zone-a
      - zone-b
```

В этом фрагменте разрешены режимы `Zonal` и `TransZonal`, а по умолчанию применяется `TransZonal`. В поле `allowedZones` указаны абстрактные зоны — замените их на реальные названия зон из используемого облачного провайдера или дата-центра.

Пользователь может выбрать уровень отказоустойчивости при создании объекта Postgres. Если не указать явно, применится значение по умолчанию (указанное в параметре `defaultTopology`).

## Автоматическая проверка настроек PostgreSQL

Для запрещения использования пользователями нежелательные комбинации параметров PostgreSQL, администратор может задать правила валидации на языке Common Expression Language (далее — CEL) в секции параметров [`spec.validations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-validations) PostgresClass. Эти правила проверяются в API-сервере до создания объектов Postgres.

Эти правила полезно применять для защиты от типичных ошибок конфигурации, которые могут привести к проблемам производительности. Например, слишком большое значение `shared_buffers` может оставить PostgreSQL недостаточно памяти для других операций, а большое количество подключений — увеличить общее потребление памяти.

В правилах доступны следующие переменные:

- `configuration.maxConnections`;
- `configuration.workMem`;
- `configuration.sharedBuffers`;
- `configuration.walKeepSize`;
- `instance.memory.size`;
- `instance.cpu.cores`.

Ниже приведён фрагмент манифеста PostgresClass, с указанием правил валидации. Полный набор правил приведён [в разделе «Пример создания PostgresClass»](#пример-создания-postgresclass).

```yaml
spec:
  validations:
    - message: "Shared buffers should not be more than 25% of RAM"
      rule: "configuration.sharedBuffers < instance.memory.size / 4"
```

В этом фрагменте правило запрещает выделять под `shared_buffers` более 25% от запрошенной памяти.

Пользователь получает ошибку валидации при попытке применить некорректные параметры, что помогает избежать ошибок конфигурации без ручной проверки.

## Задание настроек по умолчанию и их переопределение

Для задания стандартных параметров PostgreSQL и определения, какие из них пользователь может менять, администратор может использовать параметры [`spec.configuration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-configuration) и [`spec.overridableConfiguration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-overridableconfiguration).

Если значения в `configuration` не указаны, контроллер модуля применяет следующие:

- `maxConnections`: `100`;
- `sharedBuffers`: 25% от `memory.size`;
- `workMem`: `(memory.size - sharedBuffers) * 4 / maxConnections`;
- `walKeepSize`: `512Mi`.

Ниже приведён фрагмент манифеста PostgresClass, с указанием настроек сервиса по умолчанию. Полный вариант приведён [в разделе «Пример создания PostgresClass»](#пример-создания-postgresclass).

```yaml
spec:
  configuration:
    maxConnections: 200
    sharedBuffers: 1Gi
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize
```

В этом примере:

- `maxConnections` и `sharedBuffers` по умолчанию равны 200 и 1Gi соответственно.
- Пользователь может переопределить `maxConnections`, `sharedBuffers` и `walKeepSize`.
- `workMem` не входит в `overridableConfiguration` — пользователь не может его изменить.

### Автоматический расчёт workMem

Для установки ограничения использования памяти операций сортировки и хеширования внутри одного запроса, используйте параметр [workMem](/modules/managed-postgres/stable/cr.html#postgres-v1alpha1-spec-configuration-workmem).

В PostgreSQL этот лимит применяется к каждой операции в каждой активной сессии. Сложный запрос может выполнять несколько таких операций одновременно. При большом количестве подключений суммарное потребление памяти может многократно превысить заданное значение и привести к исчерпанию памяти. Слишком маленькое значение, напротив, заставит PostgreSQL использовать временные файлы на диске и снизит производительность.

Если `workMem` не задан явно, модуль `managed-postgres` автоматически рассчитывает его значение на основе выделенных ресурсов.

Модуль рассчитывает `workMem` по следующей формуле:

```text
workMem = (instance.memory.size - configuration.sharedBuffers) * 4 / configuration.maxConnections
```

Например, для Postgres со следующими параметрами:

- `instance.memory.size: 4Gi`
- `configuration.sharedBuffers: 1Gi`
- `configuration.maxConnections: 200`

Расчёт `workMem` будет выглядеть следующим образом:

```text
workMem = (4Gi - 1Gi) * 4 / 200
workMem = 3Gi * 4 / 200
workMem = 12Gi / 200
workMem = 61.44Mi
```

Множитель `* 4` — коэффициент запаса на случай нескольких одновременных операций сортировки и хеширования в рамках одного запроса.

Контроллер рассчитывает `workMem` по приведённой выше формуле с учётом объёма памяти экземпляра, `sharedBuffers`, `maxConnections` и коэффициента запаса.

## Привязка к выделенным узлам

Стандартные механизмы Kubernetes — [`spec.nodeSelector`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeselector), [`spec.tolerations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-tolerations) и [`spec.nodeAffinity`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeaffinity) — позволяют указать, на каких узлах могут размещаться поды PostgreSQL.

Размещение на выделенных узлах помогает изолировать экземпляры PostgreSQL от пользовательских приложений и сделать использование ресурсов дисковой подсистемы и сети более предсказуемым.

Ниже приведён фрагмент манифеста PostgresClass, с указанием `spec.nodeAffinity`, `spec.nodeSelector` и `spec.tolerations`. Полный вариант приведён [в разделе «Пример создания PostgresClass»](#пример-создания-postgresclass).

```yaml
spec:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: "node.deckhouse.io/group"
              operator: "In"
              values:
                - "pg"
  nodeSelector:
    "node.deckhouse.io/group": "pg"
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

В этом примере:

- Поды размещаются только на узлах с лейблом `node.deckhouse.io/group=pg`.
- На узлах установлен taint `primary-role=pg:NoSchedule`, который препятствует размещению подов без соответствующего toleration.
- `tolerations` разрешает размещение подов PostgreSQL на таких узлах.

Пользователь не заботится о выборе узлов. Поды автоматически размещаются на подготовленной инфраструктуре в соответствии с настройками PostgresClass.

Если правилам размещения не соответствует ни один узел, объекты Postgres пользователей останутся в состоянии `Pending`. Для поиска причины можно проверить события в неймспейсе , информацию об объекте Postgres и состояние PVC:

```shell
d8 k describe pods -n my-postgres <POD_NAME>
d8 k get events -n my-postgres --sort-by=.lastTimestamp
d8 k get pvc -n my-postgres
```
