---
title: "Module registry: FAQ"
description: ""
---

## How to check the registry mode switch status?

You can get the status of the registry mode switch using the following command:

```bash
kubectl -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'"
```
<!-- TODO(nabokihms): replace with a d8 subcommand when implemented -->


### What do the conditions in `registry-state` mean?

Each condition can be either `True` or `False` and also has a `message` field containing a description of the state. Below are all the types of conditions that can be present in `registry-state` and their descriptions:


| Condition                         | Description                                                                                                                                                                              |
|-----------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ContainerdConfigPreflightReady`  | The state of checking the `containerd` configuration before switching modes. Checks that the node is using `containerd` and that its configuration does not contain manual user changes. |
| `TransitionContainerdConfigReady` | The state of preparing the `containerd` configuration for the new mode. Checks that the `containerd` configuration has been successfully prepared for the new mode.                      |
| `FinalContainerdConfigReady`      | The state of completing the mode switch. Checks that all changes to the `containerd` configuration have been successfully applied and the system is ready to operate in the new mode.    |
| `DeckhouseRegistrySwitchReady`    | Switching to the new container registry address. Checks that the new address has been successfully applied and Deckhouse components are ready to work with the new registry.             |
| `InClusterProxyReady`             | The readiness state of the In-Cluster Proxy. Checks that the In-Cluster Proxy has been successfully started and is running.                                                              |
| `CleanupInClusterProxy`           | The state of cleaning up the In-Cluster Proxy if the proxy is not needed for the desired mode. Checks that all resources related to the In-Cluster Proxy have been successfully removed. |
| `NodeServicesReady`               | The readiness state of node services. Checks that all required services on the nodes have been successfully started and are running.                                                     |
| `CleanupNodeServices`             | The state of cleaning up node services if they are not needed for the desired mode. Checks that all resources related to node services have been successfully removed.                   |
| `Ready`                           | The overall readiness state of the registry for operation in the specified mode. Checks that all previous conditions are met and the module is ready for operation.                      |

