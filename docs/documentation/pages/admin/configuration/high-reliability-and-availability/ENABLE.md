---
title: Managing HA mode
permalink: en/admin/configuration/high-reliability-and-availability/enable.html
description: Managing HA mode
---

{% alert level="info" %}
Note that if the cluster has **more than one master node**, HA mode is **enabled automatically**.
This applies both when deploying a cluster with multiple master nodes from the start
and when increasing the number of master nodes from one to three.
{% endalert %}

## Enabling HA mode globally

You can enable HA mode globally for DKP in one of the following ways.

### Using ModuleConfig/global custom resource

1. Set the [`settings.highAvailability`](../../../reference/api/global.html#parameters-highavailability) parameter to `true` in `ModuleConfig/global`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: global
   spec:
     version: 2
     settings: 
       highAvailability: true
   ```

1. To ensure HA mode is enabled,
   you can, for example, check the number of `deckhouse` Pods in the `d8-system` namespace.
   To do that, run the following command:

   ```shell
   d8 k -n d8-system get po | grep deckhouse
   ```

   The number of `deckhouse` Pods in the output must be more than one:

   ```text
   deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
   deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
   deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
   ```

### Using Deckhouse web UI

If the [`console`](/modules/console/) module is enabled in the cluster,
open the Deckhouse web UI, navigate to **Deckhouse** — **Global settings** — **Global module settings**,
and switch the **HA mode** toggle to **Yes**.

## Enabling HA mode for individual components

Some DKP modules may have their own HA mode settings.
To enable HA mode in a specific module, set the `settings.highAvailability` parameter in its configuration.
The HA mode operation in individual modules is independent of the global HA mode.

List of modules supporting individual HA mode:

- [`deckhouse`](/modules/deckhouse/)
- [`openvpn`](/modules/openvpn/)
- [`istio`](/modules/istio/)
- [`dashboard`](/modules/dashboard/)
- [`multitenancy-manager`](/modules/multitenancy-manager/)
- [`user-authn`](/modules/user-authn/)
- [`ingress-nginx`](/modules/ingress-nginx/)
- [`prometheus-monitoring`](/modules/prometheus/)
- [`monitoring-kubernetes`](/modules/monitoring-kubernetes/)
- [`snapshot-controller`](/modules/snapshot-controller/)

To enable HA mode manually for a specific module,
add the `settings.highAvailability` parameter to its configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    highAvailability: true
```

To ensure HA mode is enabled, check the number of Pods for the target module.
For example, to verify the mode operation for the `deckhouse` module,
check the number of corresponding Pods in the `d8-system` namespace by running the following command:

```shell
d8 k -n d8-system get po | grep deckhouse
```

The number of `deckhouse` Pods in the output must be more than one:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```
