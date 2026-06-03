---
title: "managed-postgres"
permalink: en/admin/configuration/managed-services/postgres.html
description: "Administering the managed PostgreSQL service in Deckhouse Kubernetes Platform"
---

The managed PostgreSQL service provides PostgreSQL clusters in Deckhouse Kubernetes Platform. The service is in the [`Preview` stage](/products/kubernetes-platform/documentation/v1/architecture/module-development/versioning/#module-lifecycle). Before you enable [`managed-postgres`](/modules/managed-postgres/), meet the [installation requirements](/modules/managed-postgres/configuration.html#requirements). The main administrator cluster-wide resource is PostgresClass. It defines limits and default values for related Postgres resources. For instructions on creating and using PostgreSQL services, see [Using managed PostgreSQL](../../../user/managed-services/postgres.html).

## Before you begin

Make sure that:

- [`managed-postgres`](/modules/managed-postgres/) is available in your installation.
- The [installation requirements](/modules/managed-postgres/configuration.html#requirements) are met.
- You have permission to create cluster-wide resources.

## Enable managed-postgres

To enable the managed PostgreSQL service, apply the ModuleConfig resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: managed-postgres
spec:
  enabled: true
  version: 1
```

After `managed-postgres` is enabled, the `default` PostgresClass resource is created automatically.

The controller is also deployed in the `d8-managed-postgres` system namespace. It reconciles the state of Postgres resources in all user namespaces.

## PostgresClass resource

The PostgresClass resource is a cluster-wide resource. It is used to:

- define allowed PostgreSQL topologies;
- define CPU and memory limits;
- configure default configuration values;
- define which parameters users can override;
- add validation rules.

Each Postgres resource must reference an existing PostgresClass through the `spec.postgresClassName` parameter.

## Configure topology

In PostgresClass, you can limit allowed topology options and define the default topology. The following values are supported:

- `Ignored`;
- `Zonal`;
- `TransZonal`.

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

## Configure sizing policies

The `spec.sizingPolicies` parameter defines allowed CPU and memory ranges for related Postgres resources. The `cores.min`–`cores.max` ranges must not overlap between policies.

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

## Configure validation rules

For PostgresClass, you can define validation rules in the `spec.validations` parameter. The CEL language is supported. The following predefined variables are available:

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

## Configure overridable parameters

The `spec.overridableConfiguration` parameter defines an allowlist of PostgreSQL parameters that users can set in the Postgres resource. The following values are supported:

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

## Configure default values

In `spec.configuration` of the PostgresClass resource, you can define default PostgreSQL configuration values. If a parameter is allowed in `overridableConfiguration` and is set in the Postgres resource, the value from Postgres takes precedence.

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

## Configure pod scheduling

For PostgresClass, you can define the following pod scheduling parameters:

- `nodeAffinity`;
- `nodeSelector`;
- `tolerations`.

### nodeAffinity example

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

### tolerations example

```yaml
spec:
  tolerations:
    - key: primary-role
      operator: Equal
      value: pg
      effect: NoSchedule
```

### nodeSelector example

```yaml
spec:
  nodeSelector:
    "node.deckhouse.io/group": "pg"
```

## PostgresClass example

The following is a complete PostgresClass resource example that defines topology, configuration values, overridable parameters, validation rules, and sizing policies:

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
  configuration:
    maxConnections: 300
  overridableConfiguration:
    - maxConnections
    - sharedBuffers
    - walKeepSize
  validations:
    - message: "Max connections should be more than 100"
      rule: "configuration.maxConnections > 100"
    - message: "Shared buffers should be less than 40% of memory.size"
      rule: "configuration.sharedBuffers * 100 < instance.memory.size * 40"
    - message: "walKeepSize can not be more than 1Gi"
      rule: "configuration.walKeepSize <= 1073741824"
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
```

## Important notes

{% alert level="warning" %}
Deckhouse Kubernetes Platform does not remove the related CRDs when [`managed-postgres`](/modules/managed-postgres/) is disabled. If you no longer need these resources, delete the corresponding CRDs manually.
{% endalert %}
