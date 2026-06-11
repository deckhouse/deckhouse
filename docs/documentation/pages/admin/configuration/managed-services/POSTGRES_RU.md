---
title: "Managed PostgreSQL"
permalink: ru/admin/configuration/managed-services/postgres.html
description: "Администрирование managed-сервиса PostgreSQL в Deckhouse Kubernetes Platform"
lang: ru
---

Managed PostgreSQL в Deckhouse Kubernetes Platform добавляет в кластер API для создания и сопровождения экземпляров PostgreSQL. Эта страница описывает административную настройку сервиса: включение модуля [`managed-postgres`](/modules/managed-postgres/) и подготовку классов PostgresClass для пользователей.

Перед включением [`managed-postgres`](/modules/managed-postgres/) выполните [требования для установки](/modules/managed-postgres/configuration.html#требования). Пользовательские операции с сервисами PostgreSQL описаны в разделе [«Использование Managed PostgreSQL»](../../../user/managed-services/postgres.html).

## Базовая настройка Managed PostgreSQL

Чтобы подготовить Managed PostgreSQL для пользователей:

1. [Включите модуль `managed-postgres`](/modules/managed-postgres/configuration.html) одним из способов, описанных на странице настроек модуля в блоке «Как явно включить или отключить модуль...».
2. Проверьте автоматически созданный PostgresClass `default` или подготовьте собственный PostgresClass.
3. [Определите доступные топологии и зоны размещения](#настройка-топологии).
4. [Настройте политики размера](#настройка-политик-определения-размера): диапазоны CPU, памяти и допустимые доли CPU.
5. Задайте [значения PostgreSQL-параметров по умолчанию](#настройка-значений-конфигурации-по-умолчанию) и [список параметров, доступных для переопределения](#настройка-параметров-доступных-для-переопределения).
6. При необходимости добавьте [правила валидации](#настройка-правил-валидации) и [параметры планирования подов](#настройка-планирования-подов).
7. Примените PostgresClass и передайте пользователям его имя для создания ресурсов Postgres.

Дальше описано, что происходит после включения модуля и как настроить PostgresClass.

## После включения Managed PostgreSQL

Модуль `managed-postgres` автоматически создаёт ресурс PostgresClass с именем `default`.

Также в системном неймспейсе `d8-managed-postgres` разворачивается контроллер, который согласовывает состояние ресурсов Postgres во всех пользовательских неймспейсах.

## Подготовка PostgresClass

Ресурс PostgresClass — это cluster-wide-ресурс, который описывает класс managed-сервиса PostgreSQL для пользовательских ресурсов Postgres. Используйте его, чтобы:

- задать допустимую топологию PostgreSQL;
- ограничить CPU и память;
- настроить значения конфигурации по умолчанию;
- определить параметры, которые пользователь может переопределить;
- добавить правила валидации.

Каждый ресурс Postgres должен ссылаться на существующий PostgresClass через параметр `spec.postgresClassName`.

Чтобы посмотреть автоматически созданный PostgresClass `default`, выполните команду:

```shell
d8 k get PostgresClass default -o yaml
```

Если требуется отдельная конфигурация PostgreSQL, подготовьте собственный манифест PostgresClass.

### Пример PostgresClass

Ниже приведён пример ресурса PostgresClass, который задаёт топологию, значения конфигурации, переопределяемые параметры, правила валидации и политики определения размера:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresClass
metadata:
  labels:
    app.kubernetes.io/name: managed-psql-operator
  name: new
spec:
  topology:
    allowedTopologies:
      - Zonal
      - TransZonal
      - Ignored
    allowedZones: []
    defaultTopology: Ignored
  configuration:
    maxConnections: 300
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize
  validations:
    - message: "Max connections should be more than 100"
      rule: "configuration.maxConnections > 100"
    - message: "Shared buffers should be less than 40% of memory.size"
      rule: "configuration.sharedBuffers * 100 < instance.memory.size * 40"
    - message: "walKeepSize can not be more than 1Gi"
      rule: "configuration.walKeepSize <= 1073741824"
  sizingPolicies:
    - cores:
        min: 1
        max: 3
      memory:
        min: 1Gi
        max: 5Gi
        step: 1Gi
      coreFractions:
        - 10
        - 20
        - 50
        - 100
    - cores:
        min: 4
        max: 10
      memory:
        min: 5Gi
        max: 15Gi
        step: 1Gi
      coreFractions:
        - 50
        - 100
```

Чтобы применить манифест PostgresClass, выполните команду:

```shell
d8 k apply -f postgresclass.yaml
```

## Настройка топологии

В PostgresClass можно ограничить допустимые топологии, задать топологию по умолчанию и определить список зон, доступных для размещения экземпляров PostgreSQL.

Топологии, доступные пользователям, перечисляются в параметре [`spec.topology.allowedTopologies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedtopologies) ресурса PostgresClass. Если в ресурсе Postgres не указан параметр [`spec.cluster.topology`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-topology), контроллер применяет значение из [`spec.topology.defaultTopology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-defaulttopology). Для топологий `Zonal` и `TransZonal` список доступных зон задаётся в параметре [`spec.topology.allowedZones`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedzones).

| Топология | Особенности | Где определяется |
| --- | --- | --- |
| `Ignored` | Размещение экземпляров PostgreSQL не привязывается к зонам. Используйте для кластеров без зонального деления или если зональная топология не важна. | Разрешается в `spec.topology.allowedTopologies`; может быть значением `spec.topology.defaultTopology`. |
| `Zonal` | Экземпляры PostgreSQL размещаются в одной зоне из списка `spec.topology.allowedZones`. | Разрешается в `spec.topology.allowedTopologies`; пользователь выбирает её в `spec.cluster.topology`. |
| `TransZonal` | Экземпляры PostgreSQL распределяются по нескольким зонам из списка `spec.topology.allowedZones`. | Разрешается в `spec.topology.allowedTopologies`; пользователь выбирает её в `spec.cluster.topology`. |

Пример:

```yaml
spec:
  topology:
    allowedTopologies:
      - Ignored
      - Zonal
      - TransZonal
    defaultTopology: TransZonal
    allowedZones:
      - zone-1
      - zone-2
      - zone-3
```

## Настройка политик определения размера

Администратор может управлять размерами экземпляров PostgreSQL, доступными пользователям: задавать диапазоны CPU и памяти, а также допустимые доли CPU. Это помогает ограничить потребление ресурсов в рамках выбранного PostgresClass и не допустить создания конфигураций, которые не соответствуют требованиям к сервису.

Политики размера задаются в параметре [`spec.sizingPolicies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies) ресурса PostgresClass.

Диапазоны `cores.min`–`cores.max` для разных политик не должны пересекаться.

Пример:

```yaml
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 4
      memory:
        min: 100Mi
        max: 1Gi
        step: 1Mi
      coreFractions:
        - 10
        - 30
        - 50
    - cores:
        min: 5
        max: 10
      memory:
        min: 500Mi
        max: 2Gi
      coreFractions:
        - 50
        - 70
        - 100
```

## Настройка правил валидации

Администратор может настроить дополнительные правила проверки итоговой конфигурации PostgreSQL. Такие правила позволяют отклонять ресурсы Postgres с нежелательными сочетаниями параметров, например если число подключений слишком велико для выбранного объёма памяти.

Правила валидации задаются в параметре [`spec.validations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-validations) ресурса PostgresClass. Для описания условий поддерживается язык CEL.

В правилах доступны значения PostgreSQL-параметров после применения `spec.configuration` и пользовательских переопределений, а также выбранный размер экземпляра:

- `configuration.maxConnections`;
- `configuration.workMem`;
- `configuration.sharedBuffers`;
- `configuration.walKeepSize`;
- `instance.memory.size`;
- `instance.cpu.cores`.

Пример:

```yaml
spec:
  validations:
    - message: "Max connections should not be more than 300"
      rule: "configuration.maxConnections < 300"
    - message: "Shared buffers should not be more than 25% of RAM"
      rule: "configuration.sharedBuffers < instance.memory.size / 4"
```

## Настройка параметров, доступных для переопределения

PostgresClass разделяет базовые значения PostgreSQL-параметров и право пользователя изменять их в своём ресурсе Postgres. Администратор может разрешить переопределение только тех параметров, которые пользователь действительно должен контролировать.

Список параметров, доступных для переопределения, задаётся в [`spec.overridableConfiguration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-overridableconfiguration). Эти же параметры можно использовать при настройке значений по умолчанию в `spec.configuration`.

Поддерживаются следующие значения:

- `maxConnections`;
- `sharedBuffers`;
- `workMem`;
- `walKeepSize`.

Пример:

```yaml
spec:
  overridableConfiguration:
    - maxConnections
    - workMem
```

## Настройка значений конфигурации по умолчанию

После выбора параметров, доступных для переопределения, администратор может задать базовую конфигурацию PostgreSQL для всех ресурсов Postgres, связанных с этим PostgresClass. Значения по умолчанию применяются автоматически и дают пользователю готовую конфигурацию без необходимости указывать каждый параметр вручную.

Значения по умолчанию задаются в [`spec.configuration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-configuration). Если параметр разрешён в `spec.overridableConfiguration` и задан в ресурсе Postgres, значение из Postgres имеет приоритет.

Пример:

```yaml
spec:
  configuration:
    maxConnections: 100
    workMem: 100Mi
```

Оператор задаёт следующие значения по умолчанию:

- `maxConnections`: `100`;
- `sharedBuffers`: 25% от `memory.size`;
- `workMem`: (`memory.size` - `sharedBuffers`) * 4 / `maxConnections`;
- `walKeepSize`: `512Mi`.

## Настройка планирования подов

Для управления размещением сервиса PostgreSQL на конкретных узлах администратор может использовать стандартные механизмы планирования Kubernetes: `nodeAffinity`, `nodeSelector` и `tolerations`. Это позволяет, например, размещать экземпляры PostgreSQL на выделенных узлах с нужными лейблами или разрешать планирование на узлы с taints.

Параметры планирования задаются в PostgresClass:

- [`spec.nodeAffinity`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeaffinity);
- [`spec.nodeSelector`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeselector);
- [`spec.tolerations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-tolerations).

### Пример nodeAffinity

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
```

### Пример tolerations

```yaml
spec:
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

### Пример nodeSelector

```yaml
spec:
  nodeSelector:
    "node.deckhouse.io/group": "pg"
```
