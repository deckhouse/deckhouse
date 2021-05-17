---
title: "The monitoring-kubernetes module: configuration"
---

This module is **enabled** by default.

## Parameters

* `highAvailability` — manually enable/disable the high availability mode. By default, the high availability mode is set automatically. Click [here](../../deckhouse-configure-global.html#parameters) to learn more about the HA mode for modules.
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any nodeSelector.
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any tolerations.
