---
title: Подсистема Managed Services
permalink: ru/architecture/managed-services/
lang: ru
search: managed services
description: Архитектура подсистемы Managed Services в Deckhouse Platform.
---

В данном подразделе описывается архитектура подсистемы Managed Services в Deckhouse Platform (DP). Подсистема Managed Services автоматизирует развёртывание, масштабирование, резервное копирование и обновление управляемых сервисов в DP.

В подсистему Managed Services входят следующие модули:

* [`managed-cassandra`](/modules/managed-cassandra/) — управляет инстансами Cassandra;
* [`managed-clickhouse`](/modules/managed-clickhouse/) — управляет инстансами ClickHouse;
* [`managed-hive-metastore`](/modules/managed-hive-metastore/) — управляет инстансами Hive Metastore;
* [`managed-kafka`](/modules/managed-kafka/) — управляет инстансами Kafka;
* [`managed-memcached`](/modules/managed-memcached/) — управляет инстансами Memcached;
* [`managed-opensearch`](/modules/managed-opensearch/) — управляет инстансами OpenSearch;
* [`managed-postgres`](/modules/managed-postgres/) — управляет кластерами PostgreSQL;
* [`managed-starrocks`](/modules/managed-starrocks/) — управляет инстансами StarRocks;
* [`managed-trino`](/modules/managed-trino/) — управляет инстансами Trino;
* [`managed-valkey`](/modules/managed-valkey/) — управляет инстансами Valkey.
