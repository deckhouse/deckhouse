---
title: "Managed PostgreSQL"
permalink: en/user/managed-services/postgres/
description: "Using the managed PostgreSQL service in Deckhouse Kubernetes Platform"
---

Managed PostgreSQL in Deckhouse Kubernetes Platform uses the namespaced Postgres resource to create a managed PostgreSQL service in a user namespace. The Postgres resource references a [PostgresClass](../../../admin/configuration/managed-services/postgres.html) configured by the cluster administrator and describes the required PostgreSQL configuration.

In the Postgres resource, you can specify the following:

- compute resources;
- storage size;
- deployment type;
- topology and replication settings;
- users;
- logical databases;
- data source for recovery.

The Postgres resource must reference an existing [PostgresClass](../../../admin/configuration/managed-services/postgres.html) through the `spec.postgresClassName` parameter. The `managed-postgres` controller uses this reference to check limits and apply default values.

Use this page to create a PostgreSQL service, select a deployment type, define logical databases and users, connect to the service, and configure snapshot-based recovery.

## What the Postgres resource creates

The Postgres resource describes a PostgreSQL service: a single instance or a highly available cluster. Inside this service, DKP can create logical databases and PostgreSQL users.

In a typical user scenario, you need to:

- create a PostgreSQL service with the required CPU, memory, and storage size;
- select a deployment option: `Standalone` or `Cluster`;
- create one or more logical databases in `spec.databases`;
- create users and access roles in `spec.users`;
- connect to a database through a Service created by the controller;
- if needed, create a PostgresSnapshot and restore a new service from the snapshot.

## Before you begin

Make sure that:

- [`managed-postgres`](/modules/managed-postgres/) is enabled;
- the administrator has provided the name of a suitable [PostgresClass](../../../admin/configuration/managed-services/postgres.html#prepare-postgresclass), allowed sizes, and available topology options;
- you have permission to create resources in the target namespace.

## Create a service with a database and a user

Create a Postgres resource in the application namespace. In a single manifest, specify the instance size, deployment type, logical database, and connection user.

The following example shows a basic PostgreSQL cluster with one logical database, `testdb`, and the `test-rw` user:

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

As a result, DKP creates a PostgreSQL service, the `testdb` logical database inside this service, and the `test-rw` user with the `rw` role.

## Create logical databases

The `spec.databases` parameter defines the list of logical databases inside the PostgreSQL service. This is not a separate service or a separate PostgreSQL instance: DKP creates the listed databases in the service described by the same Postgres resource.

Define logical databases together with users in a single manifest. DKP reconciles PostgreSQL to the specified state: it creates missing databases and users, synchronizes access roles, and deletes items that were removed from the `spec.databases` or `spec.users` lists.

Example:

```yaml
spec:
  databases:
    - name: "testdb"
```

To add a database to an existing service, add it to the `spec.databases` list and apply the updated Postgres resource manifest.

## Create users

The `spec.users` parameter defines PostgreSQL users for the service. Define users declaratively in the manifest instead of manually running `CREATE USER`, `GRANT`, and configuring access inside each PostgreSQL instance.

You can define the following fields for a user:

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

## Connect to the database

For a basic scenario, use `psql` and the Service that matches the Postgres resource name and endpoint type.

Example of connecting to the `test` Postgres resource in the `postgres` namespace from a pod in the same cluster:

```shell
psql -U test-rw -d testdb -h d8ms-pg-test-rw.postgres.svc -p 5432
```

The following Services are available for database connections:

- `d8ms-pg-<postgres-name>-rw`: points to the primary instance and allows read and write operations;
- `d8ms-pg-<postgres-name>-ro`: points to replicas (in `Cluster` mode) and allows read-only operations;
- `d8ms-pg-<postgres-name>-r`: points to the primary instance or replicas (in `Cluster` mode) and allows read-only operations against a randomly selected instance.

In the Service name, `<postgres-name>` matches the name of the Postgres resource, and the `rw`, `ro`, or `r` suffix indicates the endpoint type and is not related to the user name. In the `d8ms-pg-test-rw.postgres.svc` DNS name, the `test` part is the name of the Postgres resource, and the `postgres` part is the namespace where the resource is created.

If the user has the `storeCredsToSecret` field set, the connection string is stored in the specified Secret in the `<database-name>-dsn` field.

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

Use `spec.configuration` in the Postgres manifest to override PostgreSQL settings for a specific service.

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

Whether these parameters can be overridden depends on the settings of the related [PostgresClass](../../../admin/configuration/managed-services/postgres.html#prepare-postgresclass). If the administrator has not allowed overriding a parameter in PostgresClass, the Postgres resource fails validation.

## Select a deployment option

Managed PostgreSQL supports two deployment types:

- `Cluster`: a highly available deployment with multiple PostgreSQL instances. Use it for production workloads and services that require availability or durability of committed transactions;
- `Standalone`: a single PostgreSQL instance. Use it for development environments, test environments, and small workloads.

To select a deployment type, set `spec.type` to `Cluster` or `Standalone`. The default value is `Cluster`.

### Deploy in Cluster mode

For a cluster deployment, set `spec.type: Cluster` and configure topology and replication mode in the `spec.cluster` section.

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
