---
title: "Managed PostgreSQL"
permalink: en/admin/configuration/managed-services/postgres/
description: "Administering managed PostgreSQL in Deckhouse Kubernetes Platform."
---

The Managed PostgreSQL service helps you deploy and maintain PostgreSQL instances in DKP. [Supported PostgreSQL version](/modules/managed-postgres/user_guide.html#supported-postgresql-versions) — 17.6.

This page describes the settings a cluster administrator can configure: managing resource limits and topology, configuring parameter validation, setting default values, and binding to cluster nodes.

## Enabling the service

To enable the Managed PostgreSQL service in DKP, complete the following steps:

1. Make sure the [requirements for the `managed-postgres` module](/modules/managed-postgres/configuration.html#requirements) are met.
1. [Enable the `managed-postgres` module](/modules/managed-postgres/configuration.html#enable) in the DKP web interface or another way.
1. Create a [PostgresClass](/modules/managed-postgres/cr.html#postgresclass-v1alpha1) with the settings that users will need when creating Postgres objects. You can also use the `default` PostgresClass that the module creates when enabled. The `default` PostgresClass contains baseline settings so users can start creating databases right after Managed PostgreSQL is enabled in DKP. For production environments, it's recommended to create separate PostgresClass resources with explicit settings and limits.

Once the module is enabled, users can create PostgreSQL databases on their own. User operations with the service are described in [Usage → Managed services → Managed PostgreSQL](../../../user/managed-services/postgres.html).

## Dependencies for specific features

Some module features require additional configuration of Deckhouse Kubernetes Platform components or cluster infrastructure:

| Feature                                                  | Requirement                                                                                                                                                                                 |
|-----------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Backup (managed by [PostgresSnapshot](/modules/managed-postgres/cr.html#postgressnapshot)) | The [snapshot-controller](/modules/snapshot-controller/) module enabled and a StorageClass with snapshot support                                                                    |
| TLS via `cert-manager`                                   | The [cert-manager](/modules/cert-manager/) module enabled and a configured ClusterIssuer or Issuer                                                                                          |
| Placement on dedicated nodes                             | Node labels (for example, `node.deckhouse.io/group=pg`) and taints if needed. For more information, see [the documentation](../../../admin/configuration/platform-scaling/node/node-management.html) |

## Example of creating a PostgresClass

Before applying the example, make sure the cluster has enough resources. To estimate the available resources, run:

```shell
d8 k describe node worker-0 | grep -A 5 "Allocated resources"
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests          Limits
  --------           --------          ------
  cpu                3176m (80%)       500m (12%)
  memory             8342837084 (71%)  6400Mi (57%)
```
{:.nowrap-default }
<!-- markdownlint-enable MD031 -->

Below is an example of a `production-v1` PostgresClass that you can use instead of the `default` PostgresClass and adapt to your requirements for resources, topology, PostgreSQL parameters, and node placement. The following sections describe these parameters in more detail.

Create a `postgresclass-production.yaml` file with the following content:

```yaml
apiVersion: managed-services.deckhouse.io/v1alpha1
kind: PostgresClass
metadata:
  name: production-v1
spec:
  # Limit CPU and memory resources.
  sizingPolicies:
    - cores:
        min: 1
        max: 2
      memory:
        min: 512Mi
        max: 2Gi
        step: 512Mi
      coreFractions:
        - 50
        - 100
    - cores:
        min: 3
        max: 4
      memory:
        min: 2Gi
        max: 4Gi
        step: 1Gi
      coreFractions:
        - 50
        - 100

  # Manage fault tolerance.
  topology:
    allowedTopologies:
      - Ignored
      - Zonal
      - TransZonal
    defaultTopology: TransZonal
    allowedZones:
      - zone-a
      - zone-b
      - zone-c

  # Validate PostgreSQL settings.
  validations:
    - message: "Max connections should not be more than 300"
      rule: "configuration.maxConnections <= 300"
    - message: "Shared buffers should not be more than 25% of RAM"
      rule: "configuration.sharedBuffers < instance.memory.size / 4"
    - message: "walKeepSize can not be more than 1Gi"
      rule: "configuration.walKeepSize <= 1073741824"

  # Default values and override permissions.
  configuration:
    maxConnections: 200
    sharedBuffers: 1Gi
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize

  # Bind to dedicated nodes.
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: "node.deckhouse.io/group"
              operator: "In"
              values:
                - "pg"
                - "postgres"
  nodeSelector:
    "node.deckhouse.io/group": "pg"
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

Apply the manifest:

```shell
d8 k apply -f postgresclass-production.yaml
```

Check the created PostgresClass:

```shell
d8 k describe postgresclass production-v1
```

{% offtopic title="Example output..." %}

```console
Name:         production-v1
Namespace:    
Labels:       <none>
Annotations:  <none>
API Version:  managed-services.deckhouse.io/v1alpha1
Kind:         PostgresClass
Metadata:
  Creation Timestamp:  2026-08-06T13:08:24Z
  Generation:          1
  Resource Version:    21610964
  UID:                 f2548e11-a23c-4bd1-b786-776bf1677a1d
Spec:
  Configuration:
    Max Connections:  200
    Shared Buffers:   1Gi
  Node Affinity:
    Required During Scheduling Ignored During Execution:
      Node Selector Terms:
        Match Expressions:
          Key:       node.deckhouse.io/group
          Operator:  In
          Values:
            pg
            postgres
  Node Selector:
    node.deckhouse.io/group:  pg
  Overridable Configuration:
    maxConnections
    sharedBuffers
    walKeepSize
  Sizing Policies:
    Core Fractions:
      50
      100
    Cores:
      Max:  2
      Min:  1
    Memory:
      Max:   2Gi
      Min:   512Mi
      Step:  512Mi
    Core Fractions:
      50
      100
    Cores:
      Max:  4
      Min:  3
    Memory:
      Max:   4Gi
      Min:   2Gi
      Step:  1Gi
  Tolerations:
    Effect:    NoSchedule
    Key:       primary-role
    Operator:  Equal
    Value:     pg
  Topology:
    Allowed Topologies:
      Ignored
      Zonal
      TransZonal
    Allowed Zones:
      zone-a
      zone-b
      zone-c
    Default Topology:  TransZonal
  Validations:
    Message:  Max connections should not be more than 300
    Rule:     configuration.maxConnections <= 300
    Message:  Shared buffers should not be more than 25% of RAM
    Rule:     configuration.sharedBuffers < instance.memory.size / 4
    Message:  walKeepSize can not be more than 1Gi
    Rule:     configuration.walKeepSize <= 1073741824
Events:       <none>
```

{% endofftopic %}

Users can use the created PostgresClass by specifying it in the [postgresClassName](/modules/managed-postgres/stable/cr.html#postgres-v1alpha1-spec-postgresclassname) parameter of Postgres.

{% alert level="warning" %}
Once a PostgresClass is created, its parameters can't be changed. Create a new PostgresClass with a different name to use different settings.
{% endalert %}

## Change and delete a PostgresClass

Before deleting an old class, check whether it's used by any existing Postgres objects. Run:

```shell
d8 k get postgres --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,CLASS:.spec.postgresClassName | grep production-v1
```

If Postgres objects using this PostgresClass are found, keep the following in mind:
- Databases created based on the deleted PostgresClass keep running. Their settings stay the same, since they were fixed at creation time.
- Users won't be able to create a new Postgres object that references the deleted class.

If the PostgresClass is no longer used, you can delete it.

Example of deleting the `production-v1` PostgresClass via the CLI:

```shell
d8 k delete postgresclass production-v1
```

Example output:

```console
postgresclass.managed-services.deckhouse.io "production-v1" deleted
```

{% alert level="info" %}
If you need to change PostgresClass settings, it's recommended to create a new one with a different name rather than recreating a PostgresClass with the same name.
{% endalert %}

## Limit CPU and memory resources

Policies, in the [`spec.sizingPolicies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies) parameter section, are used to define the allowed CPU and memory combinations for PostgreSQL instances. The administrator can set several core ranges, each with a minimum and maximum amount of memory and a step. These policies are useful when several independent teams work in the same cluster and create Postgres objects without coordinating with the administrator. Policies prevent Postgres objects with unrealistic resource requests and ensure predictable node utilization.

The policy is selected by the number of CPU cores. The [`coreFractions`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies-corefractions) parameter defines what percentage of the CPU limits (`limits`) becomes the guaranteed request (`requests`). For example, if the administrator specifies `coreFractions: [50, 100]`, a user creating a Postgres object can choose `coreFraction: 50` or `coreFraction: 100`.

With `coreFraction: 50`:

- The CPU `limits` equal the number of requested cores (for example, 4 cores).
- The CPU `requests` are 50% of `limits` (2 cores).

The scheduler guarantees the pod 2 cores but allows it to use up to 4 if they're available. This increases pod placement density on nodes.

With `coreFraction: 100`:

- `limits` and `requests` are equal (4 cores).
- The pod gets a guaranteed resource allocation, but loses the ability to reuse unused cores from other pods.

Below is a fragment of a PostgresClass manifest showing the allowed CPU and memory combinations. The complete example is provided in [Example of creating a PostgresClass](#example-of-creating-a-postgresclass).

```yaml
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 2
      memory:
        min: 512Mi
        max: 2Gi
        step: 512Mi
      coreFractions:
        - 50
        - 100
```

In this fragment, the available memory values for 1–2 cores are 512Mi, 1Gi, 1.5Gi, and 2Gi.

The `step` parameter defines the increment for the memory amount. A user can only specify a memory value that's a multiple of `step`. For example, if `step: 512Mi`, the allowed values are 512Mi, 1Gi, 1.5Gi, 2Gi, and so on. A value of 700Mi is rejected because it isn't a multiple of 512Mi.

The choice of memory depends on the number of cores. If a user requests 5 cores, the Postgres object isn't created, since this configuration isn't provided by the administrator.

{% alert level="warning" %}
Within a single PostgresClass, the `cores.min`–`cores.max` ranges of different policies must not overlap.
{% endalert %}

For example, in the `production-v1` PostgresClass, the first policy can't cover 1–4 cores while the second covers 2–6 cores, since cores 2–4 would belong to both policies. Ranges can overlap across different PostgresClass resources, for example `production-v1` and `production-v2`.

The user can see the available options and pick a suitable one when creating a Postgres object.

## Manage fault tolerance across availability zones

The administrator can manage the fault tolerance of users' PostgreSQL instances by setting their availability zone distribution topology in the [`spec.topology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology) parameter section. This can be useful in production environments with data-center-level fault tolerance requirements.

Three topologies are available:

- `Ignored`: standard scheduling with no zone binding;
- `Zonal`: all instances are placed in one zone (minimal latency between replicas). Suits low-latency environments where losing a zone is acceptable;
- `TransZonal`: instances are distributed across different zones (one primary instance, one synchronous replica, one asynchronous replica). Protects against the loss of an entire zone but requires more resources.

The administrator can specify the allowed topology options ([`allowedTopologies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedtopologies)), the default topology ([`defaultTopology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-defaulttopology)), and the list of available zones ([`allowedZones`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedzones)).

Below is a fragment of a PostgresClass manifest showing the topology parameters. The complete example is provided in [Example of creating a PostgresClass](#example-of-creating-a-postgresclass).

```yaml
spec:
  topology:
    allowedTopologies:
      - Zonal
      - TransZonal
    defaultTopology: TransZonal
    allowedZones:
      - zone-a
      - zone-b
```

In this fragment, `Zonal` and `TransZonal` modes are allowed, with `TransZonal` applied by default. The `allowedZones` field lists placeholder zones — replace them with the actual zone names from your cloud provider or data center.

Users can choose the fault tolerance level when creating a Postgres object. If not specified explicitly, the default value applies (as set in the `defaultTopology` parameter).

## Automatically validate PostgreSQL settings

To reject undesirable combinations of PostgreSQL parameters chosen by users, the administrator can define validation rules in Common Expression Language (CEL) in the [`spec.validations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-validations) parameter section of PostgresClass. These rules are checked on the API server before Postgres objects are created.

These rules are useful for guarding against common configuration mistakes that can lead to performance issues. For example, too large a `shared_buffers` value can leave PostgreSQL with too little memory for other operations, and a large number of connections can significantly increase overall memory consumption.

The following variables are available in the rules:

- `configuration.maxConnections`;
- `configuration.workMem`;
- `configuration.sharedBuffers`;
- `configuration.walKeepSize`;
- `instance.memory.size`;
- `instance.cpu.cores`.

Below is a fragment of a PostgresClass manifest showing validation rules. The complete set of rules is provided in [Example of creating a PostgresClass](#example-of-creating-a-postgresclass).

```yaml
spec:
  validations:
    - message: "Shared buffers should not be more than 25% of RAM"
      rule: "configuration.sharedBuffers < instance.memory.size / 4"
```

In this fragment, the rule prevents allocating more than 25% of the requested memory to `shared_buffers`.

The user gets a validation error when trying to apply invalid parameters, which helps avoid configuration mistakes without manual review.

## Set default values and allow overrides

To set default PostgreSQL parameters and define which of them users can change, the administrator can use the [`spec.configuration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-configuration) and [`spec.overridableConfiguration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-overridableconfiguration) parameters.

If no values are specified in `configuration`, the module controller applies the following:

- `maxConnections`: `100`;
- `sharedBuffers`: 25% of `memory.size`;
- `workMem`: `(memory.size - sharedBuffers) * 4 / maxConnections`;
- `walKeepSize`: `512Mi`.

Below is a fragment of a PostgresClass manifest showing the default service settings. The complete example is provided in [Example of creating a PostgresClass](#example-of-creating-a-postgresclass).

```yaml
spec:
  configuration:
    maxConnections: 200
    sharedBuffers: 1Gi
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize
```

In this example:

- `maxConnections` and `sharedBuffers` default to 200 and 1Gi, respectively.
- The user can override `maxConnections`, `sharedBuffers`, and `walKeepSize`.
- `workMem` isn't included in `overridableConfiguration`, so the user can't change it.

### Automatic workMem calculation

To limit the amount of memory used for sort and hash operations within a single query, use the [workMem](/modules/managed-postgres/stable/cr.html#postgres-v1alpha1-spec-configuration-workmem) parameter.

In PostgreSQL, this limit applies to each operation in each active session. A complex query can run several such operations at once. With a large number of connections, total memory consumption can far exceed the configured value and lead to memory exhaustion. A value that's too small, on the other hand, forces PostgreSQL to use temporary files on disk and reduces performance.

If `workMem` isn't set explicitly, the `managed-postgres` module automatically calculates its value based on the allocated resources.

The module calculates `workMem` using the following formula:

```text
workMem = (instance.memory.size - configuration.sharedBuffers) * 4 / configuration.maxConnections
```

For example, for Postgres with the following parameters:

- `instance.memory.size: 4Gi`
- `configuration.sharedBuffers: 1Gi`
- `configuration.maxConnections: 200`

The `workMem` calculation looks as follows:

```text
workMem = (4Gi - 1Gi) * 4 / 200
workMem = 3Gi * 4 / 200
workMem = 12Gi / 200
workMem = 61.44Mi
```

The `* 4` multiplier is a safety margin for the case of several concurrent sort and hash operations within a single query.

The controller calculates `workMem` using the formula above, based on the instance's memory size, `sharedBuffers`, `maxConnections`, and the safety margin.

## Bind to dedicated nodes

The standard Kubernetes mechanisms — [`spec.nodeSelector`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeselector), [`spec.tolerations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-tolerations), and [`spec.nodeAffinity`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeaffinity) — let you specify which nodes PostgreSQL pods can be placed on.

Placement on dedicated nodes helps isolate PostgreSQL instances from user applications and makes disk and network resource usage more predictable.

Below is a fragment of a PostgresClass manifest showing `spec.nodeAffinity`, `spec.nodeSelector`, and `spec.tolerations`. The complete example is provided in [Example of creating a PostgresClass](#example-of-creating-a-postgresclass).

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
  nodeSelector:
    "node.deckhouse.io/group": "pg"
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

In this example:

- Pods are placed only on nodes with the `node.deckhouse.io/group=pg` label.
- Nodes have the `primary-role=pg:NoSchedule` taint, which prevents scheduling pods without a matching toleration.
- `tolerations` allows PostgreSQL pods to be scheduled on such nodes.

Users don't need to worry about node selection. Pods are automatically placed on the prepared infrastructure according to the PostgresClass settings.

If no node meets the placement rules, users' Postgres objects will remain in the `Pending` state. To determine the cause, you can check the events in the namespace, the Postgres object information, and the PVC status:

```shell
d8 k describe pods -n my-postgres <POD_NAME>
d8 k get events -n my-postgres --sort-by=.lastTimestamp
d8 k get pvc -n my-postgres
```
