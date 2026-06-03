---
title: "managed-postgres"
permalink: ru/user/managed-services/postgres.html
description: "Использование managed-сервиса PostgreSQL в Deckhouse Kubernetes Platform"
lang: ru
---

Для работы с managed-сервисом PostgreSQL используется namespaced-ресурс Postgres. Он описывает желаемое состояние сервиса PostgreSQL, включая:

- вычислительные ресурсы;
- размер хранилища;
- тип развёртывания;
- параметры топологии и репликации;
- пользователей;
- логические базы данных;
- источник данных для восстановления.

Ресурс Postgres должен ссылаться на существующий [PostgresClass](../../../admin/configuration/managed-services/postgres.html) через параметр `spec.postgresClassName`. Настройка PostgresClass выполняется администратором кластера.

## Перед началом работы

Убедитесь, что:

- [`managed-postgres`](/modules/managed-postgres/) включён;
- в кластере существует подходящий ресурс [PostgresClass](../../../admin/configuration/managed-services/postgres.html);
- у вас есть права на создание ресурсов в целевом неймспейсе.

## Создание сервиса PostgreSQL

Ниже приведён базовый пример создания ресурса Postgres:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  labels:
    app.kubernetes.io/name: managed-psql-operator
  name: test
spec:
  users:
    - name: test-rw
      password: '123'
      role: rw
  databases:
    - name: "testdb"
  postgresClassName: default
  instance:
    memory:
      size: 4Gi
    cpu:
      cores: 2
      coreFraction: 50
    persistentVolumeClaim:
      size: 10Gi
  type: Cluster
  cluster:
    topology: TransZonal
    replication: ConsistencyAndAvailability
```

Примените манифест в нужном неймспейсе:

```shell
d8 k apply -f managed-services_v1alpha1_postgres.yaml -n postgres
```

Проверьте состояние ресурса:

```shell
d8 k get postgres test -n postgres -o wide -w
```

Для проверки работоспособности сервиса убедитесь, что все значения в `status.conditions` имеют статус `True`.

## Обязательные параметры ресурса Postgres

Для ресурса Postgres обязательны как минимум следующие параметры:

- `spec.instance`;
- `spec.instance.cpu.cores`;
- `spec.instance.cpu.coreFraction`;
- `spec.instance.memory.size`;
- `spec.instance.persistentVolumeClaim.size`;
- `spec.postgresClassName`.

Пример привязки к PostgresClass:

```yaml
spec:
  postgresClassName: default
```

## Настройка ресурсов инстанса

Параметр `spec.instance` задаёт ресурсы PostgreSQL.

Пример:

```yaml
spec:
  instance:
    memory:
      size: 1Gi
    cpu:
      cores: 1
      coreFraction: 50
    persistentVolumeClaim:
      size: 1Gi
      storageClassName: default
```

Поддерживается параметр `spec.instance.persistentVolumeClaim.storageClassName`. Если он не указан, используется storage class по умолчанию в кластере Kubernetes.

## Настройка конфигурации PostgreSQL

В `spec.configuration` можно задать параметры PostgreSQL.

Поддерживаются следующие параметры:

- `maxConnections`;
- `sharedBuffers`;
- `walKeepSize`;
- `workMem`.

Пример:

```yaml
spec:
  configuration:
    maxConnections: 300
    sharedBuffers: 128Mi
```

Доступность переопределения этих параметров зависит от настроек связанного [PostgresClass](../../../admin/configuration/managed-services/postgres.html).

## Типы развёртывания

Параметр `spec.type` определяет тип сервиса PostgreSQL.

Поддерживаются следующие значения:

- `Cluster`;
- `Standalone`.

По умолчанию используется значение `Cluster`.

### Развёртывание в режиме Cluster

Для кластера используйте `spec.type: Cluster` и укажите параметры в секции `spec.cluster`.

Поддерживаются следующие значения `spec.cluster.topology`:

- `Ignored`;
- `Zonal`;
- `TransZonal`.

Поддерживаются следующие значения `spec.cluster.replication`:

- `Availability`;
- `Consistency`;
- `ConsistencyAndAvailability`.

### Режимы репликации

Каждому значению `spec.cluster.replication` соответствуют фиксированное число экземпляров и определённые настройки PostgreSQL.

- `Availability`: два экземпляра, primary и асинхронная реплика. Режим рассчитан на быстрое восстановление после сбоя. Возможна потеря последних транзакций, если они не успели реплицироваться до отказа primary.
- `Consistency`: два экземпляра, primary и синхронная реплика. Режим рассчитан на отсутствие потери подтверждённых транзакций, но запись останавливается, пока синхронная реплика недоступна.
- `ConsistencyAndAvailability`: три экземпляра, primary, синхронная реплика и асинхронная реплика. Режим сочетает сохранность данных и доступность и рекомендуется для production-нагрузок.

Единственная поддерживаемая версия PostgreSQL — `17.6`.

### Развёртывание в режиме Standalone

Ниже приведён пример ресурса Postgres для режима `Standalone`:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  labels:
    app.kubernetes.io/name: managed-psql-operator
  name: standalone
spec:
  users:
    - name: test-rw
      password: '123'
      role: rw
  databases:
    - name: "testdb"
  postgresClassName: default
  instance:
    memory:
      size: 4Gi
    cpu:
      cores: 2
      coreFraction: 50
    persistentVolumeClaim:
      size: 10Gi
  type: Standalone
```

Примените манифест:

```shell
d8 k apply -f managed-services_v1alpha1_postgres.yaml -n postgres
```

Проверьте состояние ресурса:

```shell
d8 k get postgres standalone -n postgres -o wide -w
```

Для подключения используйте Service `d8ms-pg-standalone-rw`:

```shell
psql -U test-rw -d testdb -h d8ms-pg-standalone-rw.postgres.svc -p 5432
```

## Подключение к базе данных

Для базового сценария используйте `psql` и Service, соответствующий имени ресурса Postgres и роли доступа.

Пример подключения к кластеру из базового сценария:

```shell
psql -U test-rw -d testdb -h d8ms-pg-test-rw.postgres.svc -p 5432
```

Для подключения к базе данных доступны следующие Services:

- `d8ms-pg-<name>-rw`: указывает на primary-экземпляр и позволяет выполнять операции чтения и записи;
- `d8ms-pg-<name>-ro`: указывает на реплики (в режиме `Cluster`) и позволяет выполнять операции только для чтения;
- `d8ms-pg-<name>-r`: указывает на primary-экземпляр или реплики (в режиме `Cluster`) и позволяет выполнять операции только для чтения со случайно выбранного экземпляра.

Если для пользователя задано поле `storeCredsToSecret`, строка подключения сохраняется в указанном Secret в поле `<database-name>-dsn`.

## Настройка пользователей

Параметр `spec.users` определяет пользователей PostgreSQL.

Для пользователя можно задать:

- `name`;
- `password`;
- `hashedPassword`;
- `role`;
- `storeCredsToSecret`.

Поддерживаются следующие роли:

- `ro`;
- `rw`;
- `monitoring`.

Пример:

```yaml
spec:
  users:
    - name: test-rw
      password: '123'
      role: rw
```

Если указать `password`, оператор автоматически преобразует его в `hashedPassword` и удалит `password` из `.spec`.

Если нужно сохранить пароль в открытом виде в Secret Kubernetes, используйте `storeCredsToSecret`.

Пример:

```yaml
spec:
  users:
    - name: test-rw
      password: '123'
      storeCredsToSecret: test-rw-creds
      role: rw
```

## Настройка логических баз данных

Параметр `spec.databases` определяет список логических баз данных PostgreSQL.

Пример:

```yaml
spec:
  databases:
    - name: "testdb"
```

## Создание снимка

Для резервного копирования используйте namespaced-ресурс PostgresSnapshot.

Перед созданием снимка убедитесь, что модуль [`snapshot-controller`](/modules/snapshot-controller/) включён, а выбранный `spec.instance.persistentVolumeClaim.storageClassName` поддерживает снимки.

Ниже приведён пример:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresSnapshot
metadata:
  name: my-first-snapshot
spec:
  postgresName: my-postgres
```

После создания снимка проверьте его статус:

```shell
d8 k get postgressnapshot -n postgres my-first-snapshot -o yaml | yq .status
```

В статусе PostgresSnapshot доступны, в частности, следующие поля:

- `phase`;
- `startedAt`;
- `completedAt`;
- `volumeSnapshotName`.

## Восстановление из снимка

Чтобы восстановить сервис из снимка, создайте новый ресурс Postgres и укажите `spec.dataSource.objectRef`.

Пример:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  name: my-restored-postgres
spec:
  dataSource:
    objectRef:
      kind: PostgresSnapshot
      name: my-first-snapshot
  users:
    - name: test-rw
      hashedPassword: >-
        SCRAM-SHA-256$4096:8LTjDsWOlQ7fnvr0DqRQx0TXMTh6LIyQJow2UnNlsJE=$ZjQi5diDTvn0g7is1ez9qPSGm6SoGezF0FVCZXssDKw=:IEzN8Dz5KcGd1r47thky5XFRhXlIMeoNLNfZtIlGv/8=
      role: rw
    - name: test-ro
      password: '123'
      storeCredsToSecret: test-ro-creds
      role: ro
  databases:
    - name: "test"
  postgresClassName: default
  instance:
    memory:
      size: 1Gi
    cpu:
      cores: 1
      coreFraction: 50
    persistentVolumeClaim:
      size: 1Gi
      storageClassName: thin-local-storage-class
  configuration:
    maxConnections: 300
  type: Cluster
  cluster:
    topology: Ignored
    replication: Availability
```

Примените манифест:

```shell
d8 k apply -f managed-services_v1alpha1_postgres.yaml -n postgres
```

Проверьте состояние восстановленного ресурса:

```shell
d8 k get postgres my-restored-postgres -n postgres -o wide -w
```

{% alert level="warning" %}
При восстановлении итоговая конфигурация ресурса Postgres снова проходит валидацию по связанному PostgresClass.
{% endalert %}

{% alert level="warning" %}
Списки `users` и `databases` имеют декларативный характер. Если не указать пользователя или базу данных в новом ресурсе Postgres, после восстановления они не будут присутствовать в итоговом сервисе, даже если были в снимке.
{% endalert %}

## Проверка состояния сервиса

Состояние сервиса PostgreSQL отражается в `status.conditions` ресурса Postgres.

Для базовой проверки используйте команду:

```shell
d8 k -n <users-ns> get postgres <cluster_name> -o wide -w
```

Если значения в `status.conditions` имеют статус `True`, это означает, что соответствующие этапы синхронизации завершены успешно.

## Обратите внимание

{% alert level="danger" %}
Удаление или переименование элементов в списках `users` и `databases` приводит к удалению соответствующих пользователей и логических баз данных в сервисе PostgreSQL.
{% endalert %}
