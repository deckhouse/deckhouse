---
title: DKP cluster management
permalink: en/architecture/deckhouse/commander.html
search: commander, commander-agent, modules
description: Architecture of the commander and commander-agent modules in Deckhouse Kubernetes Platform.
---

The [`commander`](/modules/commander/) and [`commander-agent`](/modules/commander-agent/) modules are used to implement cluster management in Deckhouse Kubernetes Platform (DKP).

A DKP cluster with the `commander` module installed is a management cluster.
A DKP cluster with the `commander-agent` module installed is a managed cluster.

## Сommander module

The [`commander`](/modules/commander/) module implements a web application that allows you to create standardized DKP clusters and manage their configurations and lifecycle.

To install the [`commander`](/modules/commander/) module, you need a PostgreSQL instance. It can be deployed inside or outside the cluster by using the [`managed-postgresql`](/modules/managed-postgresql/) module.

{% alert level="warning" %}
For `commander` in production environments, using a dedicated PostgreSQL instance is recommended.

When `commander` is installed, a PostgreSQL data encryption key is generated. This key is stored in the `d8-commander` namespace in the `commander-envs` Secret.
{% endalert %}

For details about features, configuration, and usage examples, see [the module documentation section](/modules/commander/).

### Сommander module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services  (internal load balancers). Service names are omitted if they are obvious from the context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
* The diagram shows only the main containers of each component.
{% endalert %}

The Level 2 C4 architecture of the [`commander`](/modules/commander/) module
and its interaction with other DKP components are shown in the following
diagram:

![Сommander module architecture](../../images/architecture/deckhouse/c4-l2-commander.svg)

### Сommander module components

The module consists of the following components:

#### Core components

1. **Frontend** (Deployment): Provides a web interface for cluster management.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **frontend**: The main container.

1. **Backend** (Deployment): The main component that implements the following APIs:

    * UI API: Manages cluster, template, and workspace settings.
    * External API for integrations and automation: Operates on clusters, tasks, templates, projects, roles, and more.
    * API for cluster-manager: Exchanges cluster task states and processes deployment and configuration phases of managed clusters.
    * API for commander-agent: Authorizes and receives agent metrics, and provides target configuration for a managed cluster.

   Backend provides the following capabilities:

    * Cluster lifecycle management: Creates, modifies, or deletes a cluster (through cluster-manager), restarts failed tasks, and runs related checks.
    * Cluster RBAC management and API operation access restrictions.
    * Template management and storage of target and current cluster settings.
    * Container registry management.
    * Project and project template management.
    * Billing settings management (if this component is enabled).

   The component stores the target system state in a PostgreSQL instance. It also uses a Redis instance to publish tasks to managed clusters and receive task processing results.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **backend**: The main container.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

1. **Cable** (Deployment): Provides WebSocket support for real-time UI updates.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **cable**: The main container.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

1. **Sidekiq** (Deployment): A component based on the [Sidekiq](https://github.com/sidekiq/sidekiq) library that provides queue processing in Ruby on Rails.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **sidekiq**: The main container.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

1. **Cluster-manager** (Deployment): Executes tasks requested by backend to deploy managed DKP clusters and manage their configuration.

   When a new cluster is created, cluster-manager creates the following resources in the `d8-commander` namespace of the management cluster:

    * Deployment and Secret named `dhctl-<registryHash>-<version>`: Runs [dhctl](https://github.com/deckhouse/deckhouse/tree/main/dhctl/) in gRPC server mode. Cluster-manager uses RPC calls to install and configure a DKP cluster.
    * Deployment (`ampg-connector-<uuid>`), Service (`ampg-connector-<uuid>`, `ampg-console-backend-<uuid>`, `ampg-api-server-<uuid>`, `ampg-deckhouse-tools-<uuid>`, `ampg-aggregating-proxy-<uuid>`, `ampg-upmeter-<uuid>`, `ampg-label-proxy-<uuid>`), Secret (`ampg-console-backend-<uuid>`), and Ingress (`ampg-agent-api-connector-<uuid>`, `ampg-console-backend-<uuid>`): Process incoming `commander-agent` connections and implement a reverse interaction channel with the control plane of the corresponding cluster.
    * DexClient (`cluster-<uuid>`): Configures the authentication service for a managed cluster using the capabilities of the [`user-authn`](/modules/user-authn/) module.

   During cluster bootstrap, cluster-manager also performs the following actions in the managed cluster:

   * Installs the [`commander-agent`](/modules/commander-agent/) module.
   * Disables the [`terraform-manager`](/modules/terraform-manager/) module.
   * Configures authentication based on the Dex controller of the [`user-authn`](/modules/user-authn/) module. As a Dex provider, cluster-manager sets `cluster-<uuid>` DexClient from the management cluster.
   * Configures the managed cluster connection to a container registry.
   * If billing is enabled, creates a [PrometheusRemoteWrite](/modules/prometheus/cr.html#prometheusremotewrite) custom resource to send metrics to the `billing-prometheus` component in the management cluster.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **cluster-manager**: The main container.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

1. **Dhctl-&lt;registryHash&gt;-&lt;version&gt;** (Deployment): A component with a single **dhctl** container. It runs the [dhctl](https://github.com/deckhouse/deckhouse/tree/main/dhctl/) utility in gRPC server mode. Cluster-manager uses RPC calls to the `dhctl` server to install or configure a managed DKP cluster.

1. **Ampg-connector-&lt;uuid&gt;** (Deployment): A component with a single **main** container that establishes a tunnel and proxies requests to the control plane of a managed DKP cluster through the incoming `commander-agent` connection.

   The component is created by cluster-manager for each managed DKP cluster.

1. **Cluster-checker** (Deployment): Periodically starts cluster verification tasks.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **backend**: The main container.

1. **Cluster-task-checker** (Deployment): Periodically searches for running tasks that have not reported their status for a long time and moves them to `lost` or `lost_in_critical`.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **backend**: The main container.

1. **Redis** (Deployment): A component with a single **redis** container that provides a dedicated [Redis](https://github.com/redis/redis) database instance for storing task queue data and commander session states.

1. **Console-frontend** (Deployment): Provides the managed DKP cluster administration web interface.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **nginx**: The main container.

1. **Console-backend** (Deployment): An administration API backend for the managed DKP cluster that serves requests from the cluster-manager component.

   Console-backend uses Secret and Service resources created by cluster-manager to connect to the control plane of a managed DKP cluster.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **backend**: The main container.

#### Scheduled tasks

1. **Cluster-task-cleaner** (CronJob): Runs once a day and removes completed tasks created more than 30 days ago.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **cluster-task-cleaner**: The main container.

1. **Agent-tokens-rotation** (CronJob): Regularly rotates `commander-agent` tokens.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **agent-tokens-rotation**: The main container.

1. **Encryption-migrator** (CronJob): Runs once a day and migrates encrypted data from SHA-1 to SHA-256. Not all data may be processed in a single run. In addition, after restoring from backup or due to delayed migrations, SHA-1 records may appear.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **encryption-migrator**: The main container.

1. **Audits-cleaner** (CronJob): Runs once a day and deletes old audit logs.

   The Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module creates `audits-cleaner` if the [`.settings.featureFlags.auditsRetentionDays`](/modules/commander/alpha/admin_guide.html#audit-log-retention--auditsretentiondays) module parameter is set and defines the audit log retention period.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **audits-cleaner**: The main container.

#### Optional components

1. **Billing-prometheus** (StatefulSet): Runs [Deckhouse Prom++](/products/prompp/) in metrics ingestion mode over the [Prometheus Remote Write](https://prometheus.io/docs/specs/prw/remote_write_spec/) protocol.

   The [`prometheus`](/modules/prometheus/) module in a managed DKP cluster sends metrics. The component stores resource usage history for all managed clusters, which is an important data source for billing reports.

   Contains the following containers:

    * **prometheus**: The main container.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

1. **Billing-reports** (StatefulSet): A component that runs the [nginx](https://github.com/nginx/nginx) web server as billing report storage. Reports are uploaded to nginx over WebDAV (Web-based Distributed Authoring and Versioning).

   Contains the following containers:

    * **nginx**: The main container.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

1. **Billing-schedules-reporter** (Deployment): Implements a scheduler that starts and controls tasks for generating required billing reports.

   The billing-schedules-reporter component saves generated reports to billing-reports over WebDAV.

   Contains the following containers:

    * **wait-postgres**: An init container that checks PostgreSQL instance availability.
    * **wait-migrations**: An init container that waits for all database migrations to complete.
    * **backend**: The main container.

1. **Billing-aggregating-proxy** (Deployment): Aggregates PromQL queries, proxies them to billing-prometheus, and returns query results.

   Contains the following containers:

    * **billing-aggregating-proxy**: A sidecar container based on [Grafana Mimir](https://github.com/grafana/mimir). It provides query optimization and data caching. If data is missing in the cache, Grafana Mimir forwards the request to `billing-prometheus` via the `promxy` sidecar container.
    * **promxy**: A sidecar container based on [Promxy](https://github.com/jacksontj/promxy). It proxies requests to `billing-prometheus` and provides a single endpoint for accessing data from multiple [Deckhouse Prom++](/products/prompp/) instances.
    * **kube-rbac-proxy**: A sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to the main container.

{% alert level="info" %}
The billing-prometheus, billing-reports, billing-schedules-reporter, and billing-aggregating-proxy components are created by the Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module if [`.settings.featureFlags.billingEnabled`](/modules/commander/alpha/admin_guide.html#billing-and-cost-management--billingenabled) is set to `true`.
{% endalert %}

### Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

    * Works with Deployment, Service, Secret, and Ingress resources in the `d8-commander` namespace.
    * Authorizes requests.

1. **Container registry**: Retrieves available DKP releases by update channels.

1. **PostgreSQL instance**: Stores cluster states and metadata, tasks, tokens, projects, billing parameters, and reports.

1. **Managed cluster**: Creates, modifies, or deletes DKP clusters.

The following external components interact with the module:

* **Controller-nginx**: Forwards external requests to module component endpoints.

* **Prometheus-main**: Collects metrics from module components.

## Сommander-agent module

The [`commander-agent`](/modules/commander-agent/) module establishes a service connection to a management DKP cluster.

For details about module settings, see [the module documentation section](/modules/commander-agent/).

### Сommander-agent module architecture

The Level 2 C4 architecture of the [`commander-agent`](/modules/commander-agent/) module and its interaction with other DKP components are shown in the following diagram:

![Сommander-agent module architecture](../../images/architecture/deckhouse/c4-l2-commander-agent.svg)

### Сommander-agent module components

The module consists of one **agent** component and one **commander-agent** container.

Agent performs the following actions:

* Brings managed cluster configuration in line with target configuration received from `commander` module backend.
* Collects managed cluster metrics:
  * `CPU cores`: Number of node CPU cores.
  * `Memory`: Total node memory in GiB.
  * `Filesystem`: Total size of node root disks in GiB.
  * `PVC`: Total size of attached disks in GiB.
  * `Nodes`: Total number of nodes.
* Collects cluster availability information.
* Establishes a secure connection to the management DKP cluster.
* Sends metrics and cluster availability information to the management DKP cluster.
* Configures user authentication via the Dex provider of the management cluster.
* If billing is enabled in the management cluster, creates a [PrometheusRemoteWrite](/modules/prometheus/cr.html#prometheusremotewrite) custom resource to send metrics to the billing service of the management DKP cluster.

### Module interactions

The module interacts with the following components:

1. **Kube-apiserver**: Manages resources of the managed cluster.

1. **Dex**: Synchronizes access tokens for Kubernetes API requests from the management cluster to the managed cluster.

1. **Management cluster**:

    * Organizes a proxy connection to the management cluster.
    * Receives target cluster configuration.
    * Sends metrics to the billing service.
