# RUNBOOK

This document describes diagnostics and actions for registry switching errors.

Additional verification commands:

1. Checking the switching status**

  ```bash
  watch -c "kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values registry | yq '.internal.orchestrator.state.conditions // []'"
  ```

1. Checking the deckhouse queue**

  ```bash
  watch kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
  ```

## Diagnostics of the switching stages

### `RegistryContainsRequiredImages`

1. Check the deckhouse queue. There must be no errors in the queue.
1. Check the switching status. The status will indicate an error about the availability of the registry and the images in it:

  ```Yaml
  ...
  - lastTransitionTime: "2026-06-18T08:41:23Z"
    message: |-
      Mode: Default
      some-nexus.io: 0 of 182 items processed, 182 items with errors:
      - source: deckhouse/containers/deckhouse
        image: some-nexus.io/nexus/deckhouse/path:release-1.76
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/terraform-manager/terraform-manager-dvp
        image: some-nexus.io/nexus/deckhouse/path@sha256:0429bcb05580b5b8a55242953dcacc4f150a8d757844a184bc2c5295d9de6d03
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/cloud-provider-vcd/cloud-data-discoverer-legacy
        image: some-nexus.io/nexus/deckhouse/path@sha256:04f22995347e40b5d64ef2b898ecc3eced40367ab0ee312222150cd5e6dd46a4
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/control-plane-manager/kube-controller-manager133
        image: some-nexus.io/nexus/deckhouse/path@sha256:05bdde23b414ed662946bbfda8c611240f2df17c40ee4af297ba7318a0caad81
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/cloud-provider-gcp/cloud-controller-manager131
        image: some-nexus.io/nexus/deckhouse/path@sha256:071f70dd9cc6c38c8d62fd9a26ae885d5be6a1892ca89ebda3df6c90ce4a6880
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host

        ...and more
    reason: Processing
    status: "False"
    type: RegistryContainsRequiredImages
  ...
  ```

1. If the error is related to registry availability:
   1. Check whether the registry is reachable from the cluster nodes. Example command to run the check: `ctr images pull --tlscacert=./path/to/ca --user="name:pass" --http-dump some-nexus.io/deckhouse/path:release-1.76`;
   1. Check the correctness of the parameters entered in `mc/deckhouse`. If the parameters are entered incorrectly — fix them;

1. If the error is related to images (your own image storage):
   1. Check whether the image is loaded into the local image storage `ctr images pull --tlscacert=./path/to/ca --user="name:pass" --http-dump some-nexus.io/deckhouse/path:release-1.76`
   1. Check whether there are any errors in the local image storage (storage logs);

> [!NOTE]
> For `Local` mode, the stage will be in error until a previously prepared image bundle is loaded into the local registry using the `d8 mirror push` command. Load the images and wait for the recheck.
> Example: [../EXAMPLES.md#switching-to-local-mode](../EXAMPLES.md#switching-to-local-mode)

### `ContainerdConfigPreflightReady`

1. Check the deckhouse queue. There must be no errors in the queue.
1. Check the switching status. The status will indicate an error from running the preflight check:

  ```bash
  $ d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)
  ...
  - lastTransitionTime: "2026-06-18T08:41:23Z"
    message: |
      Check current nodes configuration
      2/2 node(s) Unready:
      - master-0: has custom toml merge containerd configuration
      - worker-5e389be0-578df-s5sm5: has custom toml merge containerd configuration
  ...
  ```

1. If the status indicates the error `has custom toml merge containerd configuration`. You need to perform a migration. Detailed example: [../FAQ.md#how-to-migrate-to-the-registry-module](../FAQ.md#how-to-migrate-to-the-registry-module)

### `TransitionContainerdConfigReady`

Same as the `FinalContainerdConfigReady` item

### `FinalContainerdConfigReady`

1. Check the deckhouse queue. There must be no errors in the queue.
1. Check the switching status. The status will indicate the process of running the new bashible bundle version with the new registry configuration version:

```bash
...
- lastTransitionTime: "2026-06-18T08:41:23Z"
  message: |
    Applying configuration to nodes
    1/3 node(s) ready. Waiting:
    - master-1: "a1b2c3d4..." → "e5f6a7b8..."
    - worker-0: "a1b2c3d4..." → "e5f6a7b8..."
  reason: Processing
  status: "False"
  type: FinalContainerdConfigReady
...
```

1. If the condition does not pass for a long time:
   1. Check the bashible logs on the nodes: `journalctl -u bashible.service --no-pager -f`;
   1. If there are no errors, check the logs of the `node-manager` module components in the `d8-cloud-instance-manager` namespace;
1. Make sure that the nodes have the required containerd configuration in the `/etc/containerd/registry.d` directory

### `InClusterProxyReady`

1. Check the deckhouse queue. There must be no errors in the queue.
1. Check the switching status. The status will indicate a deployment error for the `registry-incluster-proxy` component.
1. Check the deployment status of the `registry-incluster-proxy` deployment. The deployment must roll out all pods. In normal mode/HA = 1/number of master nodes. There must be no errors in the pod logs:

  ```bash
  kubectl -n d8-system get deployment registry-incluster-proxy -o yaml
  kubectl -n d8-system describe deployment registry-incluster-proxy
  kubectl -n d8-system logs pod registry-incluster-proxy-<replica>
  ```

### `CleanupInClusterProxy`

1. Check the removal status of the `registry-incluster-proxy` deployment:

  ```bash
  kubectl -n d8-system get deployment registry-incluster-proxy -o yaml
  kubectl -n d8-system describe deployment registry-incluster-proxy
  ```

1. If the deployment is not being removed, you can perform the removal manually.
1. Check the switching status. The error should disappear from the status.

### `NodeServicesReady`

1. Check the deckhouse queue. There must be no errors in the queue.
1. Check the switching status. The status will indicate a deployment error for the `registry-nodeservices` component:

  ```yaml
  ...
  - message: |
      1/3 node(s) ready. Waiting:
      - master-1: node is not Ready
      - master-2: services pod(s) is not Ready or config version mismatch (!= "e5f6a7b8...")
    reason: Processing
    status: "False"
    type: NodeServicesReady
  ...
  ```

1. Check the deployment status of the `registry-nodeservices-manager` daemonset. The daemonset must roll out all pods. The number of pods = the number of master nodes. There must be no errors in the logs:

  ```bash
  kubectl -n d8-system get daemonset registry-nodeservices-manager -o yaml
  kubectl -n d8-system describe daemonset registry-nodeservices-manager
  kubectl -n d8-system logs pod registry-nodeservices-manager-<master-node>
  ```

1. Check the deployment status of the registry static pods themselves `registry-nodeservices-<master-node>`. There must be no errors in the pod logs:

  ```bash
  kubectl -n d8-system get pod registry-nodeservices-<master-node> -o yaml
  kubectl -n d8-system describe pod registry-nodeservices-<master-node>
  kubectl -n d8-system logs pod registry-nodeservices-manager-<master-node>
  ```

1. Check the node status. The node must be in the `Ready` state:

  ```bash
  kubectl get node <master-node> -o yaml
  kubectl describe node <master-node>
  ```

### `CleanupNodeServices`

1. Check the state of the `registry-nodeservices-manager` daemonset. The daemonset must remove the registry static pods `registry-nodeservices-<node>`;
1. Check whether the `registry-nodeservices-manager` daemonset has been removed.
1. If an instance of `registry-nodeservices-manager` does not get deployed on a node to remove `registry-nodeservices-<node>`. Remove the static pod manually:

   ```bash
   mv /etc/kubernetes/manifests/registry-nodeservices.yaml ~/registry-nodeservices.yaml
   mv /etc/kubernetes/manifests/registry ~/registry
   ```

1. Check the switching status. The error should disappear from the status.

### `DeckhouseRegistrySwitchReady`

1. Check the switching status. The status will indicate a deployment error for the `registry-nodeservices` component:

  ```yaml
  ...
  - message: |
      Waiting for deckhouse-controller to become ready
    reason: Processing
    status: "False"
    type: DeckhouseRegistrySwitchReady
  ...
  ```

1. If the error is: `Waiting for deckhouse-controller to become ready`:
   1. Check the deckhouse queue. There must be no errors in the queue. Deckhouse must run all hooks in all modules. After running all hooks and rendering all manifests, deckhouse will transition to the `Ready` state.
   1. Check the deckhouse logs — there must be no errors in the logs.

### `ErrTransitionNotSupported`

1. If `ErrTransitionNotSupported` appears in the conditions with `status: "True"` and `reason: Error`, an
unsupported transition between modes was requested. The following transitions are not supported:
- `Proxy` → `Local`;
- `Local` → `Proxy`;
- `Local` → non-configurable `Unmanaged` (without `imagesRepo`).
1. To switch to these modes, you need to first switch to an intermediate `Direct`/`Unmanaged` mode. After that, you can switch to the required mode.

## Common errors

### Changing an expired login/password

1. Check the current mode:

  ```bash
  $ kubectl get mc/deckhouse -o yaml
  ...
  registry:
    mode: Direct
    direct:
      ...
  ...
  ```

  ```bash
  $ d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
  ...
  mode: Direct
  target_mode: Direct
  ```

1. Change the registry parameters in the current operating mode:

  ```bash
  $ kubectl edit mc/deckhouse

  registry:
    mode: Direct # if Direct mode was specified in the previous step
    direct:
      ...
  ```

1. Check the switching status. Wait for the parameter change to complete.

### Recovering the deckhouse pod in Direct mode (on ImagePullBackOff)

The solution covers only the case where the deckhouse pod is in `ImagePullBackOff` and cannot start due to expired `username/password` registry parameters.
The case does not cover other modes or changes to other parameters.

1. Save the mutable cluster parameters:

   ```bash
   kubectl get mc/deckhouse -o yaml > mc_deckhouse.yaml
   kubectl get ms/deckhouse -o yaml > ms_deckhouse.yaml
   kubectl -n d8-system get secret/deckhouse-registry -o yaml > secret_deckhouse_registry.yaml
   ```

1. Prepare the new `.dockerconfig` value for `mc/deckhouse` and `ms/deckhouse`:

   ```bash
   export registry_username="new-username"
   export registry_password="new-password"
   AUTH=$(echo -n "${registry_username}:${registry_password}" | base64 -w 0)

   echo -n '{"auths":{"registry.d8-system.svc:5001":{"username":"'"${registry_username}"'","password":"'"${registry_password}"'","auth":"'"${AUTH}"'"}}}' | base64 -w 0
   ```

1. Change the new username and password registry parameters in `mc/deckhouse`:

   ```bash
   kubectl --as=system:sudouser edit mc/deckhouse
   ```

1. Insert the `.dockerconfig` value obtained in step 2 into `ms/deckhouse` and into `secret/deckhouse-registry`:

   ```bash
   kubectl --as=system:sudouser edit ms/deckhouse
   kubectl --as=system:sudouser -n d8-system edit secret/deckhouse-registry
   ```

1. Make sure the `ImagePullBackOff` error has disappeared.
1. Wait for the registry switching status to the new credentials to complete.
