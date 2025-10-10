---
title: "Installation"
---

## System requirements

To start using Deckhouse Commander, you need a cluster based on Deckhouse Kubernetes Platform.

We recommend creating a fault-tolerant management cluster that will
include the following node sets ([NodeGroup](/modules/node-manager/cr.html#nodegroup)):

| Node Group | Number of nodes | CPU, cores | Memory, GB | Disk, GB |
| ---------- | --------------: | ---------: | ---------: | -------: |
| master     |               3 |          4 |          8 |       50 |
| system     |               2 |          4 |          8 |       50 |
| frontend   |               2 |          4 |          8 |       50 |
| commander  |               3 |          8 |         12 |       50 |

* PostgreSQL in [HighAvailability](../../../platform/deckhouse-configure-global.html#parameters-highavailability) mode in two replicas requires 1 core and 1 GB of memory on 2 separate nodes.
* The API server in [HighAvailability](../../../platform/deckhouse-configure-global.html#parameters-highavailability) mode for two replicas needs 1 core and 1GB of memory on two separate nodes.
* Service components used for rendering configurations and connecting to application clusters
  require 0.5 cores and 128 MB of memory per cluster.
* Cluster manager and dhctl server together require resources based on the number of clusters they
  serve and the number of DKP versions they serve simultaneously.
* Up to 2 cores per node can be occupied by DKP service components (for example:
  runtime-audit-engine, istio, cilium, log-shipper).

| Number of clusters | CPU, cores | Memory, GB | Number of 8/8 nodes | Number of 8/12 nodes |
| ------------------ | ---------: | ---------: | ------------------: | -------------------: |
| 10                 |          9 |         16 |          3 (=24/24) |           2 (=16/24) |
| 25                 |         10 |         19 |          3 (=24/24) |            3(=24/36) |
| 100                |         15 |         29 |          4 (=32/32) |           4 (=32/48) |

## Prepare DBMS

Deckhouse Commander works with the PostgreSQL database management system.
PostgreSQL extensions [plpgsql](https://www.postgresql.org/docs/14/plpgsql.html) and [pgcrypto](https://www.postgresql.org/docs/14/pgcrypto.html) are required for Deckhouse Commander to function properly.

### Option 1: External DBMS

This is the recommended way to use Deckhouse Commander in production environments. To
use Deckhouse Commander, you need to prepare the connection parameters to the database.

### Option 2: operator-postgres module

This is not a recommended method for use in production environments. However, the use of
`operator-postgres` is convenient for quick start with Deckhouse Commander or for environments where there are no
high availability and support requirements.

It is important to enable the Deckhouse Commander module after CRDs from the `operator-postgres` module have appeared in the cluster.

The `operator-postgres` module uses the [PostgreSQL operator](https://github.com/zalando/postgres-operator).
You can use your own [postgres-operator](https://github.com/zalando/postgres-operator) installation version `v1.10.0` or later.

#### Step 1: Enabling operator-postgres

First, you need to enable the postgres operator module and wait for it to be enabled.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-postgres
spec:
  enabled: true
```

#### Step 2: Complete the installation

To make sure that the module is enabled, wait for the Deckhouse task queue to become empty:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
```

Check for the necessary CRDs in the cluster:

```shell
kubectl get crd | grep postgresqls.acid.zalan.do
```

## Enabling Deckhouse Commander

{{< alert level="info" >}}
For a complete list of configuration settings, see [Settings](./configuration.html )
{{< /alert >}}

To enable Deckhouse Commander, create a ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: commander
spec:
  enabled: true
  version: 1
  settings:
    postgres:
      mode: External
      external:
        host: "..."     # Mandatory field
        port: "..."     # Mandatory field
        user: "..."     # Mandatory field
        password: "..." # Mandatory field
        db: "..."       # Mandatory field
```
