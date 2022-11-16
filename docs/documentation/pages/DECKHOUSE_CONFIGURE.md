---
title: "How to configure?"
permalink: en/
---

Deckhouse consists of a Deckhouse operator and modules. A module is a bundle of Helm chart, [Addon-operator](https://github.com/flant/addon-operator/) hooks, building commands for module components (Deckhouse components) and other files.

<div markdown="0" style="height: 0;" id="#deckhouse-configuration"></div>

You can configure Deckhouse using the:
- **[Global settings](deckhouse-configure-global.html)**. Global settings are stored in the `ModuleConfig/global` custom resource. Global settings can be considered as a special `global` module that cannot be disabled.
- **[Modules settings](#configuring-the-module)**. Module settings are stored in the custom resource `ModuleConfig`, whose name matches the module's name (in kebab-case).
- **Custom resources.** Some modules are configured using additional custom resources.

Example of a set of custom resources to configure Deckhouse:

```yaml
# Global setting.
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
# The monitoring-ping module settings.
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

You can view the list of custom resources `ModuleConfig`, the status (enabled/disabled) and the status of the corresponding modules using the command `kubectl get moduleconfigs`:

```shell
$ kubectl get moduleconfigs
NAME                VERSION   AGE   ENABLED              STATUS
deckhouse           1         12h   Enabled              Ready
deckhouse-web       2         12h   Enabled              Ready
global              1         12h   Always On
prometheus          2         12h   Enabled              Ready
upmeter             2         12h   Disabled by config
```

To change the global Deckhouse configuration or a module configuration, create or edit the corresponding `ModuleConfig` custom resource.

For example, to tune the `upmeter` module, run the following command:

```shell
kubectl -n d8-system edit moduleconfig/upmeter
```

Changes are applied automatically after saving the resource.

## Configuring the module

> Deckhouse uses [addon-operator](https://github.com/flant/addon-operator/) when working with modules. Please refer to its documentation to learn how Deckhouse works with [modules](https://github.com/flant/addon-operator/blob/main/MODULES.md), [module hooks](https://github.com/flant/addon-operator/blob/main/HOOKS.md) and [module parameters](https://github.com/flant/addon-operator/blob/main/VALUES.md). We would appreciate it if you *star* the project.

The module is configured using the custom resource `ModuleConfig`, whose name matches the module's name (in kebab-case). Custom resource `ModuleConfig` has the following fields:

- `metadata.name` — the name of the module in kebab-case (e.g, `prometheus`, `node-manager`).
- `spec.version` — a version of the module settings scheme (an integer greater than zero). Mandatory field if `spec.settings` is not empty. The current version number can be seen in the module documentation in the section *"Settings"*.
  - Deckhouse supports backward compatibility of versions of the module settings scheme. If you use an outdated version of the settings schema when editing or viewing the custom resource, a warning about the need to update the module settings schema will be displayed.
- `spec.settings` — module settings. Optional field if the `spec.enabled` field is used. A description of possible settings can be found in the module documentation in the section *"Settings"*.
- `spec.enabled` — optional field to explicitly [enable or disable the module](#enabling-and-disabling-the-module). The module may be enabled by default depending on the [used bundle](#module-bundles) when the parameter is not set.

> Deckhouse operator doesn't modify `ModuleConfig` resources. Using the Infrastructure as Code (IaC) approach, you can store Deckhouse settings in a version control system and use Helm, kubectl, and other familiar tools.

Example of a custom resource for configuring the `kube-dns` module:

```yaml
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  settings:
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

To enable/disable the module, set `spec.enabled` field of the `ModuleConfig` custom resource to `true` or `false`. It may require creating `ModuleConfig` resource for the module.

Here is an example of disabling the `user-authn` module (the module will be turned off despite the module bundles used):

```yaml
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: false
```

To check the status of the module, run the `kubectl get moduleconfig <MODULE_NAME>` command:

Example:

```shell
$ kubectl get moduleconfigs
NAME                VERSION   AGE   ENABLED              STATUS
user-authn          1         12h   Disabled by config
```

## Module bundles

Depending on the [bundle used](./modules/002-deckhouse/configuration.html#parameters-bundle), modules may be enabled or disabled by default.

<table>
<thead>
<tr><th>Bundle name</th><th>List of modules, enabled by default</th></tr></thead>
<tbody>
{% for bundle in site.data.bundles.bundleNames %}
<tr>
<td><strong>{{ bundle }}</strong></td>
<td>
<ul style="columns: 3">
{%- for moduleName in site.data.bundles.bundleModules[bundle] %}
{%- assign isExcluded = site.data.exclude.module_names | where: "name", moduleName %}
{%- if isExcluded.size > 0 %}{% continue %}{% endif %}
<li>
{{ moduleName }}</li>
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
