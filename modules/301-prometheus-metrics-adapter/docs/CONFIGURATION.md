---
title: "The prometheus-metrics-adapter module: configuration"
search: autoscaler, HorizontalPodAutoscaler 
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

The module works if the `prometheus` module is enabled. Generally, no configuration is required.

## Parameters

* `highAvailability` — manually enable/disable the high availability mode. By default, this parameter is configured automatically (additional information about the HA mode for modules is available [here](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters)).
