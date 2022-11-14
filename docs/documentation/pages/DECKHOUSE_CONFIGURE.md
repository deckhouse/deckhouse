---
title: "How to configure?"
permalink: en/
---

Deckhouse consists of a Deckhouse operator and modules. A module is a bundle of Helm chart, Addon-operator hooks, other files, and building commands for module components (Deckhouse components).

You can configure Deckhouse using the:
- [Global settings](deckhouse-configure-global.html#parameters) are stored in the `ModuleConfig/global` resource.
- Module settings are stored in ModuleConfig resources and some modules have additional custom resources.

## Deckhouse configuration

The Deckhouse configuration is stored in `ModuleConfig` resources and may contain the following parameters:

- `metadata.name` — the name of the resource is the name of the Deckhouse module (kebab-cased).
- `spec.version` — a version of module settings.
- `spec.settings` — an object with module settings.
- `spec.enabled` — optional boolean value to explicitly [enable or disable the module](#enabling-and-disabling-the-module). The module may be enabled by default depending on the [used bundle](#module-bundles) when the parameter is not set.

If `spec.settings` is not empty, `spec.version` is required. The latest version number is available in the description of module settings.

Settings version may become obsolete with new releases. The Deckhouse will support previous versions to allow managing ModuleConfig resources using IaC. Also, it will warn about necessity to update `spec.settings` and `spec.version` when resource is changed or viewed.

Resource `ModuleConfig/global` stores global settings. "global" can't be disabled, so the value of `spec.enabled` is ignored.

Example of `ModuleConfig` resources:

```yaml
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.kube.company.my"
---
# monitoring-ping settings.
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  settings:
    externalTargets:
    - host: 8.8.8.8
---
# Disable the dashboard module.
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  enabled: false
```

`status` field contains module state, you can get module state after applying changes:

```shell
kubectl get moduleconfigs
NAME                VERSION   AGE   ENABLED              STATUS
deckhouse           1         12h   Enabled              Ready
deckhouse-web       2         12h   Enabled              Ready
global              1         12h   Always On
prometheus          2         12h   Enabled              Ready
upmeter             2         12h   Disabled by config
```

To change Deckhouse configuration, create or edit ModuleConfig resource related to the module. For example, to tune `upmeter` module, use this command:

```shell
kubectl -n d8-system edit moduleconfig/upmeter
```

Changes are applied automatically after saving the resource.

Deckhouse operator doesn't modify ModuleConfig resources, so you can use kubectl, Helm, Git and other IaC utilities to manage Deckhouse configuration.

### Configuring the module

> Deckhouse uses [addon-operator](https://github.com/flant/addon-operator/) when working with modules. Please refer to its documentation to learn how Deckhouse works with [modules](https://github.com/flant/addon-operator/blob/main/MODULES.md), [module hooks](https://github.com/flant/addon-operator/blob/main/HOOKS.md) and [module parameters](https://github.com/flant/addon-operator/blob/main/VALUES.md). We would appreciate it if you *star* the project.

Deckhouse only works with the enabled modules. Modules can be enabled or disabled by default, depending on the [bundle used](#module-bundles). Learn more on how to explicitly [enable and disable the module](#enabling-and-disabling-the-module).

You can configure the module using the ModuleConfig resource named as module in kebab-case.

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

Some modules can also be configured using custom resources. Use the search bar at the top of the page or select a module in the left menu to see a detailed description of its settings and the custom resources used.

### Enabling and disabling the module

> Depending on the [bundle used](#module-bundles), some modules may be enabled by default.

To enable/disable the module, set `spec.enabled` field of ModuleConfig resource to `true` or `false`. It may require creating ModuleConfig resource for the module.

Here is an example of disabling the `user-authn` module, enabled by default:

```yaml
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: false
```

```shell
kubectl get moduleconfigs
NAME                VERSION   AGE   ENABLED              STATUS
user-authn          1         12h   Disabled by config
```

## Module bundles

Depending on the [bundle used](./modules/002-deckhouse/configuration.html#parameters-bundle), modules may be enabled or disabled by default.

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

## Managing placement of Deckhouse components

### Advanced scheduling

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
