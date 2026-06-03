---
title: "managed-postgres"
permalink: ru/admin/configuration/managed-services/postgres.html
description: "Администрирование managed-сервиса PostgreSQL в Deckhouse Kubernetes Platform"
lang: ru
---

Managed-сервис PostgreSQL предоставляет кластеры PostgreSQL в Deckhouse Kubernetes Platform. Сервис находится на [стадии `Preview`](/products/kubernetes-platform/documentation/v1/architecture/module-development/versioning/#жизненный-цикл-модуля). Перед включением [`managed-postgres`](/modules/managed-postgres/) выполните [требования для установки](/modules/managed-postgres/configuration.html#требования). Основной cluster-wide-ресурс администратора — PostgresClass. Он определяет ограничения и значения по умолчанию для связанных ресурсов Postgres. Инструкции по созданию и использованию сервисов PostgreSQL — в разделе [«Использование managed PostgreSQL»](../../../user/managed-services/postgres.html).

## Перед началом работы

Убедитесь, что:

- [`managed-postgres`](/modules/managed-postgres/) доступен в используемой инсталляции;
- выполнены [требования для установки](/modules/managed-postgres/configuration.html#требования);
- у вас есть права на создание cluster-wide-ресурсов.

## Включение managed-postgres

Чтобы включить managed-сервис PostgreSQL, примените ресурс ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: managed-postgres
spec:
  enabled: true
  version: 1
```

После включения `managed-postgres` автоматически создаётся ресурс PostgresClass с именем `default`.

Также в системном неймспейсе `d8-managed-postgres` разворачивается контроллер, который согласовывает состояние ресурсов Postgres во всех пользовательских неймспейсах.

## Ресурс PostgresClass

Ресурс PostgresClass — это cluster-wide-ресурс. Он используется для:

- задания допустимой топологии PostgreSQL;
- задания ограничений на CPU и память;
- настройки значений конфигурации по умолчанию;
- определения параметров, которые пользователь может переопределить;
- добавления правил валидации.

Каждый ресурс Postgres должен ссылаться на существующий PostgresClass через параметр `spec.postgresClassName`.

## Настройка топологии

В PostgresClass можно ограничить допустимые варианты топологии и задать топологию по умолчанию.

Поддерживаются следующие значения:

- `Ignored`;
- `Zonal`;
- `TransZonal`.

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

Параметр `spec.sizingPolicies` определяет допустимые диапазоны CPU и памяти для связанных ресурсов Postgres.

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

Для PostgresClass можно задать правила валидации в параметре `spec.validations`. Поддерживается язык CEL.

Доступны следующие предопределённые переменные:

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

Параметр `spec.overridableConfiguration` определяет белый список параметров PostgreSQL, которые пользователь может задать в ресурсе Postgres.

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

В `spec.configuration` ресурса PostgresClass можно определить значения конфигурации PostgreSQL по умолчанию.

Если параметр разрешён в `overridableConfiguration` и задан в ресурсе Postgres, значение из Postgres имеет приоритет.

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

Для PostgresClass можно задать параметры планирования подов:

- `nodeAffinity`;
- `nodeSelector`;
- `tolerations`.

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

## Пример PostgresClass

Ниже приведён полный пример ресурса PostgresClass, который задаёт топологию, значения конфигурации, переопределяемые параметры, правила валидации и политики определения размера:

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

## Обратите внимание

{% alert level="warning" %}
Deckhouse Kubernetes Platform не удаляет связанные CRD при отключении [`managed-postgres`](/modules/managed-postgres/). Если эти ресурсы больше не нужны, удалите соответствующие CRD вручную.
{% endalert %}
