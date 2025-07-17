---
title: "Registry Module: FAQ"
description: ""
---

## How to prepare Containerd V1?

{% alert level="danger" %}
When removing [custom Auth configurations](/products/kubernetes-platform/documentation/v1/modules/node-manager/faq.html#how-to-add-additional-registry-auth), the containerd service will be restarted.  
New Mirror Auth configurations added to `/etc/containerd/registry.d` will only take effect after switching to one of the `Managed` registry modes (`Direct`, `Local`, `Proxy`).
{% endalert %}

During the switch to any of the `Managed` modes (`Direct`, `Local`, `Proxy`), the `Containerd V1` service will be restarted.  
The `Containerd V1` authorization configuration will be changed to Mirror Auth (this configuration is used by default in `Containerd V2`).

Before switching, make sure there are no [custom authorization configurations](/products/kubernetes-platform/documentation/v1/modules/node-manager/faq.html#how-to-add-additional-registry-auth) on nodes with `Containerd V1` that are located in the `/etc/containerd/conf.d` directory.

If such configurations exist, they must be deleted and replaced with new authorization configurations in the `/etc/containerd/registry.d` directory. Example:

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
      [host."${REGISTRY_SCHEME}://${REGISTRY_ADDRESS}"]
        capabilities = ["pull", "resolve"]
        skip_verify = true
        ca = ["/path/to/ca.crt"]
        [host."${REGISTRY_SCHEME}://${REGISTRY_ADDRESS}".auth]
          username = "username"
          password = "password"
          # If auth string:
          auth = "<base64>"
    EOF
    )
    mkdir -p "/etc/containerd/registry.d/${REGISTRY_ADDRESS}"
    echo "$host_toml" > "/etc/containerd/registry.d/${REGISTRY_ADDRESS}/hosts.toml"
  nodeGroups:
    - '*'
  weight: 0
```

To verify the new configuration is working correctly, use the command:

```bash
# for https:
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ registry.io/registry/path:tag
# for http:
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http registry.io/registry/path:tag
```

## How to check the registry mode switch status?

The status of the registry mode switch can be retrieved using the following command:

<!-- TODO(nabokihms): replace with d8 subcommand when available -->
```bash
kubectl -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
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
| `ContainerdConfigPreflightReady`  | State of the `containerd` configuration preflight check. Verifies there are no custom `containerd` auth configurations on the nodes.                                             |
| `TransitionContainerdConfigReady` | State of preparing the `containerd` configuration for the new mode. Verifies that the configuration contains both the old and new mode settings.                                 |
| `FinalContainerdConfigReady`      | State of finalizing the switch to the new `containerd` mode. Verifies that the `containerd` configuration has been successfully applied and contains only the new mode settings. |
| `DeckhouseRegistrySwitchReady`    | State of switching Deckhouse and its components to use the new registry. `True` means Deckhouse successfully switched and is ready to operate.                                   |
| `InClusterProxyReady`             | State of In-Cluster Proxy readiness. Checks that the In-Cluster Proxy has started successfully and is running.                                                                   |
| `CleanupInClusterProxy`           | State of cleaning up the In-Cluster Proxy if it is not needed in the selected mode. Verifies that all related resources have been removed.                                       |
| `NodeServicesReady`               | State of node services readiness. Verifies that all necessary services on the nodes have started and are operational.                                                            |
| `CleanupNodeServices`             | State of cleaning up node services if they are not needed in the selected mode. Ensures all related resources have been removed.                                                 |
| `Ready`                           | Overall state of registry readiness in the selected mode. Indicates that all other conditions are met and the module is ready to operate.                                        |
