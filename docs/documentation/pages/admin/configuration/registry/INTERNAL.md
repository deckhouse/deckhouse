---
title: Managing the internal container image registry
permalink: en/admin/configuration/registry/internal.html
description: "Configure internal container image registry in Deckhouse Kubernetes Platform. Image caching, storage optimization, and high availability registry management."
---

The ability to use internal registry is implemented by the [`registry`](/modules/registry/) module.

The internal registry allows for optimizing the downloading and storage of images, as well as helping to ensure availability and fault tolerance for Deckhouse Kubernetes Platform.

## Modes of operation with the internal registry

The [`registry`](/modules/registry/) module, which implements internal storage, operates in the following modes:

- `Direct` — enables the internal container image registry. Access to the internal registry is performed via the fixed address `registry.d8-system.svc:5001/system/deckhouse`. This fixed address allows Deckhouse images to avoid being re-downloaded and components to avoid being restarted when registry parameters change. Switching between modes and registries is done through the [`deckhouse`](/modules/deckhouse/configuration.html) ModuleConfig. The switching process is automatic (for more details, see the switching examples below) for more information. The architecture of the mode is described in the section [Direct Mode Architecture](../../../architecture/registry-direct-mode.html).

- `Unmanaged` — operation without using an internal registry. Access within the cluster is performed via an address that can be [set during the cluster installation](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo) or [changed in a deployed cluster](../registry/third-party.html).

{% alert level="info" %}
- The `Direct` mode requires using the `Containerd` or `Containerd V2` CRI on all cluster nodes. For CRI setup, refer to the [`ClusterConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration).
{% endalert %}

## Restrictions on working with the internal registry

Working with the internal registry using the [`registry`](/modules/registry/) module has a number of limitations and restrictions concerning installation, operating conditions, and mode switching.

### Cluster installation limitations

Bootstrapping a DKP cluster with `Direct` mode enabled is not supported. The cluster is deployed with settings for `Unmanaged` mode.

### Operating conditions restrictions

The [`registry`](/modules/registry/) module works under the following conditions:

- If CRI containerd or containerd v2 is used on the cluster nodes. To configure CRI, refer to the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) configuration.
- The cluster is fully managed by DKP. The module will not work in Managed Kubernetes clusters.

### Mode switching restrictions

Mode switching restrictions are as follows:

- Switching to `Direct` mode is possible if there are no [user registry configurations](/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry) on the nodes.
- Switching to `Unmanaged` mode is only available from `Direct` mode.
- In `Unmanaged` mode, changing registry settings is not supported. To change settings, you need to switch to `Direct` mode, make the necessary changes, and then switch back to `Unmanaged` mode.

## Examples of switching

### Switching to Direct Mode

To switch an already running cluster to `Direct` mode, follow these steps:

{% alert level="danger" %}

- During the first switch, the containerd v1 service will be restarted, as the switch to the [new authorization configuration](#preparation-of-containerd-v1) will take place.
- When changing the registry mode or registry parameters, Deckhouse will be restarted.

{% endalert %}

1. If the cluster is running with containerd v1, [you need to prepare custom containerd configuration](#preparation-of-containerd-v1).

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Make sure the [`registry`](/modules/registry/) module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Set the `Direct` mode configuration in the [`deckhouse`](/modules/deckhouse/configuration.html) ModuleConfig. If you're using a registry other than `registry.deckhouse.io`, refer to the [deckhouse](/modules/deckhouse/) module documentation for correct configuration.

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status). Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Direct
   target_mode: Direct
   ```

### Switching to Unmanaged Mode

{% alert level="danger" %}
When changing the registry mode or registry parameters, Deckhouse will be restarted.
{% endalert %}

{% alert level="warning" %}
Switching to the `Unmanaged` mode is only available from `Direct` mode. Registry configuration parameters will be taken from the previously active mode.
{% endalert %}

To switch the cluster to `Unmanaged` mode, follow these steps:

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Ensure that the [`registry`](/modules/registry/) module is running in `Direct` mode and the switch status to `Direct` is `Ready`. You can verify the state via the `registry-state` secret using [this guide](#check-registry-mode-switch-status). Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Direct
   target_mode: Direct
   ```

1. Set the `Unmanaged` mode in the [`deckhouse`](/modules/deckhouse/configuration.html) ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status). Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. If you need to switch back to the previous containerd v1 auth configuration, refer to the [instruction](#switch-to-the-previous-containerd-v1-authorization-configuration).

## Preparation of containerd v1

When switching to the `Direct` mode, the `Containerd V1` service will be restarted.  
The authorization configuration will be switched to Mirror Auth (this configuration is used by default in containerd v2).  
After switching back to `Unmanaged`, the updated authorization configuration will remain unchanged.

Example directory structure for Mirror Auth configuration:

```bash
tree /etc/containerd/registry.d
.
├── registry.d8-system.svc:5001
│   ├── ca.crt
│   └── hosts.toml
└── registry.deckhouse.ru
    ├── ca.crt
    └── hosts.toml
```

Example hosts.toml configuration:

```toml
[host]
  [host."https://registry.deckhouse.ru"]
    capabilities = ["pull", "resolve"]
    skip_verify = true
    ca = ["/path/to/ca.crt"]
    [host."https://registry.deckhouse.ru".auth]
      username = "username"
      password = "password"
      # If providing auth string:
      auth = "<base64>"
```

Before switching, make sure there are no [custom authorization configurations](/modules/node-manager/faq.html#how-to-add-additional-registry-auth) present on nodes with containerd V1 in the `/etc/containerd/conf.d` directory.

If such configurations exist:

{% alert level="danger" %}
- After deleting [custom authorization configurations](/modules/node-manager/faq.html#how-to-add-additional-registry-auth) from the `/etc/containerd/conf.d` directory, the containerd service will be restarted. The removed configurations will no longer work.
- New Mirror Auth configurations added to `/etc/containerd/registry.d` will only take effect after switching to `Direct` mode.
{% endalert %}

1. Create new Mirror Auth configurations in the `/etc/containerd/registry.d` directory. Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: custom-registry
   spec:
     bundles:
       - '*'
     content: |
       #!/bin/bash
       REGISTRY_ADDRESS="registry.io"
       REGISTRY_SCHEME="https"
       host_toml=$(cat <<EOF
       [host]
         [host."https://registry.deckhouse.ru"]
           capabilities = ["pull", "resolve"]
           skip_verify = true
           ca = ["/path/to/ca.crt"]
           [host."https://registry.deckhouse.ru".auth]
             username = "username"
             password = "password"
             # If providing auth string:
             auth = "<base64>"
       EOF
       )
       mkdir -p "/etc/containerd/registry.d/${REGISTRY_ADDRESS}"
       echo "$host_toml" > "/etc/containerd/registry.d/${REGISTRY_ADDRESS}/hosts.toml"
     nodeGroups:
       - '*'
     weight: 0
   ```

   To test the configuration:

   ```bash
   # HTTPS:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ registry.io/registry/path:tag

   # HTTP:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http registry.io/registry/path:tag
   ```

1. Delete auth configurations from the `/etc/containerd/conf.d` directory.

## Switch to the previous containerd v1 authorization configuration

{% alert level="danger" %}
- This switch is only possible from the `Unmanaged` mode.
- When switching to the legacy containerd v1 auth configuration, any custom configurations in `/etc/containerd/registry.d` will stop working.
- [Custom auth configurations](/modules/node-manager/faq.html#how-to-add-additional-registry-auth) for the legacy auth format (using `/etc/containerd/conf.d`) can only be applied after switching to the legacy mode.
{% endalert %}

1. Switch the registry mode to `Unmanaged`.

1. Check the switching status using [this guide](#check-registry-mode-switch-status). Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Delete the secret registry-bashible-config:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. After deleting the secret, wait for the auth configuration to switch back to the legacy one in containerd v1.

   You can use [this guide](#check-registry-mode-switch-status) to track the switch. Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

## Check registry mode switch status

The status of the registry mode switch can be retrieved using the following command:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Example output:

```yaml
conditions:
  - lastTransitionTime: "2025-07-15T12:52:46Z"
    message: 'registry.deckhouse.ru: all 157 items are checked'
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2025-07-11T11:59:03Z"
    message: ""
    reason: ""
    status: "True"
    type: ContainerdConfigPreflightReady
  - lastTransitionTime: "2025-07-15T12:47:47Z"
    message: ""
    reason: ""
    status: "True"
    type: TransitionContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:52:48Z"
    message: ""
    reason: ""
    status: "True"
    type: InClusterProxyReady
  - lastTransitionTime: "2025-07-15T12:54:53Z"
    message: ""
    reason: ""
    status: "True"
    type: DeckhouseRegistrySwitchReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: FinalContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
mode: Direct
target_mode: Direct
```

The output displays the status of the switch process. Each condition can have a status of `True` or `False`, and may contain a `message` field with additional details.

Description of conditions:

| Condition                         | Description                                                                                                                                                                      |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ContainerdConfigPreflightReady`  | State of the containerd configuration preflight check. Verifies there are no custom containerd auth configurations on the nodes.                                             |
| `TransitionContainerdConfigReady` | State of preparing the containerd configuration for the new mode. Verifies that the configuration contains both the old and new mode settings.                                 |
| `FinalContainerdConfigReady`      | State of finalizing the switch to the new containerd mode. Verifies that the containerd configuration has been successfully applied and contains only the new mode settings. |
| `DeckhouseRegistrySwitchReady`    | State of switching Deckhouse and its components to use the new registry. `True` means Deckhouse successfully switched and is ready to operate.                                   |
| `InClusterProxyReady`             | State of In-Cluster Proxy readiness. Checks that the In-Cluster Proxy has started successfully and is running.                                                                   |
| `CleanupInClusterProxy`           | State of cleaning up the In-Cluster Proxy if it is not needed in the selected mode. Verifies that all related resources have been removed.                                       |
| `Ready`                           | Overall state of registry readiness in the selected mode. Indicates that all other conditions are met and the module is ready to operate.                                        |
