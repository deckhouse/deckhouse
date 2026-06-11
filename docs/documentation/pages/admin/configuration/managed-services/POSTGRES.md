---
title: "Managed PostgreSQL"
permalink: en/admin/configuration/managed-services/postgres.html
description: "Administering the managed PostgreSQL service in Deckhouse Kubernetes Platform"
---

Managed PostgreSQL in Deckhouse Kubernetes Platform adds an API for creating and maintaining PostgreSQL instances in the cluster. This page describes service administration: enabling the [`managed-postgres`](/modules/managed-postgres/) module and preparing PostgresClass classes for users.

Before you enable [`managed-postgres`](/modules/managed-postgres/), meet the [installation requirements](/modules/managed-postgres/configuration.html#requirements). For user operations with PostgreSQL services, see [Using Managed PostgreSQL](../../../user/managed-services/postgres.html).

## Basic Managed PostgreSQL configuration

To prepare Managed PostgreSQL for users:

1. [Enable the `managed-postgres` module](/modules/managed-postgres/configuration.html) using one of the methods described on the module configuration page in the "How to explicitly enable the module..." block.
2. Check whether the limits and default values in the automatically created `default` PostgresClass are suitable.
3. Select a PostgresClass for users:
   - if the `default` PostgresClass is suitable for user Postgres resources, provide users with the `default` name;
   - if you need a separate PostgreSQL configuration, prepare a custom PostgresClass manifest, apply it, and provide users with the name of the created PostgresClass.

The following sections describe what happens after the module is enabled and which settings are included in PostgresClass preparation.

## After Enabling Managed PostgreSQL

The `managed-postgres` module automatically creates the `default` PostgresClass resource.

The controller is also deployed in the `d8-managed-postgres` system namespace. It reconciles the state of Postgres resources in all user namespaces.

## Prepare PostgresClass

The PostgresClass resource is a cluster-wide resource that describes a managed PostgreSQL service class for user Postgres resources. Use it to:

- define allowed PostgreSQL topologies;
- limit CPU and memory;
- configure default configuration values;
- define which parameters users can override;
- add validation rules.

Each Postgres resource must reference an existing PostgresClass through the `spec.postgresClassName` parameter.

To view the automatically created `default` PostgresClass, run:

```shell
d8 k get PostgresClass default -o yaml
```

When reviewing the `default` PostgresClass, check topology, sizing policies, default PostgreSQL parameter values, the list of parameters users can override, validation rules, and pod scheduling parameters.

If the `default` PostgresClass is suitable for your requirements, provide users with the `default` name; if you need a separate PostgreSQL configuration, prepare a custom manifest using the following sections. Examples show separate `spec` fragments; a complete manifest is provided at the end of the section.

### Configure topology

In PostgresClass, you can limit allowed topologies, define the default topology, and set the list of zones available for PostgreSQL instance placement.

Topologies available to users are listed in the [`spec.topology.allowedTopologies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedtopologies) parameter of the PostgresClass resource. If the [`spec.cluster.topology`](/modules/managed-postgres/cr.html#postgres-v1alpha1-spec-cluster-topology) parameter is not set in a Postgres resource, the controller applies the value from [`spec.topology.defaultTopology`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-defaulttopology). For `Zonal` and `TransZonal` topologies, the list of available zones is set in the [`spec.topology.allowedZones`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-topology-allowedzones) parameter.

| Topology | Features | Where it is defined |
| --- | --- | --- |
| `Ignored` | PostgreSQL instance placement is not bound to zones. Use it for clusters without zone separation or when zone-aware placement is not important. | Allowed in `spec.topology.allowedTopologies`; can be the value of `spec.topology.defaultTopology`. |
| `Zonal` | PostgreSQL instances are placed in one zone from the `spec.topology.allowedZones` list. | Allowed in `spec.topology.allowedTopologies`; users select it in `spec.cluster.topology`. |
| `TransZonal` | PostgreSQL instances are distributed across several zones from the `spec.topology.allowedZones` list. | Allowed in `spec.topology.allowedTopologies`; users select it in `spec.cluster.topology`. |

Example:

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

### Configure sizing policies

Administrators can control PostgreSQL instance sizes available to users by defining CPU and memory ranges and allowed CPU fractions. This helps keep resource consumption within the selected PostgresClass limits and prevents configurations that do not meet service requirements.

Sizing policies are configured in the [`spec.sizingPolicies`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-sizingpolicies) parameter of the PostgresClass resource.

The `cores.min`–`cores.max` ranges must not overlap between policies.

Example:

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

### Configure validation rules

Administrators can configure additional validation rules for the resulting PostgreSQL configuration. These rules let the controller reject Postgres resources with unwanted parameter combinations, for example when the number of connections is too high for the selected memory size.

Validation rules are configured in the [`spec.validations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-validations) parameter of the PostgresClass resource. The CEL language is supported.

Rules can use PostgreSQL parameter values after `spec.configuration` and user overrides are applied, as well as the selected instance size:

- `configuration.maxConnections`;
- `configuration.workMem`;
- `configuration.sharedBuffers`;
- `configuration.walKeepSize`;
- `instance.memory.size`;
- `instance.cpu.cores`.

Example:

```yaml
spec:
  validations:
    - message: "Max connections should not be more than 300"
      rule: "configuration.maxConnections < 300"
    - message: "Shared buffers should not be more than 25% of RAM"
      rule: "configuration.sharedBuffers < instance.memory.size / 4"
```

### Configure overridable parameters

PostgresClass separates baseline PostgreSQL parameter values from the user's ability to override them in a Postgres resource. Administrators can allow users to override only the parameters they should control.

Parameters available for override are configured in [`spec.overridableConfiguration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-overridableconfiguration). The same parameters can be used when configuring default values in `spec.configuration`.

The following values are supported:

- `maxConnections`;
- `sharedBuffers`;
- `workMem`;
- `walKeepSize`.

Example:

```yaml
spec:
  overridableConfiguration:
    - maxConnections
    - workMem
```

### Configure default values

After choosing which parameters users can override, administrators can define a baseline PostgreSQL configuration for all Postgres resources that reference this PostgresClass. Default values are applied automatically, so users get a ready configuration without setting every parameter manually.

Default values are configured in [`spec.configuration`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-configuration). If a parameter is allowed in `spec.overridableConfiguration` and is set in the Postgres resource, the value from Postgres takes precedence.

Example:

```yaml
spec:
  configuration:
    maxConnections: 100
    workMem: 100Mi
```

The operator sets the following default values:

- `maxConnections`: `100`;
- `sharedBuffers`: 25% of `memory.size`;
- `workMem`: (`memory.size` - `sharedBuffers`) * 4 / `maxConnections`;
- `walKeepSize`: `512Mi`.

### Configure pod scheduling

To control placement of the PostgreSQL service on specific nodes, administrators can use standard Kubernetes scheduling mechanisms: `nodeAffinity`, `nodeSelector`, and `tolerations`. For example, this allows PostgreSQL instances to run on dedicated nodes with specific labels or to be scheduled onto nodes with taints.

Scheduling parameters are configured in PostgresClass:

- [`spec.nodeAffinity`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeaffinity);
- [`spec.nodeSelector`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-nodeselector);
- [`spec.tolerations`](/modules/managed-postgres/cr.html#postgresclass-v1alpha1-spec-tolerations).

#### nodeAffinity example

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

#### nodeSelector example

```yaml
spec:
  nodeSelector:
    "node.deckhouse.io/group": "pg"
```

#### tolerations example

```yaml
spec:
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

### Complete PostgresClass manifest example

After selecting the parameters, create a `postgresclass.yaml` file with a PostgresClass manifest.

The following manifest example defines topology, sizing policies, validation rules, configuration values, overridable parameters, and pod scheduling parameters:

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

To apply a PostgresClass manifest, run:

```shell
d8 k apply -f postgresclass.yaml
```
