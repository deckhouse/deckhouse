---
title: "How to configure?"
permalink: en/
---

Deckhouse consists of a Deckhouse operator and modules. A module is a set of helm charts, hooks, files, and assembly rules for module components (Deckhouse components).

You can configure Deckhouse using the:
- [Global settings](deckhouse-configure-global.html#parameters) stored in the `global` parameters of the [Deckhouse configuration](#deckhouse-configuration).
- Module settings stored in [Deckhouse configuration](#deckhouse-configuration) and custom resources (for some Deckhouse modules).

## Deckhouse configuration

The Deckhouse configuration is stored in the `deckhouse` ConfigMap in the `d8-system` namespace and may contain the following parameters (keys):

- `global` —  contains the [global Deckhouse settings](deckhouse-configure-global.html) as a multi-line string in YAML format;
- `<moduleName>` (where `<moduleName>` is the name of the Deckhouse module in camelCase) — contains the [module settings](#configuring-the-module) as a multi-line string in YAML format;
- `<moduleName>Enabled` (where `<moduleName>` is the name of the Deckhouse module in camelCase) — this one explicitly [enables or disables the module](#enabling-and-disabling-the-module).

Use the following command to view the `deckhouse` ConfigMap:

```shell
kubectl -n d8-system get cm/deckhouse -o yaml
```

Example of the `deckhouse` ConfigMap:
```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # Note the vertical bar.
    # Section of the YAML file with global settings.
    modules:
      publicDomainTemplate: "%s.kube.company.my"
  # monitoring-ping related section of the YAML file.
  monitoringPing: |
    externalTargets:
    - host: 8.8.8.8
  # Disabling the dashboard module.
  dashboardEnabled: "false"
```

Pay attention to the following:
- The `|` sign — vertical bar glyph that must be specified when passing settings, because the parameter being passed is a multi-line string, not an object.
- A module name is in *camelCase* style.

Use the following command to edit the `deckhouse` ConfigMap:

```shell
kubectl -n d8-system edit cm/deckhouse
```

### Configuring the module

> Deckhouse uses [addon-operator](https://github.com/flant/addon-operator/) when working with modules. Please refer to its documentation to learn how Deckhouse works with [modules](https://github.com/flant/addon-operator/blob/main/MODULES.md), [module hooks](https://github.com/flant/addon-operator/blob/main/HOOKS.md) and [module parameters](https://github.com/flant/addon-operator/blob/main/VALUES.md). We would appreciate it if you *star* the project.

Deckhouse only works with the enabled modules. Modules can be enabled or disabled by default, depending on the [bundle used](#module-bundles). Learn more on how to explicitly [enable and disable the module](#enabling-and-disabling-the-module).

You can configure the module using the parameter with the module name in camelCase in the Deckhouse configuration. The parameter value is a multi-line YAML string with the module settings.

Some modules can also be configured using custom resources. Use the search bar at the top of the page or select a module in the left menu to see a detailed description of its settings and the custom resources used.

Below is an example of the `kube-dns` module settings:
```yaml
data:
  kubeDns: |
    stubZones:
    - upstreamNameservers:
      - 192.168.121.55
      - 10.2.7.80
      zone: directory.company.my
    upstreamNameservers:
    - 10.2.100.55
    - 10.2.200.55
```

### Enabling and disabling the module

> Depending on the [bundle used](#module-bundles), some modules may be enabled by default.

To enable/disable a module, add the `<moduleName>Enabled` parameter to the `deckhouse` ConfigMap with one of the following two values: `"true"` or `"false"` (note: quotation marks are mandatory), where `<moduleName>` is the name of the module in camelCase.

Here is an example of enabling the `user-authn` module:
```yaml
data:
  userAuthnEnabled: "true"
```

## Module bundles

Depending on the [bundle used](./modules/020-deckhouse/configuration.html#parameters-bundle), modules may be enabled or disabled by default.

{%- assign bundles = site.data.bundles | sort %}
<table>
<thead>
<tr><th>Bundle name</th><th>List of modules, enabled by default</th></tr></thead>
<tbody>
{% for bundle in bundles %}
<tr>
<td><strong>{{ bundle[0] |  replace_first: "values-", "" | capitalize }}</strong></td>
<td>{% assign modules = bundle[1] | sort %}
<ul style="columns: 3">
{%- for module in modules %}
{%- assign moduleName = module[0] | regex_replace: "Enabled$", '' | camel_to_snake_case | replace: "_", '-' %}
{%- assign isExcluded = site.data.exclude.module_names | where: "name", moduleName %}
{%- if isExcluded.size > 0 %}{% continue %}{% endif %}
{%- if module[1] != true %}{% continue %}{% endif %}
<li>
{{ module[0] | regex_replace: "Enabled$", '' | camel_to_snake_case | replace: "_", '-' }}</li>
{%- endfor %}
</ul>
</td>
</tr>
{%- endfor %}
</tbody>
</table>

## Advanced scheduling

If no `nodeSelector/tolerations` are explicitly specified in the module parameters, the following strategy is used for all modules:
1. If the `nodeSelector` module parameter is not set, then Deckhouse will try to calculate the `nodeSelector` automatically. Deckhouse looks for nodes with the specific labels in the cluster  (see the list below). If there are any, then the corresponding `nodeSelectors` are automatically applied to module resources.
1. If the `tolerations` parameter is not set for the module, all the possible tolerations are automatically applied to the module's Pods (see the list below).
1. You can set both parameters to `false` to disable their automatic calculation.

You cannot set `nodeSelector` and `tolerations` for modules:
- that involve running a DaemonSet on all cluster nodes (e.g., `cni-flannel`, `monitoring-ping`);
- designed to run on master nodes (e.g., `prometheus-metrics-adapter` or some `vertical-pod-autoscaler` components).

### Module features that depend on its type
{% raw %}
* The *monitoring*-related modules (operator-prometheus, prometheus and vertical-pod-autoscaler):
  * Deckhouse examines nodes to determine a nodeSelector in the following order:
    * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster.
    * It checks if a node with the <code>node-role.deckhouse.io/monitoring</code> label is present in the cluster.
    * It checks if a node with the <code>node-role.deckhouse.io/system</code> label is present in the cluster.
  * Tolerations to add (note that tolerations are added all at once):
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>

      E.g., <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}</code>.
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}</code>.
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>.
* The *frontend*-related modules (nginx-ingress only):
    * Deckhouse examines nodes to determine a nodeSelector in the following order:
        * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster.
        * It checks if a node with the <code>node-role.deckhouse.io/frontend</code> label is present in the cluster.
    * Tolerations to add (note that tolerations are added all at once):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>.
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}</code>.
* Other modules:
    * Deckhouse examines nodes to determine a nodeSelector in the following order:
        * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster;

          E.g., <code>node-role.deckhouse.io/cert-manager</code>);
        * It checks if a node with the <code>node-role.deckhouse.io/system</code> label is present in the cluster.
    * Tolerations to add (note that tolerations are added all at once):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>

          E.g., <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}</code>;
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>.
{% endraw %}
