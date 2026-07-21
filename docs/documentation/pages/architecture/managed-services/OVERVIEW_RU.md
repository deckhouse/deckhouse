---
title: Подсистема Managed Services
permalink: ru/architecture/managed-services/
lang: ru
search: managed services
description: Архитектура подсистемы Managed Services в Deckhouse Kubernetes Platform.
---

В данном подразделе описывается архитектура подсистемы Managed Services в Deckhouse Kubernetes Platform (DKP). Подсистема Managed Services автоматизирует развёртывание, масштабирование, резервное копирование и обновления управляемых сервисов в DKP.

В подсистему Managed Services входят следующие модули:

* [`managed-cassandra`](/modules/managed-cassandra/) — управляет кластерами Cassandra;
* [`managed-clickhouse`](/modules/managed-clickhouse/) — управляет экземплярами ClickHouse;
* [`managed-hive-metastore`](/modules/managed-hive-metastore/) — управляет кластерами Hive Metastore;
* [`managed-kafka`](/modules/managed-kafka/) — управляет инстансами Kafka;
* [`managed-memcached`](/modules/managed-memcached/) — управляет инстансами Memcached;
* [`managed-opensearch`](/modules/managed-opensearch) — управляет инстансами Opensearch;
* [`managed-postgres`](/modules/managed-postgres/) — управляет кластерами PostgreSQL;
* [`managed-trino`](/modules/managed-trino/) — управляет кластерами Trino;
* [`managed-valkey`](/modules/managed-valkey/) — управляет кластерами Valkey.
