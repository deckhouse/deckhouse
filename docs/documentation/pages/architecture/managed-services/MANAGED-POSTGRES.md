---
title: Managed-postgres module
permalink: en/architecture/managed-services/managed-postgres.html
search: managed-postgres, postgresql
description: Architecture of the managed-postgres module in Deckhouse Kubernetes Platform.
---

The [`managed-postgres`](/modules/managed-postgres/) module manages PostgreSQL clusters in Deckhouse Kubernetes Platform (DKP). It allows users to configure and scale PostgreSQL clusters according to their needs, ensuring optimal performance and security.

Main features:

* **Automatic Deployment**: Deploys a Postgres instance using a simple YAML configuration
* **High Availability**: Supports deployment of a highly available Postgres cluster or a standalone instance of your choice.
* **Configuration Management**: Separate PostgresClass custom resource for templating the cluster creation approach with the ability to flexibly validate user configs
* **User and Database Management**: Declarative model for creating users and logical databases.
* **Status**: Informative set of states for tracking the deployed Postgres.

For more details about module settings and usage examples, refer to [the module documentation](/modules/managed-postgres/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica per Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-postgres`](/modules/managed-postgres/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-postgres module architecture](../../images/architecture/managed-services/c4-l2-managed-postgres.ru.png)

## Module components

The module consists of the following components:

1. **Managed-postgres-operator**: Kubernetes operator consisting of a single container **manager** that performs the following operations:

   * It reconciles [Postgres](/modules/managed-postgres/stable/cr.html#postgres) custom resources in all user namespaces. The Postgres resource defines the settings of the PostgreSQL cluster, including the topology and the replication mode, PostgreSQL instances configuration, and other parameters such as lists of logical databases and internal users.

   * It reconciles [PostgresSnapshot](/modules/managed-postgres/stable/cr.html#postgressnapshot) custom resources. PostgresSnapshot resource designed for [PostgreSQL instances backup and restore](https://deckhouse.ru/modules/managed-postgres/stable/snapshots.html);

   * It performs Postgres and PostgresClass custom resources validation, as well as Postgres custom resources mutation using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

   Managed-postgres-operator uses the `cnpg.internal.managed.deckhouse.io` API group custom resources managed by the d8-cnpg-operator component as a backend.

1. **D8-cnpg-operator**: A fork of [CloudNativePG](https://github.com/cloudnative-pg/cloudnative-pg), a Kubernetes operator that automates the management of PostgreSQL clusters. D8-cnpg-operator consists of a single container **manager** and performs the following operations:

   * It manages following `cnpg.internal.managed.deckhouse.io` API group custom resources:

     * Cluster: Defines a PostgreSQL cluster.
     * Pooler: Defines connection pool settings.
     * FailoverQuorum: It is used to display a quorum status of PostgreSQL cluster replicas in High Available configuration.
     * Database: Defines a logical data base.
     * Backup: Defines a PostgreSQL instance backup.
     * ScheduledBackup: Defines a PostgreSQL scheduled backup settings.
     * Subscription: Defines a logical replication destination.
     * Publication: Defines a logical replication source.
     * ImageCatalog: Defines PostgreSQL images for user namespaces.
     * ClusterImageCatalog: Defines cluster-wide PostgreSQL images.

   * It performs `cnpg.internal.managed.deckhouse.io` API group custom resources validation and mutation using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

   {% alert level="info" %}
   The components for [ConsistencyAndAvailability](/modules/managed-postgres/stable/user_guide.html#replication-modes-and-operational-trade-offs) replication mode are described below. The components for other two replication modes are the part of components for [ConsistencyAndAvailability](/modules/managed-postgres/stable/user_guide.html#replication-modes-and-operational-trade-offs) mode.
   {% endalert %}

1. **d8ms-pg-\<instance name>\-1-initdb** (Job): A job created by the d8-cnpg-operator component that runs SQL queries to complete initialization of the primary PostgreSQL instance.

   It consists of the following containers:

   * **bootstrap-controller**: Init container that installs `manager` executable file of d8-cnpg-operator component.
   * **initdb**: Main container that runs `manager` executable file, which executes the SQL queries described above.

1. **d8ms-pg-\<instance name>\-2-join** (Job): A job created by the d8-cnpg-operator component that runs `pg_basebackup` script to get `data` directory from primary instance for first replica. This job uses a Cluster custom resource to configure the replica.

   It consists of the following containers:

   * **bootstrap-controller**:  Init container that installs `manager` executable file of d8-cnpg-operator component.
   * **join**: Main container that runs `manager` executable file, which executes the script described above.

1. **d8ms-pg-\<instance name>\-3-join** (Job): A job created by the d8-cnpg-operator component that runs `pg_basebackup` script to get `data` directory from primary instance for second replica. This job uses a Cluster custom resource to configure the replica.

   It consists of the following containers:

   * **bootstrap-controller**:  Init container that installs `manager` executable file of d8-cnpg-operator component.
   * **join**: Main container that runs `manager` executable file, which executes the script described above.

1. **d8ms-pg-\<instance name>\-1**: PostgreSQL primary instance. It is created by the d8-cnpg-operator component.

   It consists of the following containers:

   * **bootstrap-controller**: Init container that installs `manager` executable file of d8-cnpg-operator component.
   * **postgres**: Main container that runs `manager` executable file, which performs following operations:

     * It starts PostgreSQL processes.
     * It manages PostreSQL instance lifecycle, including server monitoring, handling server shutdown and restart.
     * It participates `switchover`/`failover` procedures. `switchover` is a planned and controlled process in which an active primary instance is intentionally decommissioned and an assigned backup instance (replica) is promoted to the primary instance role. Its main goal is to ensure zero data loss: before transferring the role to the replica, we wait until all current transactions are replicated. A `Failover` is an emergency situation: the primary instance has failed, become unavailable, or cannot be safely used to write data. In this case, the designated backup instance (replica) is promoted to the primary instance role, and data loss is possible.
     * It interacts with the operator and publish instance status;
     * It watches Cluster, Database, Publication and Subscription custom resources.

1. **d8ms-pg-\<instance name>\-2**: PostgreSQL first replica. It is created by the d8-cnpg-operator component.

   It consists of the following containers:

   * **bootstrap-controller**: Init container that installs `manager` executable file of d8-cnpg-operator component.
   * **postgres**: Main container that runs `manager` executable file, which performs operations described for PostgreSQL primary instance.

1. **d8ms-pg-\<instance name>\-3**: PostgreSQL second replica. It is created by the d8-cnpg-operator component.

   It consists of the following containers:

   * **bootstrap-controller**: Init container that installs `manager` executable file of d8-cnpg-operator component.
   * **postgres**: Main container that runs `manager` executable file, which performs operations described for PostgreSQL primary instance.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Manages Postgres and PostgresSnapshot custom resources.
   * Manages `cnpg.internal.managed.deckhouse.io` API group custom resources.

The following external components interact with the module:

1. **Kube-apiserver**:

   * Validates Postgres and PostgresClass custom resources, mutates Postgres custom resources.
   * Validates and mutates `cnpg.internal.managed.deckhouse.io` API group custom resources.

1. **Prometheus-main**: Collects PostgreSQL cluster instances metrics (which are exported by `manager` executable file of d8-cnpg-operator component that is run in postgres container of PostgreSQL instance pod).

1. **opAgent**: Collects PostgreSQL cluster instances metrics (connecting directly to PostgreSQL cluster instances) and sends them to prometheus-main.

1. **User applications**: Sends requests to PostgreSQL cluster instances. Write requests are sent to the primary instance via the **d8ms-pg-\<instance name>\-rw** service. The primary instance replicates transactions to replicas.  Read requests are balanced to PostgreSQL instances via **d8ms-pg-\<instance name>\-r** or **d8ms-pg-\<instance name>\-ro** services.
