---
title: Managed Services subsystem
permalink: en/architecture/managed-services/
search: managed services
description: Architecture of the Managed Services subsystem in Deckhouse Kubernetes Platform.
---

This subsection describes the architecture of the Managed Services subsystem of Deckhouse Kubernetes Platform (DKP). Managed Services subsystem automates the deployment, scaling, backup, and updating of managed services in DKP.

The Managed Services subsystem includes the following modules

* [`managed-cassandra`](/modules/managed-cassandra/): Manages Cassandra clusters.
* [`managed-clickhouse`](/modules/managed-clickhouse/): Manages ClickHouse instances.
* [`managed-hive-metastore`](/modules/managed-hive-metastore/): Manages Hive Metastore clusters.
* [`managed-kafka`](/modules/managed-kafka/): Manages Kafka instances.
* [`managed-memcached`](/modules/managed-memcached/): Manages Memcached instances.
* [`managed-opensearch`](/modules/managed-opensearch): Manages OpenSearch instances.
* [`managed-postgres`](/modules/managed-postgres/): Manages PostgreSQL clusters.
* [`managed-starrocks`](/modules/managed-starrocks/): Manages StarRocks clusters.
* [`managed-trino`](/modules/managed-trino/): Manages Trino clusters.
* [`managed-valkey`](/modules/managed-valkey/): Manages Valkey clusters.
