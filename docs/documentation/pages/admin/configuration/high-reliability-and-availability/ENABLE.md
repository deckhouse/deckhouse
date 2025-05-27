---
title: Managing HA mode
permalink: en/admin/configuration/high-reliability-and-availability/enable.html
description: Managing HA mode
lang: en
---

{% alert level="info" %}
Note that if the cluster has **more than one master node**, HA mode is **enabled automatically**.
This applies both when deploying a cluster with multiple master nodes from the start
and when increasing the number of master nodes from one to three.
{% endalert %}

To enable HA mode globally for DKP,
set the `settings.highAvailability` parameter to `true` in `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings: 
    highAvailability: true
...
```

To ensure HA mode is enabled,
you can, for example, check the number of `deckhouse` Pods in the `d8-system` namespace.
To do that, run the following command:

```shell
sudo -i d8 k -n d8-system get po | grep deckhouse
```

The number of `deckhouse` Pods in the output must be more than one:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```

<!--
- If the [`console`](/products/kubernetes-platform/modules/console/stable/) module is enabled in the cluster,
  open the Deckhouse web UI, navigate to the **Deckhouse** — **Global settings** — **Global module settings** section,
  and switch the **Fault tolerance mode** toggle to **Yes**.
-->

## Enabling HA mode for individual components

Some DKP modules may have their own HA mode settings.
To enable HA mode in a specific module, set the `settings.highAvailability` parameter in its configuration.
The HA mode operation in individual modules is independent of the global HA mode.

List of modules supporting individual HA mode:

- `deckhouse`
- `openvpn`
- `istio`
- `dashboard`
- `multitenancy-manager`
- `user-authn`
- `ingress-nginx`
- `prometheus-monitoring`
- `monitoring-kubernetes`
- `snapshot-controller`

For example, to enable HA mode manually for the `deckhouse` module,
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
...
```

To ensure HA mode is enabled, check the number of Pods for the target module.
For example, to verify the mode operation for the `deckhouse` module,
check the number of corresponding Pods in the `d8-system` namespace by running the following command:

```shell
sudo -i d8 k -n d8-system get po | grep deckhouse
```

The number of `deckhouse` Pods in the output must be more than one:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```
