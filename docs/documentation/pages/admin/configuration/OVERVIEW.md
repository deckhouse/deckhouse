---
title: "How to configure?"
permalink: en/admin/configuration/
description: "Learn how to configure Deckhouse Kubernetes Platform using global settings, module configurations, and custom resources."
---

## Deckhouse configuration

You can configure Deckhouse using:

- **[Global settings](../../reference/api/global.html)**. Global settings are stored in the `ModuleConfig/global` custom resource. Global settings can be be thought of as a special `global` module that cannot be disabled.
- **[Module settings](#configuring-the-module)**. Module settings are stored in the `ModuleConfig` custom resource; its name is the same as that of the module (in kebab-case).
- **Custom resources.** Some modules are configured using the additional custom resources.

An example of a set of custom resources for configuring Deckhouse:

```yaml
# Global settings.
apiVersion: deckhouse.io/v1alpha1
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
apiVersion: deckhouse.io/v1alpha1
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
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  enabled: false
```

You can view the list of `ModuleConfig` custom resources and the states of the corresponding modules (enabled/disabled) as well as their statuses using the following command:

```shell
d8 k get moduleconfigs
```

{% offtopic title="Example output..." %}

```console
$ d8 k get moduleconfigs
NAME            ENABLED   VERSION   AGE     MESSAGE
deckhouse       true      1         12h
documentation   true      1         12h
global                    1         12h
prometheus      true      2         12h
upmeter         false     2         12h
```

{% endofftopic %}

To change the global Deckhouse configuration or module configuration, create or edit the corresponding `ModuleConfig` custom resource.

For example, this command allows you to configure the [`upmeter`](/modules/upmeter/) module:

```shell
d8 k edit moduleconfig/upmeter
```

Changes are applied automatically once the resource configuration is saved.

### Modifying cluster configuration

{% alert level="warning" %}
To apply changes related to node configuration, you must run the `dhctl converge` command using the DKP installer.  
This command synchronizes the actual node state with the specified configuration.
{% endalert %}

General cluster parameters are defined in the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) structure.

To modify these parameters, run the following command:

```shell
d8 platform edit cluster-configuration
```

After saving the changes, DKP will automatically reconcile the cluster state with the new configuration.
Depending on the cluster size, this process may take some time.

### Viewing current configuration

DKP is managed through global settings, module configurations, and various custom resources.

1. To view global settings, run:

   ```shell
   d8 k get mc global -o yaml
   ```

1. To view the status of all modules (available in Deckhouse version 1.47+):

   ```shell
   d8 k get modules
   ```

1. To view the configuration of the [`user-authn`](/modules/user-authn/) module:

   ```shell
   d8 k get moduleconfigs user-authn -o yaml
   ```

## Configuring the module

The module is configured using the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) custom resource , whose name is the same as the module name (in kebab-case). The ModuleConfig custom resource has the following fields:

- `metadata.name` — the name of the module in kebab-case (e.g, `prometheus`, `node-manager`).
- `spec.version` — version of the module settings schema. It is an integer greater than zero. This field is mandatory if `spec.settings` is not empty. You can find the latest version number in the module's documentation under *Settings*.
  - Deckhouse is backward-compatible with older versions of the module's settings schema. If an outdated version of the schema is used, a warning stating that you need to update the module's schema will be displayed when editing or viewing the custom resource.
- `spec.settings` — module settings. This field is optional if the `spec.enabled` field is used. For a description of the available settings, see *Settings* in the module's documentation.
- `spec.enabled` — this optional field allows you to explicitly [enable or disable the module](#enabling-and-disabling-the-module). The module may be enabled by default based on the [bundle in use](#module-bundles) if this parameter is not set.

> Deckhouse doesn't modify ModuleConfig resources. As part of the Infrastructure as Code (IaC) approach, you can store ModuleConfigs in a version control system and use Helm, `d8 k`, and other familiar tools for deploy.

An example of a custom resource for configuring the [`kube-dns`](/modules/kube-dns/) module:

```yaml
apiVersion: deckhouse.io/v1alpha1
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

To enable/disable the module, set [`spec.enabled`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig-v1alpha1-spec-enabled) field of the ModuleConfig custom resource to `true` or `false`. Note that this may require you to first create a ModuleConfig resource for the module.

Here is an example of disabling the [`user-authn`](/modules/user-authn/) module (the module will be turned off even if it is enabled as part of a module bundle):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: false
```

To check the status of the module, run the `d8 k get moduleconfig <MODULE_NAME>` command:

Example:

```shell
$ d8 k get moduleconfig user-authn
NAME         ENABLED   VERSION   AGE   MESSAGE
user-authn   false     1         12h
```

## Module bundles

Depending on the [bundle used](/modules/deckhouse/configuration.html#parameters-bundle), modules may be enabled or disabled by default.

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
{%- if site.data.excludedModules contains moduleName %}{% continue %}{% endif %}
<li>{{ moduleName }}</li>
{%- endfor %}
</ul>
</td>
</tr>
{%- endfor %}
</tbody>
</table>

### Things to keep in mind when working with the Minimal module set

{% alert level="warning" %}
**Note** that several basic modules are not included in the `Minimal` set of modules (for example, the CNI module).

Deckhouse with the `Minimal` module set and no basic modules included will only be able to operate in an already deployed cluster.
{% endalert %}

To install Deckhouse with the `Minimal` module set, enable at least the following modules by specifying them in the installer configuration file:

* cloud provider module (for example, [`cloud-provider-aws`](/modules/cloud-provider-aws/) for AWS), in a case of deploying a cloud cluster;
* [`cni-cilium`](/modules/cni-cilium/) or another CNI control module (if necessary);
* [`control-plane-manager`](/modules/control-plane-manager/);
* [`kube-dns`](/modules/kube-dns/);
* [`node-manager`](/modules/node-manager/);
* `registry-packages-proxy`;
* [`terraform-manager`](/modules/terraform-manager/), in a case of deploying a cloud cluster.

### Accessing documentation for the current version

The documentation for the running version of Deckhouse is available at `documentation.<cluster_domain>`,  
where `<cluster_domain>` is the DNS name generated according to the template specified in the [`modules.publicDomainTemplate`](../../reference/api/global.html#parameters-modules-publicdomaintemplate) parameter of the global configuration.

{% alert level="warning" %}
Documentation is available only if the [documentation](/modules/documentation/) module is enabled in the cluster.  
It is enabled by default, except when using the [`Minimal` delivery bundle](/modules/deckhouse/configuration.html#parameters-bundle).
{% endalert %}

## Managing placement of Deckhouse components

### Advanced scheduling

If no `nodeSelector/tolerations` are explicitly specified in the module parameters, the following strategy is used for all modules:
1. If the `nodeSelector` module parameter is not set, then Deckhouse will try to calculate the `nodeSelector` automatically. Deckhouse looks for nodes with the specific labels in the cluster  (see [the list](#module-features-that-depend-on-its-type) below). If there are any, then the corresponding `nodeSelectors` are automatically applied to module resources.
1. If the `tolerations` parameter is not set for the module, all the possible tolerations are automatically applied to the module's Pods (see the list below).
1. You can set both parameters to `false` to disable their automatic calculation.
1. If there are no nodes with a [specific role](#module-features-that-depend-on-its-type) in the cluster and `nodeSelector` is automatically selected (see point 1), `nodeSelector` will not be specified in the module resources. The module will then use any node with non-conflicting `taints`.

You cannot set `nodeSelector` and `tolerations` for modules:

- running on all cluster nodes (such as [`cni-flannel`](/modules/cni-flannel/) or [`monitoring-ping`](/modules/monitoring-ping/));
- running on all master nodes (such as [`prometheus-metrics-adapter`](/modules/prometheus-metrics-adapter/) or [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/)).

### Module features that depend on its type

{% raw %}
* The *monitoring*-related modules ([`operator-prometheus`](/modules/operator-prometheus/), [`prometheus`](/modules/prometheus/) and [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/)):
  * Deckhouse examines nodes to determine a [`nodeSelector`](/modules/prometheus/configuration.html#parameters-nodeselector) in the following order:
    1. It checks if a node with the `node-role.deckhouse.io/MODULE_NAME` label is present in the cluster.
    1. It checks if a node with the `node-role.deckhouse.io/monitoring` label is present in the cluster.
    1. It checks if a node with the `node-role.deckhouse.io/system` label is present in the cluster.
  * Tolerations to add (note that tolerations are added all at once):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (e.g., `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}`).
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}`.
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
* The *frontend*-related modules ([`ingress-nginx`](/modules/ingress-nginx/) only):
  * Deckhouse examines nodes to determine a `nodeSelector` in the following order:
    1. It checks if a node with the `node-role.deckhouse.io/MODULE_NAME` label is present in the cluster.
    1. It checks if a node with the `node-role.deckhouse.io/frontend` label is present in the cluster.
  * Tolerations to add (note that tolerations are added all at once):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}`.
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}`.
* Other modules:
  * Deckhouse examines nodes to determine a `nodeSelector` in the following order:
    1. It checks if a node with the `node-role.deckhouse.io/MODULE_NAME` label is present in the cluster (e.g., `node-role.deckhouse.io/cert-manager`).
    1. It checks if a node with the `node-role.deckhouse.io/system` label is present in the cluster.
  * Tolerations to add (note that tolerations are added all at once):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (e.g., `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}`).
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
{% endraw %}
