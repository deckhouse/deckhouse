---
title: "The prometheus-metrics-adapter module: configuration"
search: autoscaler, HorizontalPodAutoscaler 
---

By default, the module is **enabled** in clusters starting with version 1.9 if the `prometheus` module is enabled. Generally, no configuration is required.

## Parameters

* `highAvailability` — manually enable/disable the high availability mode. By default, this parameter is configured automatically (additional information about the HA mode for modules is available [here](../../deckhouse-configure-global.html#parameters)).
