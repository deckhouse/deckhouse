---
title: "How to configure?"
permalink: en/
---

The configuration data of *Deckhouse* and its modules are stored in one place — in the `deckhouse` ConfigMap resource in the `d8-system` namespace. Some modules (in addition to the CM configuration) are also configured using dedicated custom resources in the cluster. Information about the module's parameters and the custom resources used by the module is available in the description of the module or subsystem features.

The `deckhouse` config (the `deckhouse` ConfigMap resource) has a [global section](deckhouse-configure-global.html) and a module section.

Use the following command to edit the `deckhouse` ConfigMap:
```
kubectl -n d8-system edit cm/deckhouse
```

## Example of the `deckhouse` ConfigMap

Pay attention to the `|` - vertical bar glyph.

```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # <--- note the vertical bar!!!
    # Section of the YAML file with global settings
    modules:
      publicDomainTemplate: "%s.kube.company.my"
  nginxIngress: |
    # nginx-ingress-related section of the YAML file
    config:
      hsts: true
  someModuleName: |  # <--- the module name in the camelCase
    foo: bar
  dashboardEnabled: "false"   # <--- this is how you can disable the module
```

## Enabling and disabling the module

Deckhouse only installs the [modules](https://github.com/flant/addon-operator/blob/master/MODULES.md) that are enabled. [Read more](https://github.com/flant/addon-operator/blob/master/LIFECYCLE.md#modules-discovery) about the algorithm for determining if the module is enabled.

Modules can be enabled or disabled by default, depending on the [bundle used](./modules/020-deckhouse/configuration.html).

To enable/disable the module, add the `<moduleName>Enabled` parameter to the `deckhouse` ConfigMap and set it to `"true"` or `"false"` (here, `<moduleName>` is the name of the module in camelCase).

Here is an example of enabling the user-authn module:
```yaml
data:
  userAuthnEnabled: "true"
```

## Advanced scheduling

The following general strategy is used for making scheduling decisions:
1. If the `nodeSelector` module parameter is not set, Deckhouse looks for nodes with the specific labels in the cluster. If there are any, then the corresponding nodeSelectors are automatically applied. Below you may find the list of specific labels and the description of the discovery process.
1. If the `tolerations` parameter is not set for the module, all the possible tolerations are automatically applied to the module's pods (see the list below).
1. You can set both parameters to `false` to disable their automatic calculation..

**Caution!** Note that you cannot set `nodeSelector` and `tolerations` for modules that involve running a DaemonSet on all cluster nodes (e.g., `ping-exporter` and `node-problem-detector`) or modules designed to run on master nodes (e.g., `prometheus-metrics-adapter` or some `vertical-pod-autoscaler` components).

{% offtopic title="The nuances of the automatic calculation related to the 'type' of the module" %}{% raw %}
* The *monitoring*-related modules (operator-prometheus, prometheus и vertical-pod-autoscaler):
  * Deckhouse examines nodes to determine a nodeSelector in the following order:
    * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster
    * It checks if a node with the <code>node-role.deckhouse.io/monitoring</code> label is present in the cluster
    * It checks if a node with the <code>node-role.deckhouse.io/system</code> label is present in the cluster
  * Tolerations to add (note that tolerations are added all at once):
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>

      (e.g., <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}</code>)
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}</code>
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>
* The *frontend*-related modules (nginx-ingress only)
    * Deckhouse examines nodes to determine a nodeSelector in the following order:
        * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster
        * It checks if a node with the <code>node-role.deckhouse.io/frontend</code> label is present in the cluster
    * Tolerations to add (note that tolerations are added all at once):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}</code>
* Other modules
    * Deckhouse examines nodes to determine a nodeSelector in the following order:
        * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> (e.g., <code>node-role.deckhouse.io/cert-manager</code>) label is present in the cluster
        * It checks if a node with the <code>node-role.deckhouse.io/system</code> label is present in the cluster
    * Tolerations to add (note that tolerations are added all at once):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code> (e.g., <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}</code>)
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>
{% endraw %}
{% endofftopic %}
