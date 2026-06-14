---
title: "Managed PostgreSQL"
permalink: ru/admin/configuration/managed-services/postgres.html
description: "Администрирование managed-сервиса PostgreSQL в Deckhouse Kubernetes Platform"
lang: ru
---

Managed PostgreSQL в Deckhouse Kubernetes Platform добавляет в кластер API для создания и сопровождения экземпляров PostgreSQL. Эта страница описывает административную настройку сервиса: включение модуля [`managed-postgres`](/modules/managed-postgres/) и подготовку классов PostgresClass для пользователей.

Перед включением `managed-postgres` проверьте [требования для установки](/modules/managed-postgres/configuration.html#требования). Пользовательские операции с сервисом PostgreSQL описаны в разделе [«Использование Managed PostgreSQL»](../../../user/managed-services/postgres.html).

## Включение модуля managed-postgres

Чтобы включить модуль `managed-postgres`, создайте файл `module-config.yaml` с манифестом ModuleConfig `managed-postgres`. Если такой ресурс уже существует, проверьте, что в нём параметр `spec.enabled` установлен в `true`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: managed-postgres
spec:
  enabled: true
```

Примените манифест:

```shell
d8 k apply -f module-config.yaml
```

Проверьте, что модуль перешёл в состояние `Ready`:

```shell
d8 k get module managed-postgres
```

Пример вывода:

```console
$ d8 k get module managed-postgres
NAME               STAGE   SOURCE   PHASE   ENABLED   READY
managed-postgres                    Ready   True      True
```

## Действия после включения модуля

После перехода модуля `managed-postgres` в состояние `Ready` проверьте, что в кластере появились ресурсы и служебные компоненты Managed PostgreSQL.

Модуль автоматически создаёт PostgresClass с именем `default`:

```shell
d8 k get postgresclass default
```

Пример вывода:

```console
$ d8 k get postgresclass default
NAME      AGE
default   20s         
```

Также в системном пространстве имён `d8-managed-postgres` разворачивается контроллер, который согласовывает состояние ресурсов Postgres в пользовательских пространствах имён:

```shell
d8 k -n d8-managed-postgres get pods
```

Пример вывода:

```console
d8 k -n d8-managed-postgres get pods
NAME                                         READY   STATUS    RESTARTS   AGE
d8-cnpg-operator-79b448c5bf-zv8d9            1/1     Running   0          4m
managed-postgres-operator-5dbcbf96b5-8mqqt   1/1     Running   0          4m        
```

Далее подготовьте PostgresClass, который пользователи будут указывать в параметре `spec.postgresClassName` ресурсов Postgres:

1. Проверьте ограничения и значения по умолчанию в автоматически созданном PostgresClass `default`.
1. Если PostgresClass `default` подходит для пользовательских ресурсов Postgres, передайте пользователям имя `default`.
1. Если требуется отдельная конфигурация PostgreSQL, создайте свой PostgresClass и передайте пользователям его имя.

## Подготовка PostgresClass

PostgresClass — это ресурс уровня кластера, который описывает класс managed-сервиса PostgreSQL для пользовательских ресурсов Postgres. Настройте его, чтобы:

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

При проверке PostgresClass `default` обратите внимание на топологию, политики размера, значения PostgreSQL-параметров по умолчанию, параметры, доступные для переопределения, правила валидации и параметры размещения подов.

Если настройки не соответствуют вашим требованиям, отредактируйте PostgresClass `default` с учётом примеров ниже:

```shell
d8 k edit PostgresClass default
```

Примеры показывают отдельные фрагменты `spec`; полный манифест приведён в [примере полного манифеста PostgresClass](#пример-полного-манифеста-postgresclass).

### Настройка топологии

В PostgresClass можно ограничить доступные топологии, задать топологию по умолчанию и определить зоны для размещения экземпляров PostgreSQL.

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

### Настройка политик размера

Используйте политики размера, чтобы ограничить вычислительные ресурсы, доступные пользователям. Политики определяют правила выделения CPU и памяти экземплярам Postgres.

Политики размера задаются в обязательном параметре [`spec.sizingPolicies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies) ресурса PostgresClass.

Диапазоны `cores.min`–`cores.max` для разных политик не должны пересекаться.

В каждой политике задайте:

- `cores.min` и `cores.max` — минимальное и максимальное количество CPU;
- `memory.min` и `memory.max` — минимальный и максимальный объём памяти;
- `memory.step` — шаг допустимого значения памяти: выбранный объём должен делиться на него без остатка;
- `coreFractions` — множители для расчёта `requests` на основе заданных `limits` в CPU.

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

### Настройка правил валидации

Используйте правила валидации, чтобы отклонять ресурсы Postgres с недопустимыми сочетаниями параметров. Например, правило может ограничивать число подключений в зависимости от выбранного объёма памяти.

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

### Настройка параметров, доступных для переопределения

PostgresClass разделяет базовые значения PostgreSQL-параметров и право пользователя изменять их в ресурсе Postgres. Разрешайте переопределение только тех параметров, которые пользователь должен контролировать самостоятельно.

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

### Настройка значений конфигурации по умолчанию

Задайте базовую конфигурацию PostgreSQL для всех ресурсов Postgres, которые ссылаются на PostgresClass. Значения по умолчанию применяются автоматически, поэтому пользователю не нужно указывать каждый параметр вручную.

Значения по умолчанию задаются в [`spec.configuration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-configuration). Если параметр разрешён в `spec.overridableConfiguration` и задан в ресурсе Postgres, значение из Postgres имеет приоритет.

Контроллер модуля задаёт следующие значения по умолчанию:

- `maxConnections`: `100`;
- `sharedBuffers`: 25% от `memory.size`;
- `workMem`: (`memory.size` - `sharedBuffers`) * 4 / `maxConnections`;
- `walKeepSize`: `512Mi`.

Пример:

```yaml
spec:
  configuration:
    maxConnections: 100
    workMem: 100Mi
```

### Настройка размещения подов

Для размещения PostgreSQL на конкретных узлах используйте стандартные механизмы планирования Kubernetes: `nodeAffinity`, `nodeSelector` и `tolerations`. Например, так можно размещать экземпляры PostgreSQL на выделенных узлах с нужными лейблами или разрешать запуск на узлах с `taint`.

Параметры размещения задаются в PostgresClass:

- [`spec.nodeAffinity`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeaffinity);
- [`spec.nodeSelector`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeselector);
- [`spec.tolerations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-tolerations).

#### Пример nodeAffinity

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

#### Пример nodeSelector

```yaml
spec:
  nodeSelector:
    "node.deckhouse.io/group": "pg"
```

#### Пример tolerations

```yaml
spec:
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

### Пример полного манифеста PostgresClass

После выбора параметров создайте файл `postgresclass.yaml` с манифестом PostgresClass.

Пример манифеста, который задаёт топологию, политики размера, правила валидации, значения конфигурации, переопределяемые параметры и параметры размещения подов:

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
  validations:
    - message: "Max connections should be more than 100"
      rule: "configuration.maxConnections > 100"
    - message: "Shared buffers should be less than 40% of memory.size"
      rule: "configuration.sharedBuffers * 100 < instance.memory.size * 40"
    - message: "walKeepSize can not be more than 1Gi"
      rule: "configuration.walKeepSize <= 1073741824"
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize
  configuration:
    maxConnections: 300
  nodeSelector:
    "node.deckhouse.io/group": "pg"
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

Чтобы применить манифест PostgresClass, выполните команду:

```shell
d8 k apply -f postgresclass.yaml
```
