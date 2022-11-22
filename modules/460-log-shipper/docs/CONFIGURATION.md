---
title: "The log-shipper module: configuration"
---

{% include module-bundle.liquid %}

Module is enabled by default, but agents won't be deployed. It will wait for log-pipeline creation. Log-pipeline consists of [ClusterLoggingConfig](cr.html#clusterloggingconfig)/[PodLoggingConfig](cr.html#podloggingconfig) connected to [ClusterLogDestination](cr.html#clusterlogdestination).

## Parameters

<!-- SCHEMA -->
