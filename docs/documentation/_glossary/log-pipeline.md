---
title: Log pipeline
description: The chain of log collection, filtering and delivery that the [`log-shipper`](/modules/log-shipper/) module builds from [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) and [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination).
lang: en
---

ClusterLoggingConfig describes log sources at the cluster level, PodLoggingConfig describes sources within one namespace, and ClusterLogDestination defines the parameters of the log storage. Along the pipeline, logs can be filtered by labels and message content, enriched with Kubernetes metadata, [buffered](glossary.html#log-buffering) and sent to several storages at once, for example to the [short-term log storage](glossary.html#short-term-log-storage) and to Elasticsearch.

For details, see [Log collection and delivery](admin/configuration/logging/delivery.html).
