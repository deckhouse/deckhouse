---
title: "Managed PostgreSQL"
permalink: en/user/managed-services/postgres/
description: "Creating, configuring, and operating PostgreSQL using the managed-postgres module."
relatedLinks:
  - title: "Frequently Asked Questions"
    url: "faq.html"
---

The `managed-postgres` module lets you create and configure PostgreSQL using the Postgres resource. The user sets the required configuration, and the module creates and maintains PostgreSQL instances according to the PostgresClass, which defines the available parameters and limits. The PostgresClass is created and configured by the cluster administrator.

This guide uses two examples:

- `app-postgres`: the [main example](#main-example-of-creating-postgres) for creating and operating PostgreSQL — resources, `Cluster` mode, replication, users, databases, PostgreSQL parameters, TLS, and observability;
- `snapshot-pg`: a separate example for creating and restoring snapshots, since it requires a StorageClass with CSI snapshot support.

{% alert level="info" %}
The examples use two worker nodes so that PostgreSQL instances in `Cluster` mode can be placed on different nodes. The cluster is located in a single zone, `default`.
{% endalert %}

## Check available resources

Before creating Postgres, check the available resources of the worker nodes. This lets you choose CPU and memory values for the example based on the actual load on the cluster.

First, list the nodes:

```shell
d8 k get nodes -o wide
```

The example uses two available worker nodes.

Example output:

```console
NAME       STATUS   ROLES    AGE   VERSION
worker-1   Ready    worker   25d   v1.34.9
worker-2   Ready    worker   43m   v1.34.9
```

Check the resources allocated on the first worker node:

```shell
d8 k describe node worker-1 | grep -A 5 "Allocated resources"
```

Example output:

```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests          Limits
  --------           --------          ------
  cpu                1104m (28%)       500m (12%)
  memory             4096854330 (53%)  390Mi (5%)
```

Check the second worker node:

```shell
d8 k describe node worker-2 | grep -A 5 "Allocated resources"
```

Example output:

```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests      Limits
  --------           --------      ------
  cpu                472m (12%)    500m (12%)
  memory             1004Mi (13%)  256Mi (3%)
```

## Check storage

Before creating Postgres, check the available StorageClass options and choose the storage class where the PostgreSQL data will be placed:

```shell
d8 k get storageclass
```

Example output from the test bench:

<!-- markdownlint-disable MD031 -->
```console
NAME                   PROVISIONER            RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION
local                  csi.dvp.deckhouse.io   Delete          WaitForFirstConsumer   true
replicated (default)   csi.dvp.deckhouse.io   Delete          WaitForFirstConsumer   true
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

The [`spec.instance.persistentVolumeClaim.storageClassName`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-persistentvolumeclaim-storageclassname) parameter is set only when Postgres is created and can't be changed later.

## Main example of creating Postgres

Create a namespace:

```shell
d8 k create namespace postgres
```

To create a Postgres resource, complete the following steps:

- [Select PostgresClass](#select-postgresclass);
- [Configure Postgres resources](#configure-postgres-resources);
- [Select deployment mode](#select-deployment-mode);
- [Configure topology and replication mode](#configure-topology-and-replication-mode);
- [Create logical database and user](#create-logical-database-and-user);
- [Configure PostgreSQL parameters](#configure-postgresql-parameters);
- [Configure TLS](#configure-tls);
- [Configure observability](#configure-observability).

The following example shows an `app-postgres` Postgres resource manifest. You can apply it as is and then customize it using the steps below.

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

Save the manifest to `postgres.yaml` and apply it:

```shell
d8 k apply -f postgres.yaml
```

Check the status of the created Postgres:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

Once the rollout is complete, the main conditions should change to `True`. For details about each condition, see [Check status](#check-status).

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME           AVAILABLE   CONFIGURATIONVALID   LASTVALIDCONFIGURATIONAPPLIED   SCALEDTOLASTVALIDCONFIGURATION   DATABASESSYNCED   USERSSYNCED
app-postgres   True        True                 True                            True                             True              True
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

To change a parameter of `app-postgres`, edit the corresponding fragment of `postgres.yaml` and reapply the file.

## Select PostgresClass

The [`spec.postgresClassName`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-postgresclassname) parameter defines the PostgresClass that sets the available parameters and limits for Postgres. To list the PostgresClass resources available in the cluster, run:

```shell
d8 k get postgresclass
```

Example output:

```console
NAME      AGE
default   13d
```

This example uses the `default` PostgresClass with standard limits. If a different PostgresClass is selected, you can view its limits in the configuration:

```shell
d8 k get postgresclass <CLASS_NAME> -o yaml
```

Where `<CLASS_NAME>` is the name of the selected PostgresClass.

{% alert level="warning" %}
When choosing a PostgresClass, consider the allowed CPU, memory, and `coreFraction` values, available topologies, and PostgreSQL parameters that can be overridden. If the Postgres configuration doesn't meet the limits of the selected class, the API rejects it on apply.
{% endalert %}

The PostgresClass settings and limits are described in [Limit CPU and memory resources](/admin/configuration/managed-services/postgres/#limit-cpu-and-memory-resources), [Manage fault tolerance across availability zones](/admin/configuration/managed-services/postgres/#manage-fault-tolerance-across-availability-zones), and [Automatically validate PostgreSQL settings](/admin/configuration/managed-services/postgres/#automatically-validate-postgresql-settings).

### Placement restrictions

A PostgresClass can also define PostgreSQL instance placement rules using `nodeSelector`, `nodeAffinity`, and `tolerations`. These rules apply automatically once the class is selected and aren't specified in the Postgres resource.

## Configure Postgres resources

For each PostgreSQL instance, you can set the number of CPUs, the guaranteed CPU share, and the amount of memory.

The [`spec.instance`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance) parameter defines the resources of each PostgreSQL instance.

In the example, this fragment is responsible for resources and storage:

```yaml
spec:
  instance:
    cpu:
      cores: 1
      coreFraction: 50
    memory:
      size: 1Gi
```

In the example, the instance is allocated one CPU core and `1Gi` of memory. The [`coreFraction`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-instance-cpu-corefraction) parameter defines the ratio of the CPU request to the CPU limit. For `cores: 1` and `coreFraction: 50`, the module produced:

Example output:

```console
limits.cpu:   1
requests.cpu: 500m
```

For more information, see [Limit CPU and memory resources](/admin/configuration/managed-services/postgres/#limit-cpu-and-memory-resources).

### Change resources of an existing Postgres

You can change Postgres resources by reapplying the manifest, provided the new values are allowed by the selected PostgresClass. First, check the current resource values:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o custom-columns='NAME:.metadata.name,CPU_REQUEST:.spec.containers[0].resources.requests.cpu,MEMORY_REQUEST:.spec.containers[0].resources.requests.memory'
```

Example output:

```console
NAME                       CPU_REQUEST   MEMORY_REQUEST
d8ms-pg-app-postgres-1     500m          1Gi
d8ms-pg-app-postgres-2     500m          1Gi
```

You can apply changes to both memory and CPU at once, but for clarity, first increase memory from `1Gi` to `2Gi`:

```yaml
spec:
  instance:
    memory:
      size: 2Gi
```

Apply the updated manifest:

```shell
d8 k apply -f postgres.yaml
```

After the update completes, the CPU request stays at `500m`, and the memory request of the instances changes to `2Gi`.

Example output:

```console
NAME                       CPU_REQUEST   MEMORY_REQUEST
d8ms-pg-app-postgres-1     500m          2Gi
d8ms-pg-app-postgres-2     500m          2Gi
```

Next, change `coreFraction` from `50` to `100`, leaving `cores: 1`:

```yaml
spec:
  instance:
    cpu:
      cores: 1
      coreFraction: 100
```

Reapply the manifest:

```shell
d8 k apply -f postgres.yaml
```

After the update completes, the CPU request of the instances changes from `500m` to `1`, and the memory request stays at `2Gi`.

Example output:

```console
NAME                       CPU_REQUEST   MEMORY_REQUEST
d8ms-pg-app-postgres-1     1             2Gi
d8ms-pg-app-postgres-2     1             2Gi
```

For a running Postgres, you can change memory and `coreFraction` within the limits allowed by the selected PostgresClass.

### Check CPU and memory limits via PostgresClass

CPU and memory values must comply with the limits of the selected PostgresClass. If the specified resources or their combination don't match the allowed values, the API rejects the configuration.

The `default` PostgresClass isn't well suited for a clear demonstration of these limits. So this example uses a separate `check` PostgresClass, which allows memory from `512Mi` to `2Gi` in `512Mi` steps for `1–2` CPUs.

With `cores: 1` and `coreFraction: 50`, a memory value of `700Mi` doesn't match the configured step, so the manifest is rejected:

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

Apply the manifest:

```shell
d8 k apply -f postgres.yaml
```

The API rejects the resource. Example output:

```console
spec.instance.memory.size: Invalid value: 734003200: memory setting does not fit Step 536870912 of the selected PostgresClass
```

## Select deployment mode

The set of PostgreSQL instances depends on the selected deployment mode: `Cluster` creates a primary instance and replicas, whose composition depends on the selected replication mode; `Standalone` creates a single PostgreSQL instance without replicas.

### Cluster mode

To work with a primary instance and replicas, use `Cluster` mode, set by the [`spec.type`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-type) parameter.

```yaml
spec:
  type: Cluster
  cluster:
    topology: Ignored
    replication: Consistency
```

The replication mode and its parameters are configured separately. The available modes and usage examples are described in [Configure replication](#configure-replication).

### Standalone mode

`Standalone` mode runs PostgreSQL as a single instance without replication. Unlike `Cluster` mode, it doesn't use topology or replication parameters.

To use this mode, specify:

```yaml
spec:
  type: Standalone
```

After Postgres is created, a single PostgreSQL instance starts. Check the created PostgreSQL instances:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Example output:

```console
NAME                     STATUS    NODE
d8ms-pg-app-postgres-1   Running   worker-1
```

Check the Services created for connecting to PostgreSQL:

```shell
d8 k get svc -n postgres | grep app-postgres
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
d8ms-pg-app-postgres-r    ClusterIP   10.223.234.52    <none>   5432/TCP
d8ms-pg-app-postgres-ro   ClusterIP   10.223.70.248    <none>   5432/TCP
d8ms-pg-app-postgres-rw   ClusterIP   10.223.120.250   <none>   5432/TCP
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Check which instances the Services point to, via endpoints:

```shell
d8 k get endpoints -n postgres | grep app-postgres
```

Example output:

```console
d8ms-pg-app-postgres-r    10.112.2.31:5432   42h
d8ms-pg-app-postgres-ro   <none>             42h
d8ms-pg-app-postgres-rw   10.112.2.31:5432   42h
```

The `-r` and `-rw` Services route connections to the only instance. The `-ro` Service is also created but has no endpoint, since `Standalone` mode has no replicas.

## Configure topology and replication mode

In `Cluster` mode, topology determines how PostgreSQL instances are placed across nodes and availability zones. It lets you control where instances are placed to meet PostgreSQL fault tolerance requirements.

### Configure topology

The [`spec.cluster.topology`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-topology) parameter determines how PostgreSQL instances are placed across nodes and availability zones.

The following values are supported:

- `Ignored`: placement follows the standard Kubernetes scheduling rules, spreading instances across different nodes;
- `Zonal`: instances are placed within one of the allowed zones;
- `TransZonal`: instances are placed across different availability zones.

The available topology values and zones are determined by the selected PostgresClass. For `Zonal` and `TransZonal`, the cluster infrastructure must provide the corresponding availability zones. For more information, see [Manage fault tolerance across availability zones](/admin/configuration/managed-services/postgres/#manage-fault-tolerance-across-availability-zones).

#### Placement without zone selection

With `topology: Ignored`, instance placement is managed by the Kubernetes scheduler. This mode spreads instances across different nodes without any additional configuration from the user. The [main example](#main-example-of-creating-postgres) uses this mode:

```yaml
spec:
  cluster:
    topology: Ignored
```

Check instance placement:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

The instances should be on different nodes.

Example output:

```console
NAME                       STATUS    NODE
d8ms-pg-app-postgres-1     Running   worker-1
d8ms-pg-app-postgres-2     Running   worker-2
```

#### Placement within a single zone

With `topology: Zonal`, one of the zones allowed by the selected PostgresClass is chosen for placing Postgres. All cluster instances are placed within this zone.

```yaml
spec:
  cluster:
    topology: Zonal
```

To use `Zonal`, nodes must have the `topology.kubernetes.io/zone` label set to the corresponding zone value. The zone must be allowed by the selected PostgresClass.

For example, if two available nodes belong to the `default` zone:

Example output:

```console
NAME       ZONE
worker-1   default
worker-2   default
```

The instances can be placed as follows.

Example output:

```console
NAME                       STATUS    NODE
d8ms-pg-app-postgres-1     Running   worker-1
d8ms-pg-app-postgres-2     Running   worker-2
```

In this example, both instances are placed in the `default` zone.

### Configure replication

In `Cluster` mode, replication transfers data from the primary PostgreSQL instance to the replicas. The replication mode is set in [`spec.cluster.replication`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-replication).

The following modes are supported:

- `Availability`: a primary instance and one asynchronous replica;
- `Consistency`: a primary instance and one synchronous replica;
- `ConsistencyAndAvailability`: a primary instance, one synchronous replica, and one asynchronous replica.

#### Check replication mode

Replication status is checked through the `pg_stat_replication` view on the primary instance. Use this procedure for any `Cluster` mode, including after changing `spec.cluster.replication`.

First, identify the current primary instance:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"
```

Then run the query:

```shell
d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -c \
  "SELECT application_name, state, sync_state FROM pg_stat_replication;"
```

The expected `sync_state` values depend on the mode:

| Mode | Number of replicas | Expected `sync_state` |
| :-- | :-- | :-- |
| `Availability` | 1 | `async` |
| `Consistency` | 1 | `quorum` |
| `ConsistencyAndAvailability` | 2 | `quorum` and `async` |

A `state: streaming` value means the replica is receiving changes from the primary instance. Instance roles can change while the mode is being switched, so always identify the primary instance again via `status.targetPrimary` rather than by Pod number.

#### Availability mode

`Availability` mode creates a primary PostgreSQL instance and one asynchronous replica.

To use this mode, specify:

```yaml
spec:
  type: Cluster
  cluster:
    topology: Zonal
    replication: Availability
```

After Postgres is created, two PostgreSQL instances start:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Example output:

```console
NAME                     READY   STATUS
d8ms-pg-app-postgres-1   1/1     Running
d8ms-pg-app-postgres-2   1/1     Running
```

Check the replication mode as described in [Check replication mode](#check-replication-mode). For `Availability`, expect one replica with `sync_state = async`:

```console
     application_name      |   state   | sync_state
---------------------------+-----------+------------
 d8ms-pg-app-postgres-2    | streaming | async
(1 row)
```

You can check how Service traffic is distributed between the primary instance and the replica through EndpointSlice:

```shell
d8 k get endpointslice -n postgres | grep app-postgres
```

Example output:

```console
d8ms-pg-app-postgres-r-v8kcv    IPv4   5432   10.112.2.249,10.112.2.155
d8ms-pg-app-postgres-ro-696bp   IPv4   5432   10.112.2.155
d8ms-pg-app-postgres-rw-8nx8s   IPv4   5432   10.112.2.249
```

The `-rw` Service routes connections to the primary instance, `-ro` to the replica, and `-r` to both instances.

#### Consistency mode

`Consistency` mode, used in the [main example](#main-example-of-creating-postgres), creates a primary PostgreSQL instance and one synchronous replica:

```yaml
spec:
  type: Cluster
  cluster:
    topology: Ignored
    replication: Consistency
```

After Postgres is created, two PostgreSQL instances start:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Example output:

```console
NAME                       STATUS    NODE
d8ms-pg-app-postgres-1     Running   worker-1
d8ms-pg-app-postgres-2     Running   worker-2
```

Check the replication mode as described in [Check replication mode](#check-replication-mode). For `Consistency`, expect one replica with `sync_state = quorum`:

```console
      application_name      |   state   | sync_state
----------------------------+-----------+------------
 d8ms-pg-app-postgres-2     | streaming | quorum
(1 row)
```

You can additionally verify synchronous replication by checking the actual data transfer. Identify the primary instance the same way as in [Check replication mode](#check-replication-mode):

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"
```

Create a check table on the primary instance and insert a row:

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

Identify the replica:

```shell
REPLICA="$(d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | \
  grep -v "^${PRIMARY}$" | head -n1)"
```

Check that the row exists directly on the replica:

```shell
d8 k exec -n postgres "$REPLICA" -- \
  psql -U postgres -d postgres -c \
  "SELECT pg_is_in_recovery(), * FROM consistency_check;"
```

Example output:

```console
 pg_is_in_recovery | id |   value
-------------------+----+------------
 t                 |  1 | replicated
(1 row)
```

A `pg_is_in_recovery() = t` value shows that the query ran on the replica. The presence of the `replicated` row confirms that data was transferred from the primary instance to the synchronous replica.

#### ConsistencyAndAvailability mode

`ConsistencyAndAvailability` mode creates a primary PostgreSQL instance, one synchronous replica, and one asynchronous replica.

To use this mode, specify:

```yaml
spec:
  type: Cluster
  cluster:
    topology: Zonal
    replication: ConsistencyAndAvailability
```

After Postgres is created, three PostgreSQL instances start:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

Example output:

```console
NAME                     READY   STATUS
d8ms-pg-app-postgres-1   1/1     Running
d8ms-pg-app-postgres-2   1/1     Running
d8ms-pg-app-postgres-3   1/1     Running
```

Check the replication mode as described in [Check replication mode](#check-replication-mode). For `ConsistencyAndAvailability`, expect two replicas — with `sync_state = quorum` and `sync_state = async`:

```console
     application_name      |   state   | sync_state
---------------------------+-----------+------------
 d8ms-pg-app-postgres-2    | streaming | quorum
 d8ms-pg-app-postgres-3    | streaming | async
(2 rows)
```

### Change replication mode of an existing cluster

You can change the replication mode of an existing Postgres in `Cluster` mode. To do this, change `spec.cluster.replication` in the `app-postgres` manifest and reapply it.

For example, to switch from `Availability` to `Consistency`, specify:

```yaml
spec:
  cluster:
    replication: Consistency
```

Apply the changes:

```shell
d8 k apply -f postgres.yaml
```

During the update, `ScaledToLastValidConfiguration` may temporarily switch to `False`. After the update completes, the resource conditions should return to `True`.

Check the new mode as described in [Check replication mode](#check-replication-mode). After switching to `Consistency`, the replica should work in synchronous mode:

```console
d8ms-pg-app-postgres-1 | streaming | quorum
```

When switching back to `Availability`, the same check should show asynchronous replication:

```console
d8ms-pg-app-postgres-2 | streaming | async
```

When switching to `ConsistencyAndAvailability`, the number of instances increases from two to three. Check the running instances:

```shell
d8 k get pods -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o wide
```

After the update completes, the `pg_stat_replication` check should show a synchronous and an asynchronous replica:

```console
     application_name      |   state   | sync_state
---------------------------+-----------+------------
 d8ms-pg-app-postgres-3    | streaming | async
 d8ms-pg-app-postgres-2    | streaming | quorum
(2 rows)
```

When switching back from `ConsistencyAndAvailability` to `Consistency`, the number of instances decreases from three to two, and the remaining replica works in `streaming | quorum` mode.

## Create logical database and user

The [main example](#main-example-of-creating-postgres) creates the `app-rw` user and the `app` logical database:

```yaml
spec:
  users:
    - name: app-rw
      role: rw
      storeCredsToSecret: app-postgres-rw

  databases:
    - name: app
```

After applying the manifest, wait for the users and databases to synchronize. The `USERSSYNCED` and `DATABASESSYNCED` conditions should be `True`:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

### PostgreSQL user

User credentials are stored in the Secret specified in [`storeCredsToSecret`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-users-storecredstosecret).

Check the created Secret:

```shell
d8 k get secret app-postgres-rw -n postgres
```

Example output:

```console
NAME              TYPE                       DATA
app-postgres-rw   kubernetes.io/basic-auth   4
```

The Secret contains the parameters required for connecting:

```text
app-dsn
host
password
username
```

Get the connection parameters as follows:

```shell
echo "host: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.host}' | base64 --decode)"
echo "username: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.username}' | base64 --decode)"
echo "password: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.password}' | base64 --decode)"
echo "app-dsn: $(d8 k get secret app-postgres-rw -n postgres -o jsonpath='{.data.app-dsn}' | base64 --decode)"
```

Example output:

```console
host: d8ms-pg-app-postgres-rw
username: app-rw
password: <PASSWORD>
app-dsn: postgresql://app-rw:<PASSWORD>@d8ms-pg-app-postgres-rw:5432/app
```

Where `<PASSWORD>` is the user's password from the Secret.

You can use the values from the Secret to configure your application or PostgreSQL client connection.

{% alert level="info" %}
For application connections, use the Secret named in `storeCredsToSecret`. Don't use the internal Secrets named `d8ms-pg-...` for this purpose.
{% endalert %}

### Declarative user management

The list of users in [`spec.users`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-users) describes the required PostgreSQL state. When the list changes, the module synchronizes user roles and their related Secrets.

For example, remove the `app-rw` user from the manifest:

```yaml
spec:
  users: []
```

Apply the updated manifest:

```shell
d8 k apply -f postgres.yaml
```

After synchronization completes, the `USERSSYNCED` condition should return to `True`:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

Check that the role is gone directly in PostgreSQL:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"

d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -Atc \
  "SELECT rolname FROM pg_roles WHERE rolname = 'app-rw';"
```

The command shouldn't return a role name.

When running `d8 k exec`, a message about the selected container may appear:

```console
Defaulted container "postgres" out of: postgres, bootstrap-controller (init)
```

The `app` logical database, which remains in [`spec.databases`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-databases), isn't deleted when the user is removed. Check that it still exists:

```shell
d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -Atc \
  "SELECT datname FROM pg_database WHERE datname = 'app';"
```

Expected output:

```console
app
```

{% alert level="warning" %}
Removing a user from `spec.users` deletes the corresponding PostgreSQL role. Before removing a user, make sure it's no longer used by any applications.
{% endalert %}

To recreate the user, add it back to `spec.users`:

```yaml
spec:
  users:
    - name: app-rw
      role: rw
      storeCredsToSecret: app-postgres-rw
```

After you reapply the manifest, the module recreates the PostgreSQL role and the Secret.

### Logical database

The logical databases that the module must create and maintain are defined in `spec.databases`:

```yaml
spec:
  databases:
    - name: app
```

After the database is created, the `DATABASESSYNCED` condition should be `True`.

{% alert level="warning" %}
Removing a database from `spec.databases` deletes the corresponding logical PostgreSQL database along with its data.
{% endalert %}

## Connect to PostgreSQL

After Postgres is created, the module creates the `-r`, `-ro`, and `-rw` Services, used to connect to the PostgreSQL instances depending on their role:

- `-rw`: the primary instance;
- `-ro`: the replicas;
- `-r`: all available instances.

For clarity, the `app-postgres` example creates the following Services:

```text
NAME                      TYPE        PORT(S)
d8ms-pg-app-postgres-r    ClusterIP   5432/TCP
d8ms-pg-app-postgres-ro   ClusterIP   5432/TCP
d8ms-pg-app-postgres-rw   ClusterIP   5432/TCP
```

By default, these Services have type `ClusterIP` and are available inside the cluster. The user's connection credentials and parameters are stored in the Secret specified in `storeCredsToSecret`.

You can connect to PostgreSQL both from applications inside the cluster and from an external network. For an external connection, the corresponding Service needs to be published separately.

### Connect from within the cluster

For a connection from within the cluster, use the corresponding Service and the credentials from the user's Secret. In the [main example](#main-example-of-creating-postgres), an application with write access connects to the `d8ms-pg-app-postgres-rw` Service as the `app-rw` user to the `app` database.

To test the connection, you don't need to install `psql` on a control plane node. You can use a temporary client Pod instead:

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

Example output:

```console
 current_database | session_user | current_user
------------------+--------------+--------------
 app              | app-rw       | rw
(1 row)
```

`session_user` shows the user the connection was made as (`app-rw`), and `current_user` shows the current access role (`rw`).

### External connection to PostgreSQL

To work with PostgreSQL from an external network, you can use graphical clients and other applications that support connecting to PostgreSQL. For this, the Service through which PostgreSQL is published must be reachable from outside the cluster, and the client needs to specify the server address, port, database, and user credentials.

{% alert level="info" %}
This section covers external connections using DBeaver as an example. You can use other PostgreSQL clients and applications the same way.
{% endalert %}

In this example, the connection is made to the previously created `app` database as the `app-rw` user in the `app-postgres` Postgres.

#### Publish PostgreSQL for external access

The publishing method depends on the cluster's network infrastructure. In this example, an external load balancer accepts connections on `<EXTERNAL_IP>:5432` and forwards them to `NodePort` `30001` on a cluster node. A separate Service routes this traffic to the primary PostgreSQL instance.

Don't modify the `d8ms-pg-app-postgres-rw` Service created by the module. Create a separate Service for external access:

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

Apply the manifest:

```shell
d8 k apply -f app-postgres-external.yaml
```

Check the created Service:

```shell
d8 k get svc app-postgres-external -n postgres -o wide
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                    TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE   SELECTOR
app-postgres-external   NodePort   10.223.111.45   <none>        5432:30001/TCP   4s    cnpg.internal.managed.deckhouse.io/cluster=d8ms-pg-app-postgres,cnpg.internal.managed.deckhouse.io/instanceRole=primary
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

On the external load balancer, configure it to accept TCP connections on port `5432` and forward them to `NodePort` `30001` on the cluster node. In this example, the following chain is set up:

```text
<EXTERNAL_IP>:5432
        |
external load balancer
        |
<NODE_IP>:30001
        |
NodePort
        |
primary PostgreSQL :5432
```

{% alert level="warning" %}
When publishing PostgreSQL to an external network, make sure access to the database port is restricted to trusted sources only. Use a firewall, allowed IP lists, a VPN, or other network infrastructure controls for this. It isn't recommended to leave PostgreSQL accessible from the internet without restrictions.
{% endalert %}

Before testing the external connection, you can confirm that the created `NodePort` routes traffic to the primary PostgreSQL instance. To do this, get the user's password:

```shell
PGPASSWORD="$(d8 k get secret app-postgres-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode)"
```

Start a temporary client Pod and connect via the node IP and `NodePort`:

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

Example of a successful result:

```console
 current_database | pg_is_in_recovery | inet_server_addr
------------------+-------------------+------------------
 app              | f                 | <POD_IP>
(1 row)
```

A `pg_is_in_recovery = f` value confirms that the connection is directed to the primary PostgreSQL instance.

#### Connect by IP address

You can connect directly by IP address, for example, to verify external access to PostgreSQL. For a permanent connection, it's recommended to use a DNS name and TLS with server certificate verification, as described below.

Get the user's password:

```shell
d8 k get secret app-postgres-rw -n postgres \
  -o jsonpath='{.data.password}' | base64 --decode; echo
```

In DBeaver, create a PostgreSQL connection and specify:

```text
Host:     <EXTERNAL_IP>
Port:     5432
Database: app
Username: app-rw
Password: <PASSWORD>
```

Where `<PASSWORD>` is the password from the `app-postgres-rw` Secret.

After connecting, open the SQL Editor and run:

```sql
SELECT
    current_database(),
    session_user,
    inet_server_addr(),
    inet_server_port(),
    pg_is_in_recovery();
```

On the tested bench, the query returned:

<!-- markdownlint-disable MD031 -->
```console
 current_database | session_user | inet_server_addr | inet_server_port | pg_is_in_recovery
------------------+--------------+------------------+------------------+-------------------
 app              | app-rw       | <POD_IP>         |             5432 | f
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

A `pg_is_in_recovery = f` value confirms the connection to the primary PostgreSQL instance.

#### Connect with TLS verification

For a permanent external connection, it's recommended to use TLS with server certificate verification.

In the example, `app-postgres` uses `K8s` mode, so PostgreSQL TLS certificates are issued automatically. The server certificate is signed by `cluster-selfsigned-ca`.

Save the automatically created server certificate to a file to determine the DNS name from the SAN:

```shell
d8 k get secret d8ms-pg-app-postgres-server-cert \
  -n postgres \
  -o jsonpath='{.data.tls\.crt}' | \
  base64 --decode > /tmp/app-postgres-server.crt
```

View the certificate details and its Subject Alternative Name (SAN):

```shell
openssl x509 \
  -in /tmp/app-postgres-server.crt \
  -noout \
  -subject -issuer -dates -ext subjectAltName
```

For `app-postgres`, the certificate contains the DNS name of the `-rw` Service:

```console
d8ms-pg-app-postgres-postgres-rw.<EXTERNAL_IP>.sslip.io
```

With `verify-full` mode, the client checks that the server name matches the certificate, so use the DNS name from the SAN for the connection.

Get the CA certificate:

```shell
d8 k get secret selfsigned-ca-key-pair \
  -n d8-cert-manager \
  -o jsonpath='{.data.tls\.crt}' | \
  base64 --decode > /tmp/app-postgres-ca.crt
```

Verify the trust chain:

```shell
openssl verify \
  -CAfile /tmp/app-postgres-ca.crt \
  /tmp/app-postgres-server.crt
```

Example of a successful result:

```console
/tmp/app-postgres-server.crt: OK
```

Transfer the CA certificate to the machine you're connecting from. For example, if the cluster node is reachable via SSH, copy the certificate using `scp`:

```shell
scp user@<NODE_IP>:/tmp/app-postgres-ca.crt ~/app-postgres-ca.crt
```

In DBeaver, specify the connection parameters:

```text
Host:     d8ms-pg-app-postgres-postgres-rw.<EXTERNAL_IP>.sslip.io
Port:     5432
Database: app
Username: app-rw
Password: <PASSWORD>
```

Where `<PASSWORD>` is the password from the `app-postgres-rw` Secret.

In the SSL settings, specify the CA certificate and `verify-full` mode:

```text
CA Certificate: <CA_CERT_PATH>
SSL mode:       verify-full
```

Where `<CA_CERT_PATH>` is the path to the `app-postgres-ca.crt` file.

After connecting, run:

```sql
SELECT
    current_database(),
    session_user,
    inet_server_addr(),
    inet_server_port(),
    pg_is_in_recovery();
```

A successful query execution and a `pg_is_in_recovery = f` value confirm the connection to the primary PostgreSQL instance.

If you use `verify-full` and specify the `<EXTERNAL_IP>` address instead of the DNS name from the SAN, server name verification fails:

```console
The hostname <EXTERNAL_IP> could not be verified by hostnameverifier PgjdbcHostnameVerifier.
```

When using `verify-full`, connect using a DNS name listed in the server certificate's SAN.

## Configure PostgreSQL parameters

PostgreSQL parameters can be changed via [`spec.configuration`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-configuration), if the selected PostgresClass allows overriding them.

Whether a parameter can be changed is determined by the PostgresClass settings:

- the parameter must be allowed to be overridden;
- the parameter value must comply with the configured validation rules.

If a parameter cannot be overridden or its value is outside the allowed limits, the API rejects the Postgres resource when it is applied.

### Change an allowed parameter

In the [main example](#main-example-of-creating-postgres), `app-postgres` uses the `default` PostgresClass, which allows changing the `maxConnections` parameter.

Change the value:

```yaml
spec:
  configuration:
    maxConnections: 100
```

Apply the changes:

```shell
d8 k apply -f postgres.yaml
```

After the update completes, check the applied value directly in PostgreSQL:

```shell
PRIMARY="$(d8 k get clusters.cnpg.internal.managed.deckhouse.io d8ms-pg-app-postgres \
  -n postgres \
  -o jsonpath='{.status.targetPrimary}')"

d8 k exec -n postgres "$PRIMARY" -- \
  psql -U postgres -d postgres -c \
  "SHOW max_connections;"
```

Example output:

```console
 max_connections
-----------------
 100
```

The parameter was changed because it's allowed to be overridden by the selected PostgresClass.

### Restrictions on parameter overrides

A PostgresClass can restrict the list of PostgreSQL parameters a user can change via `spec.configuration`.

For example, if the PostgresClass only allows overriding:

```yaml
overridableConfiguration:
  - maxConnections
  - sharedBuffers
  - walKeepSize
```

an attempt to change a parameter not on this list is rejected.

Apply, for example:

```yaml
spec:
  configuration:
    workMem: 16Mi
```

```shell
d8 k apply -f postgres.yaml
```

The API returns an error. Example output:

```console
Configuration field workmem restricted to override by administrator in selected postgresClass
```

In this case, the Postgres configuration doesn't change, since the parameter is restricted by the selected PostgresClass.

### Validate parameter values

In addition to specifying which parameters can be overridden, a PostgresClass can define validation rules for their values.

For example, if `maxConnections` has the following constraint:

```text
configuration.maxConnections >= 100
```

the following change is rejected:

```yaml
spec:
  configuration:
    maxConnections: 50
```

Apply the manifest:

```shell
d8 k apply -f postgres.yaml
```

The API returns an error. Example output:

```console
Rule: configuration.maxConnections >= 100
```

The existing Postgres continues running with the last successfully applied configuration.

## Configure TLS

The [`spec.tls`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-tls) parameter defines how PostgreSQL TLS certificates are managed. The `CertManager`, `CustomCertificate`, and `K8s` modes are supported.

To use certificates issued by cert-manager, specify `CertManager` mode:

```yaml
spec:
  tls:
    mode: CertManager
    certManager:
      clusterIssuerName: postgres-ca
```

The corresponding Issuer or ClusterIssuer must be prepared in advance. The administrative dependencies are described in [Dependencies for specific features](/admin/configuration/managed-services/postgres/#dependencies-for-specific-features).

To use existing certificates from a Secret, select `CustomCertificate` mode:

```yaml
spec:
  tls:
    mode: CustomCertificate
    customCertificate:
      serverCASecret: postgres-ca
      serverTLSSecret: postgres-tls
```

### K8s mode

In `K8s` mode, PostgreSQL certificates are issued automatically:

```yaml
spec:
  tls:
    mode: K8s
```

Once the resource reaches the ready state, the module creates a Secret with the CA, server, and replication certificates.

Check TLS usage on the PostgreSQL side through the `pg_stat_ssl` view. Identify the primary instance the same way as in [Check replication mode](#check-replication-mode), and run the query:

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

For a TLS connection, the `ssl` field is `t`, and `version` and `cipher` show the TLS version and cipher used.

Configuring a client connection with server certificate verification is described in [Connect with TLS verification](#connect-with-tls-verification).

## Configure observability

For Postgres, you can enable monitoring with alerts, fully disable monitoring, or keep monitoring without alerts. The observability mode is set by the [`spec.observability`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-observability) parameter.

The [main example](#main-example-of-creating-postgres) enables monitoring and alerts:

```yaml
spec:
  observability: Enabled
```

To fully disable monitoring, use:

```yaml
spec:
  observability: Disabled
```

To keep monitoring but disable alerts, use:

```yaml
spec:
  observability: EnabledWithoutAlerts
```

Check the applied mode by the Pod labels:

```shell
d8 k get pod -n postgres \
  -l managed-services.deckhouse.io/managed-service-name=app-postgres \
  -o json | \
  jq '.items[].metadata.labels | with_entries(select(.key | test("observability|prometheus")))'
```

The `observability.deckhouse.io/servicemonitoring` label value depends on the selected mode:

```text
Enabled                → enabled
Disabled               → disabled
EnabledWithoutAlerts   → no-alerts
```

With monitoring enabled, the output also contains the label:

```console
"prometheus.deckhouse.io/custom-target": "managed-postgres"
```

## Backup and restore

The PostgresSnapshot resource is used to create snapshots. The StorageClass where Postgres is placed must use a CSI driver with snapshot support, and a corresponding VolumeSnapshotClass must be available in the cluster.

The [main example](#main-example-of-creating-postgres) uses the `replicated` StorageClass, for which the provider in this configuration doesn't support creating snapshots. So for this demonstration, a separate `snapshot-local` StorageClass on `sds-local-volume` with LVM Thin is used.

Check the available snapshot classes:

```shell
d8 k get volumesnapshotclass
```

The following snapshot class is available for `snapshot-local`:

<!-- markdownlint-disable MD031 -->
```console
NAME                              DRIVER                           DELETIONPOLICY
sds-local-volume-snapshot-class   local.csi.storage.deckhouse.io   Delete
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

### Create a snapshot

To test this, create a separate `snapshot-pg` Postgres in the `snapshot-local` StorageClass:

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

To clearly verify data recovery at the moment the snapshot was created, use a check table: add the `BEFORE_SNAPSHOT` row before creating the snapshot, and `AFTER_SNAPSHOT` after.

Create the check table and insert the first row:

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

Example output:

```console
 id |      value
----+-----------------
  1 | BEFORE_SNAPSHOT
(1 row)
```

Create the PostgresSnapshot resource:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresSnapshot
metadata:
  name: snapshot-pg-backup
  namespace: postgres
spec:
  postgresName: snapshot-pg
```

Apply the manifest:

```shell
d8 k apply -f snapshot-pg-backup.yaml
```

Check the snapshot status:

```shell
d8 k get postgressnapshot snapshot-pg-backup -n postgres \
  -o jsonpath='{.status.phase}{"\n"}'
```

After the snapshot is created successfully, the command returns:

```console
completed
```

Check the created VolumeSnapshot:

```shell
d8 k get volumesnapshot -n postgres
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                         READYTOUSE   SOURCEPVC               RESTORESIZE   SNAPSHOTCLASS
d8ms-pg-snapshot-pg-backup   true         d8ms-pg-snapshot-pg-1   2Gi           sds-local-volume-snapshot-class
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

`READYTOUSE=true` confirms the snapshot is ready for recovery.

After the snapshot is created, add a second check row to the original database:

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

Example output:

```console
 id |      value
----+-----------------
  1 | BEFORE_SNAPSHOT
  2 | AFTER_SNAPSHOT
(2 rows)
```

### Restore from a PostgresSnapshot

To restore, create a new Postgres resource and specify the created PostgresSnapshot in [`spec.dataSource.objectRef`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-datasource-objectref). You don't need to delete the original Postgres:

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

The `type` and `instance` fields must be specified explicitly — they aren't inherited from the original Postgres. After that, apply the manifest:

```shell
d8 k apply -f snapshot-pg-restored.yaml
```

Wait for the restored Postgres to become ready:

```shell
d8 k get postgres snapshot-pg-restored -n postgres -o wide -w
```

After the restored PostgreSQL starts, check the check table:

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

Example output:

```console
 id |      value
----+-----------------
  1 | BEFORE_SNAPSHOT
(1 row)
```

The presence of only `BEFORE_SNAPSHOT` confirms that the database state was restored to the moment the snapshot was created.

## Check status

The current state of Postgres is reflected in `status.conditions` of this resource.

For a quick check, use:

```shell
d8 k get postgres app-postgres -n postgres -o wide
```

Main conditions:

| Condition | What it shows |
| :-- | :-- |
| `ConfigurationValid` | The configuration passed the checks of the related PostgresClass |
| `LastValidConfigurationApplied` | The last valid configuration was applied |
| `ScaledToLastValidConfiguration` | The instances match the last valid configuration |
| `Available` | Postgres is available |
| `UsersSynced` | Users are synchronized |
| `DatabasesSynced` | Logical databases are synchronized |

While resources or PostgreSQL parameters are being changed, some conditions may temporarily be `False`, while `Available` stays `True`.

To watch the status change:

```shell
d8 k get postgres app-postgres -n postgres -o wide -w
```

To see the details:

```shell
d8 k get postgres app-postgres -n postgres -o yaml
```

If Postgres doesn't reach the ready state, see the [Frequently Asked Questions](faq.html) section for diagnostics.
