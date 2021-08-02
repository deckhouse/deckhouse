---
title: "The log-shipper module"
---

Module is created for creating log-pipeline on nodes with Custom Resources.

You can store logs using log-pipeline to Loki/Elasticsearch/Logstash storages.

Module is enabled by default, but agents won't be deployed. It will wait for log-pipeline creation. Log-pipeline consists of [ClusterLoggingConfig](cr.html#clusterloggingconfig)/[PodLoggingConfig](cr.html#podloggingconfig) connected to [ClusterLogDestination](cr.html#clusterlogdestination).
