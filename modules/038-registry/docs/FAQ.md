---
title: "Registry Module: FAQ"
description: "Frequently asked questions about the Deckhouse Kubernets Platform registry module including migration procedures, mode switching, containerd configuration, and troubleshooting registry issues."
---

## How to prepare containerd v1?

{% alert level="info" %}

When switching to the `Direct` mode, the containerd v1 service will be restarted.  
The authorization configuration will be switched to Mirror Auth (this configuration is used by default in `Containerd V2`).  
After switching back to `Unmanaged`, the updated authorization configuration will remain unchanged.

{% endalert %}

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

Before switching, make sure there are no [custom registry configurations](/products/kubernetes-platform/documentation/v1.72/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry) present on nodes with containerd v1 in the `/etc/containerd/conf.d` directory.

If such configurations exist:

{% alert level="danger" %}
- After deleting [custom registry configurations](/products/kubernetes-platform/documentation/v1.72/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry) from the `/etc/containerd/conf.d` directory, the containerd service will be restarted. The removed configurations will no longer work.
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

## How to switch back to the previous containerd v1 auth configuration?

{% alert level="warning" %}
This containerd configuration format is deprecated.
{% endalert %}

{% alert level="danger" %}
- This switch is only possible from the `Unmanaged` mode.
- When switching to the legacy `Containerd V1` auth configuration, any custom configurations in `/etc/containerd/registry.d` will stop working.
- [Custom registry configurations](/products/kubernetes-platform/documentation/v1.72/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry) for the legacy auth format (using `/etc/containerd/conf.d`) can only be applied after switching to the legacy mode.
{% endalert %}

1. Switch the registry mode to `Unmanaged`.

1. Check the switching status using [this guide](./faq.html#how-to-check-the-registry-mode-switch-status). Example output:

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

1. After deleting the secret, wait for the auth configuration to switch back to the legacy one in `Containerd V1`.

   You can use [this guide](./faq.html#how-to-check-the-registry-mode-switch-status) to track the switch. Example output:

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

## How to check the registry mode switch status?

The status of the registry mode switch can be retrieved using the following command:

<!-- TODO(nabokihms): replace with d8 subcommand when available -->
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
