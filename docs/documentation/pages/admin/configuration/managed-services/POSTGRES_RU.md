---
title: "Managed PostgreSQL"
permalink: ru/admin/configuration/managed-services/postgres/
description: "Администрирование managed-сервиса PostgreSQL в Deckhouse Kubernetes Platform"
lang: ru
---

Managed-сервис PostgreSQL позволяет запускать управляемые кластеры PostgreSQL в Deckhouse Kubernetes Platform. DKP берёт на себя управление жизненным циклом экземпляров PostgreSQL: развёртывание, масштабирование, резервное копирование и обновление. Пользователи managed-сервиса описывают желаемый экземпляр PostgreSQL в неймспейсе, а DKP создаёт и поддерживает его в нужном состоянии.

На этой странице описана настройка и управление сервиса администратором DKP. Пользовательские операции с сервисом описаны в разделе [Использование → Managed-сервисы → Managed PostgreSQL](../../../user/managed-services/postgres.html).   

## Включение

Для того чтобы включить managed-сервис PostgreSQL в кластере, выполните следующие шаги:
1. [Включите модуль `managed-postgres`](/modules/managed-postgres/configuration.html#enable).

1. Проверьте, что в кластере появились ресурсы и служебные компоненты Managed PostgreSQL.

   Модуль автоматически создаёт PostgresClass `default`. Проверить его наличие можно с помощью команды:

   ```shell
   d8 k get postgresclass default
   ```

   Пример вывода:
   
   ```console
   $ d8 k get postgresclass default
   NAME      AGE
   default   20s         
   ```

   Также в системном пространстве имён `d8-managed-postgres` разворачивается оператор. Проверить его наличие можно с помощью команды:

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

## Настройка

Для создания экземпляра PostgreSQL, пользователь использует кастомный ресурс [Postgres](/modules/managed-postgres/cr.html#postgres), в котором указывает имя PostgresClass в параметре [postgresClassName](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-postgresclassname).

PostgresClass позволяет определить:
- допустимую топологию PostgreSQL;
- ограничения по CPU и памяти;
- правила валидации;
- значения конфигурации по умолчанию, и параметры, которые пользователь может переопределять.

По умолчанию, модуль создаёт PostgresClass `default`, который можно изменить, но администратор может создать отдельные PostgresClass с различными конфигурациями.

Чтобы посмотреть параметры автоматически созданного PostgresClass `default`, выполните команду:

```shell
d8 k get PostgresClass default -o yaml
```

### Настройка топологии

Администратор может управлять размещением экземпляров PostgreSQL по зонам и узлам кластера (топологией) — ограничивать доступные топологии, задавать топологию по умолчанию и определять зоны для размещения экземпляров PostgreSQL.

Доступные топологии:

| Топология | Особенности | Где определяется                                                                                 |
| --- | --- |--------------------------------------------------------------------------------------------------|
| `Ignored` | Размещение экземпляров PostgreSQL не привязывается к зонам. Используйте для кластеров без зонального деления или если зональная топология не важна. | В `spec.topology.allowedTopologies`; также может быть значением `spec.topology.defaultTopology`. |
| `Zonal` | Экземпляры PostgreSQL размещаются в одной зоне из списка `spec.topology.allowedZones`. | В `spec.topology.allowedTopologies`; пользователь выбирает её в `spec.cluster.topology`.         |
| `TransZonal` | Экземпляры PostgreSQL распределяются по нескольким зонам из списка `spec.topology.allowedZones`. | В `spec.topology.allowedTopologies`; пользователь выбирает её в `spec.cluster.topology`.         |

Топологии, доступные пользователям, перечисляются в параметре [`spec.topology.allowedTopologies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedtopologies) PostgresClass. Если в объекте Postgres пользователя не указан параметр [`spec.cluster.topology`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-topology), ипользуется значение из [`spec.topology.defaultTopology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-defaulttopology) PostgresClass. Для топологий `Zonal` и `TransZonal` список доступных зон задаётся в параметре [`spec.topology.allowedZones`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedzones).

Пример настройки доступных для использования пользователем топологий в PostgresClass:

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

В примере выше пользователи могут выбрать любую из трёх доступных топологий. Если пользователь не укажет топологию в ресурсе Postgres, будет использована топология `TransZonal`. Для топологий `Zonal` и `TransZonal` пользователи смогут выбрать только зоны `zone-1`, `zone-2` и `zone-3`.

### Настройка ограничений по CPU и памяти

Чтобы ограничить выделение CPU и памяти, доступные экземплярам PostgreSQL пользователя, используйте секцию параметров [`spec.sizingPolicies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies) PostgresClass. Эти параметры позволяют создавать набор политик определения размера связанных экземпляров PostgreSQL, позволяя избегать неравномерного распределения ресурсов на узлах кластера.

Пользователь указывает ресурсы по CPU и памяти в параметрах [`spec.instance.cpu`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-cpu) и [`spec.instance.memory`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-memory) объекта Postgres. Значения этих параметров должны попадать в диапазоны, заданные в соответствующем PostgresClass.

В `spec.sizingPolicies` PostgresClass определяется одна или несколько политик ограничений по CPU и памяти, в каждой из которой задаются следующие параметры:

- [spec.sizingPolicies.cores.min](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-cores-min) и [spec.sizingPolicies.cores.max](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-cores-max) — минимальное и максимальное количество CPU. На основании этих значений выбирается политика ограничения, после чего уже проверяется соответствие ей остальных значений [`spec.instance.cpu`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-cpu) и [`spec.instance.memory`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-memory), указанных пользователем в объекте Postgres.

  {% alert level="warning" %}Диапазоны `cores.min`–`cores.max` для разных политик не должны пересекаться.{% endalert %}

- [spec.sizingPolicies.coreFractions](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-corefractions) — список допустимых для указания пользователем в объекте Postgres множителей, используемых для расчёта `requests` на основе заданных `limits` в CPU.
- [spec.sizingPolicies.memory.min](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-memory-min), [spec.sizingPolicies.memory.max](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-memory-max) и [spec.sizingPolicies.memory.step](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-memory-step) — минимальный и максимальный объём памяти которое пользователь может указать в объекте Postgres, а также шаг допустимого значения (указанное пользователем значение должно делиться на него без остатка);

Пример настройки ограничений по CPU и памяти в PostgresClass:

```yaml
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 4
      memory:
        min: 100Mi
        max: 1Gi
        step: 50Mi
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
        step: 100Mi
      coreFractions:
        - 50
        - 70
        - 100
```

В примере выше пользователи, создающие экземпляры PostgreSQL на основе этого класса, могут указывать от 1 до 10 CPU в [`spec.instance.cpu`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-cpu) Postgres. Но, например, для 4 и менее CPU можно указать от 100Mi до 1Gi памяти (с шагом 50Mi), а для от 5 до 10 CPU — от 500Mi до 2Gi памяти (с шагом 100Mi).

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
