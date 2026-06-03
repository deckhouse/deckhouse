---
title: "managed-postgres"
permalink: en/user/managed-services/postgres.html
description: "Using the managed PostgreSQL service in Deckhouse Kubernetes Platform"
---

Use the namespaced resource Postgres to manage a PostgreSQL service. It describes the desired state of the managed PostgreSQL service, including the following:

- compute resources;
- storage size;
- deployment type;
- topology and replication settings;
- users;
- logical databases;
- data source for recovery.

The Postgres resource must reference an existing [PostgresClass](../../../admin/configuration/managed-services/postgres.html) through the `spec.postgresClassName` parameter. PostgresClass is configured by the cluster administrator.

## Before you begin

Make sure that:

- [`managed-postgres`](/modules/managed-postgres/) is enabled;
- a suitable [PostgresClass](../../../admin/configuration/managed-services/postgres.html) resource exists in the cluster;
- you have permission to create resources in the target namespace.

## Create a PostgreSQL service

The following example shows a basic Postgres resource:

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

Apply the manifest in the required namespace:

```shell
d8 k apply -f managed-services_v1alpha1_postgres.yaml -n postgres
```

Check the resource status:

```shell
d8 k get postgres test -n postgres -o wide -w
```

To verify that the service works correctly, make sure all values in `status.conditions` have the `True` status.

## Required parameters of the Postgres resource

The Postgres resource requires at least the following parameters:

- `spec.instance`;
- `spec.instance.cpu.cores`;
- `spec.instance.cpu.coreFraction`;
- `spec.instance.memory.size`;
- `spec.instance.persistentVolumeClaim.size`;
- `spec.postgresClassName`.

Example of binding to a PostgresClass:

```yaml
spec:
  postgresClassName: default
```

## Configure instance resources

The `spec.instance` parameter defines PostgreSQL resources.

Example:

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

The `spec.instance.persistentVolumeClaim.storageClassName` parameter is supported. If it is not specified, the default storage class in the Kubernetes cluster is used.

## Configure PostgreSQL settings

Use `spec.configuration` to define PostgreSQL settings.
The following parameters are supported:

- `maxConnections`;
- `sharedBuffers`;
- `walKeepSize`;
- `workMem`.

Example:

```yaml
spec:
  configuration:
    maxConnections: 300
    sharedBuffers: 128Mi
```

Whether these parameters can be overridden depends on the settings of the related [PostgresClass](../../../admin/configuration/managed-services/postgres.html).

## Deployment types

The `spec.type` parameter defines the PostgreSQL service type. The following values are supported:

- `Cluster`;
- `Standalone`.

The default value is `Cluster`.

### Deploy in Cluster mode

For a cluster deployment, use `spec.type: Cluster`
and specify parameters in the `spec.cluster` section.

The following `spec.cluster.topology` values are supported:

- `Ignored`;
- `Zonal`;
- `TransZonal`.

The following `spec.cluster.replication` values are supported:

- `Availability`;
- `Consistency`;
- `ConsistencyAndAvailability`.

### Replication modes

Each `spec.cluster.replication` value maps to a fixed number of instances and specific PostgreSQL settings.

- `Availability`: two instances, a primary and an asynchronous replica. This mode prioritizes fast recovery after a failure. It can lose the last transactions if they were not replicated before the primary failed.
- `Consistency`: two instances, a primary and a synchronous replica. This mode prioritizes zero loss of committed transactions, but writes stop while the synchronous replica is unavailable.
- `ConsistencyAndAvailability`: three instances, a primary, a synchronous replica, and an asynchronous replica. This mode balances durability and availability and is recommended for production workloads.

The only supported PostgreSQL version is `17.6`.

### Deploy in Standalone mode

The following example shows a Postgres resource for the `Standalone` mode:

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

Apply the manifest:

```shell
d8 k apply -f managed-services_v1alpha1_postgres.yaml -n postgres
```

Check the resource status:

```shell
d8 k get postgres standalone -n postgres -o wide -w
```

Use the `d8ms-pg-standalone-rw` Service to connect:

```shell
psql -U test-rw -d testdb -h d8ms-pg-standalone-rw.postgres.svc -p 5432
```

## Connect to the database

For a basic scenario, use `psql` and the Service that matches the Postgres resource name and access role.

Example of connecting to the cluster from the basic scenario:

```shell
psql -U test-rw -d testdb -h d8ms-pg-test-rw.postgres.svc -p 5432
```

The following Services are available for database connections:

- `d8ms-pg-<name>-rw`: points to the primary instance and allows read and write operations;
- `d8ms-pg-<name>-ro`: points to replicas (in `Cluster` mode) and allows read-only operations;
- `d8ms-pg-<name>-r`: points to the primary instance or replicas (in `Cluster` mode) and allows read-only operations against a randomly selected instance.

If the user has the `storeCredsToSecret` field set, the connection string is stored in the specified Secret in the `<database-name>-dsn` field.

## Configure users

The `spec.users` parameter defines PostgreSQL users. You can define the following fields for a user:

- `name`;
- `password`;
- `hashedPassword`;
- `role`;
- `storeCredsToSecret`.

The following roles are supported:

- `ro`;
- `rw`;
- `monitoring`.

Example:

```yaml
spec:
  users:
    - name: test-rw
      password: '123'
      role: rw
```

If you specify `password`, the operator automatically converts it to `hashedPassword` and removes `password` from `.spec`.

If you need to store the password in plain text in a Kubernetes Secret, use `storeCredsToSecret`.

Example:

```yaml
spec:
  users:
    - name: test-rw
      password: '123'
      storeCredsToSecret: test-rw-creds
      role: rw
```

## Configure logical databases

The `spec.databases` parameter defines the list of PostgreSQL logical databases.

Example:

```yaml
spec:
  databases:
    - name: "testdb"
```

## Create a snapshot

Use the namespaced resource PostgresSnapshot for backup.

Before you create a snapshot, make sure the [`snapshot-controller`](/modules/snapshot-controller/) module is enabled and the selected `spec.instance.persistentVolumeClaim.storageClassName` supports snapshots.

The following example shows a basic configuration:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresSnapshot
metadata:
  name: my-first-snapshot
spec:
  postgresName: my-postgres
```

After the snapshot is created, check its status:

```shell
d8 k get postgressnapshot -n postgres my-first-snapshot -o yaml | yq .status
```

The PostgresSnapshot status includes, among others, the following fields:

- `phase`;
- `startedAt`;
- `completedAt`;
- `volumeSnapshotName`.

## Restore from a snapshot

To restore a service from a snapshot, create a new Postgres resource and specify `spec.dataSource.objectRef`.

Example:

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

Apply the manifest:

```shell
d8 k apply -f managed-services_v1alpha1_postgres.yaml -n postgres
```

Check the status of the restored resource:

```shell
d8 k get postgres my-restored-postgres -n postgres -o wide -w
```

{% alert level="warning" %}
During recovery, the resulting Postgres resource configuration is validated again against the related PostgresClass.
{% endalert %}

{% alert level="warning" %}
The `users` and `databases` lists are declarative. If you do not specify a user or database in the new Postgres resource, it will not be present in the resulting service after recovery, even if it existed in the snapshot.
{% endalert %}

## Check service status

The PostgreSQL service status is reflected in `status.conditions` of the Postgres resource.

For a basic check, use the following command:

```shell
d8 k -n <users-ns> get postgres <cluster_name> -o wide -w
```

If the values in `status.conditions` have the `True` status, the corresponding synchronization stages have completed successfully.

## Important notes

{% alert level="danger" %}
Deleting or renaming items in the `users` and `databases` lists causes the corresponding users and logical databases to be deleted from the PostgreSQL service.
{% endalert %}
