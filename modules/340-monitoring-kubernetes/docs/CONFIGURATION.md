---
title: "The monitoring-kubernetes module: configuration"
---

This module is **enabled** by default.

## Parameters

* `highAvailability` — manually enable/disable the high availability mode. By default, the high availability mode is set automatically. Click [here](../../deckhouse-configure-global.html#parameters) to learn more about the HA mode for modules.
* `nodeSelector` — the same as in the Pod's `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted of `false`, it will be determined [automatically](../../#advanced-scheduling).
* `tolerations` — the same as in the Pod's `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted of `false`, it will be determined [automatically](../../#advanced-scheduling).
