---
title: "Registry Module: FAQ"
description: "Frequently asked questions about the Deckhouse Kubernets Platform registry module including migration procedures, containerd configuration, and troubleshooting registry issues."
---

## Which implementation is my cluster running?

The module records what the cluster is actually running, not what any configuration asks for:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "current implementation" || echo "previous implementation"
```

If it is the previous one, the module says why on every reconciliation, and raises
[`D8RegistryMigrationPending`](#what-do-the-registry-alerts-mean):

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

## How do I complete the migration?

There is no implementation to select — the handover is automatic, and it waits for one thing:
that the previous implementation has let go of the pull path. Both configure the same thing on
every node, which registry the container runtime asks and with which credentials, so running
both would not merge those answers but race them.

1. Bring the registry configuration in the `deckhouse` ModuleConfig to `Unmanaged`. If that
   cluster is in `Direct`, `Proxy` or `Local`, follow
   [the mode switching examples](examples.html#examples-for-the-previous-implementation).

1. Wait for that transition to settle — `mode: Unmanaged` with no pending target mode:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Nothing else is required. On its next reconciliation the module takes over, and the cluster
   keeps pulling from the same registry throughout: with `mode: Unmanaged` — the default of the
   current implementation — it manages nothing either, so the handover changes no behaviour.

1. To have it manage the pull path, set `mode: Managed` in the `registry` ModuleConfig together
   with the registry to pull from. A ready-made configuration for your cluster is published in
   the `registry-suggested-config` secret — see
   [enabling the module](examples.html#enabling-the-module).

## What do the registry alerts mean?

None of these mean the cluster has stopped pulling images. Most of them mean it is pulling in a
way that is working but not what was asked for, which is the state that would otherwise go
unnoticed.

`D8RegistryMigrationPending`
: The cluster is still on the previous implementation. Nothing is degraded; the migration has
  not completed. See [above](#how-do-i-complete-the-migration).

`D8RegistryConfigInvalid`
: The configuration was rejected, so the cluster keeps the arrangement it already had. What was
  wrong is in `registryconfig/registry` under `.status.conditions`.

`D8RegistryNodeNotConverged`
: The agent on some nodes has not applied the layout it was given. Those nodes keep pulling as
  they were configured before, so a change has not reached them — and neither will the next one.

`D8RegistryNodeRunningFromDisk`
: Some nodes cannot reach the API server and are routing from their on-disk copy. That copy
  working as designed, which is exactly why it is worth saying: those nodes pull normally, so
  from every other angle it looks like success, and their configuration can drift arbitrarily
  far behind the cluster's.

`D8RegistryStorageIncomplete`
: Some cache replicas do not hold the whole expected image set. With an upstream configured this
  costs nothing at pull time, but the cluster could not survive losing that upstream — and this
  is what holds an air-gap transition back.

`D8RegistryAirGapTransitionHeld`
: You removed the upstream and the module is still using it, because the cache cannot stand
  alone yet. The safe outcome, and the one transition here that could otherwise cut every node
  off from images. It does not resolve on its own if the cache has stopped filling.

`D8RegistryUpstreamProbeFailing`
: A change to the primary upstream was refused and the cluster is still using the last one that
  worked. The `outcome` label distinguishes three different problems: `unreachable` is network
  or registry, `auth` is usually an expired license key, `sentinel` means the registry answered
  and accepted the credentials but does not hold the Deckhouse images — usually the wrong
  repository path.

`D8RegistryUpstreamRejected`
: A `RegistryUpstream` was not accepted, so pulls for the registry it names are not intercepted
  anywhere. The `reason` label says whether it conflicts with the primary registry or with
  another resource claiming the same name.

`D8RegistryStorageNotReclaimed`
: No replica has reclaimed its disk for a week. The collection is the only thing that removes
  anything from the store, so a cluster where it has stopped is on a path that ends with a full
  disk. See [below](#the-cache-keeps-growing-what-reclaims-it).

`D8RegistryStaleCacheData`
: A node holds cache data nothing uses. See [below](#a-node-still-holds-cache-data-nothing-uses).

## A node still holds cache data nothing uses

When the cache is turned off, the blobs under `/opt/deckhouse/registry` are deliberately left
behind: turning it back on then refills from what is already there rather than from scratch, and
over a slow link that difference is hours. Deleting them automatically would make the decision
irreversible in the one direction that hurts.

What the module will not do is keep them quietly — nothing else would ever mention the disk they
occupy, since the storage that wrote them is gone. So the agent measures them and reports:

```bash
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,STALE:.status.staleStorageDataBytes
```

To reclaim the space, remove the directory on the node:

```bash
ssh <node> 'du -sh /opt/deckhouse/registry && sudo rm -rf /opt/deckhouse/registry'
```

## The cache keeps growing. What reclaims it?

A garbage collection, on a schedule, run by the replicas themselves.

It exists because nothing else ever removes anything. Every release adds a slice of the
repository, so a cluster that lives for years fills its store and then stops being able to
write to it — which in an air-gapped cluster means it cannot be updated.

What it removes is the slices belonging to releases the cluster has moved past. What it keeps:

- the deployed release, and the previous one, so a rollback does not re-download what it rolls
  back to;
- anything newer than the deployed release, which is an update in progress — or, in air-gap, a
  release someone pushed on purpose;
- every tag that is not a version at all: release channel names like `stable`, floating tags,
  anything pushed by hand. Not "this is garbage" but "what this means is unknown", and those
  are different.

That asymmetry is deliberate throughout. Deleting a blob the cluster still needs is, in
air-gap, unrecoverable without another `d8 mirror push`; keeping one nobody needs costs disk.
So a run that cannot establish what to keep — no deployed release, for instance — does nothing
at all rather than its best.

Where it stands:

```bash
d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
  'map({node, collectedAt, collectionError})'
d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
```

### When it runs, and why a replica goes read-only

Collecting reclaims blobs by walking the store, and the registry's own collector computes the
set of reachable blobs and then deletes the rest — so a blob uploaded between those two steps
would be deleted. The only safe way to run it is against a store nothing is writing to, so the
replica refuses writes for the duration.

That replica keeps serving every image it holds. What it cannot do is store the result of a
cache miss (the node's agent falls back to the upstream, so the pull is slower rather than
failed) or accept a `d8 mirror push` (which fails visibly and can be retried). Only one replica
collects at a time, so the others are unaffected throughout.

Because of that, the schedule defaults to a night hour — or, if the `master` node group has a
maintenance window, to the start of it, that being an hour already declared safe for disruption.
To choose your own:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        schedule: "0 2 * * Sun"
```

An expression that cannot be read is refused rather than guessed at: collecting at some other
hour than the one you wrote would be worse than not collecting.

### Turning it off

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        enabled: false
```

Which only makes sense with a disk large enough that a store growing without bound never
matters. [`D8RegistryStorageNotReclaimed`](#what-do-the-registry-alerts-mean) fires a week after
the last collection either way, since "switched off" and "silently stopped" look identical from
the outside.

## A pull is failing on a node. Where do I look?

The agent is on the path of every pull on the node, so start there. It runs as a static pod, so
it is present even when the cluster is not:

```bash
d8 k -n kube-system logs -l component=registry-agent --tail=100
```

What the agent thinks it should be doing, and whether it agrees with the cluster:

```bash
d8 k get registrynode <node> -o jsonpath='{.status}' | jq
```

Its own view of the pulls passing through it, read from the node. Not scraped by Prometheus, and
deliberately: the agent is a static pod because it has to work when the API server does not, and
a kube-rbac-proxy beside it would authenticate against that same API server.

```bash
ssh <node> 'curl -s http://127.0.0.1:4286/metrics | grep d8_registry_agent'
```

What the container runtime was told, which is one file and does not depend on how many
registries are configured:

```bash
ssh <node> 'cat /etc/containerd/registry.d/_default/hosts.toml'
```

If that file is missing, the agent has not applied a layout yet — nothing on the node can pull,
and the reason is in its log. If it is present and pulls still fail, the failure is past the
agent: the metrics above name which target failed and why.

## How do I check the state of the in-cluster cache?

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq
```

`replicas` is the only place completeness is reported, and each entry is a replica's own account
of itself. A replica reporting `full: true` alongside an `error` is not complete: `full` says
what it holds, and the error says whether its last pass finished.

`Leader` is the replica filling from the upstream and acting as the replication source for the
others. It is not a plain election — only a replica holding the whole set stands for it, and one
holding it steps aside when another does. That condition is what keeps an air-gapped cluster
from deadlocking with an empty leader and a full follower.

## The previous implementation

Everything below applies to a cluster still running the implementation configured through the
`deckhouse` ModuleConfig.

### How to Migrate to the registry module?

During the migration, Containerd v1 will switch to the new registry configuration format.
Containerd v2 uses the new format by default. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry).

#### For containerd v2

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the `deckhouse` `moduleConfig`. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

   You can view the current registry settings using the following command:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Specify this configuration when setting up the `Unmanaged` mode:

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
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Wait for the switch to complete. Example [status output](./faq.html#how-to-check-the-registry-mode-switch-status):

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

#### For Containerd v1

{% alert level="danger" %}
- During the switch, containerd v1 will be restarted.
- During the switch, containerd v1 will be migrated to the new registry configuration scheme.
- During the switch, [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) for containerd v1 will be temporarily unavailable.
{% endalert %}

1. Make sure that nodes with containerd v1 do not have any [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) located in the `/etc/containerd/conf.d` directory.

1. If configurations are present, you need to migrate to the new registry configuration format in containerd. To do this, add new configuration files to the `/etc/containerd/registry.d` directory. These configurations will take effect after switching to the `registry` module. To add configurations, prepare a `NodeGroupConfiguration`. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry). Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # The step can be arbitrary, as restarting the containerd service is not required
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.
  
       REGISTRY_URL=private.registry.example

       mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
       bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
       [host]
         [host."https://${REGISTRY_URL}"]
           capabilities = ["pull", "resolve"]
           [host."https://${REGISTRY_URL}".auth]
             username = "username"
             password = "password"
       EOF
   ```

1. Apply the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Wait until the configuration files appear in the `/etc/containerd/registry.d` directory on all nodes.

1. Verify that the configurations are working correctly. To do this, use the following command:

   ```bash
   # For HTTPS:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag

   # For HTTP:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the `deckhouse` `moduleConfig`. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

   You can view the current registry settings using the following command:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Specify this configuration when setting up the `Unmanaged` mode:

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
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. After applying, wait for the following message to appear in the [switch status](faq.html#how-to-check-the-registry-mode-switch-status):

   Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T15:22:34Z"
     message: |
       Check current nodes configuration
       2/2 node(s) Unready:
       - master-0: has custom toml merge containerd configuration
       - worker-5e389be0-578df-s5sm5: has custom toml merge containerd configuration
     reason: Processing
     status: "False"
     type: ContainerdConfigPreflightReady
   ```

   This message means that there are old registry configurations on the nodes located in the `/etc/containerd/conf.d` directory. The switch to the new containerd configuration is currently blocked. To allow the switch, you need to remove the old configuration files.

1. Remove the old configuration files to allow switching to the `registry` module. To do this, create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Example of a NodeGroupConfiguration manifest:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth-delete.sh
   spec:
     # To add a file before the '032_configure_containerd.sh' step
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.
  
       file="/etc/containerd/conf.d/old-config.toml"

       [ -f "$file" ] && rm -f "$file"
   ```

1. After removing the old configurations, make sure that the switch has resumed. Example of the [switch status](faq.html#how-to-check-the-registry-mode-switch-status):

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Wait for the switch to complete. Example of the [switch status](faq.html#how-to-check-the-registry-mode-switch-status):

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

1. Delete the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) created in the step for deleting old configuration files:

   ```shell
   d8 k delete nodegroupconfiguration containerd-additional-config-auth-delete.sh
   ```

   To verify that NodeGroupConfiguration has been deleted, use the command:

   ```shell
   d8 k get nodegroupconfiguration
   ```

   The list should not contain the NodeGroupConfiguration to be deleted (for this example, `containerd-additional-config-auth-delete.sh`).

### How to Migrate Back from the Registry Module?

{% alert level="danger" %}
- This is a deprecated registry management format.
- During the switch, containerd v1 will be restarted.
- During the switch, containerd v1 will be migrated to the legacy registry configuration scheme.
- During the switch, [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) for containerd v1 will be temporarily unavailable.
{% endalert %}

1. Switch the registry to `Unmanaged` mode. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

   Example configuration:

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
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY>
   ```

1. Check the switch status using the [instruction](./faq.html#how-to-check-the-registry-mode-switch-status). Example output:

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

1. Switch the registry to the non-configurable `Unmanaged` mode. Example configuration:

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

1. Check the switch status using the [instruction](./faq.html#how-to-check-the-registry-mode-switch-status). Example output:

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

1. If containerd v1 is used and [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) are applied in the cluster, they must be replaced with the old format. To do this, prepare the registry configurations in the old format. These configurations do not need to be applied at this stage. Example configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # To add a file before the '032_configure_containerd.sh' step
     weight: 31
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.
  
       REGISTRY_URL=private.registry.example

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
               [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
                 endpoint = ["https://${REGISTRY_URL}"]
             [plugins."io.containerd.grpc.v1.cri".registry.configs]
               [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
                 username = "username"
                 password = "password"
                 # OR
                 auth = "dXNlcm5hbWU6cGFzc3dvcmQ="
       EOF
   ```

1. Delete the `registry-bashible-config` secret. This will trigger containerd v1 to switch back to the legacy registry format:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. After deletion, wait for the switch to complete. Use the [instruction](faq.html#how-to-check-the-registry-mode-switch-status) to track the progress. Example output:

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

1. If containerd v1 is used, apply the previously prepared `NodeGroupConfiguration` with custom registry configurations.

1. Disable the `registry` module. Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: false
     settings: {}
     version: 1
   ```

### How to check the registry mode switch status?

The status of the registry mode switch can be retrieved using the following command:

<!-- TODO(nabokihms): replace with d8 subcommand when available -->
```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Example output:

```yaml
conditions:
  - lastTransitionTime: "2025-07-15T12:52:46Z"
    message: 'registry.deckhouse.io: all 157 items are checked'
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

| Condition                         | Description                                                                                                                                                                                                                |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ContainerdConfigPreflightReady`  | State of the containerd configuration preflight check. Verifies there are no custom containerd auth configurations on the nodes.                                                                                           |
| `TransitionContainerdConfigReady` | State of preparing the containerd configuration for the new mode. Verifies that the configuration contains both the old and new mode settings.                                                                             |
| `FinalContainerdConfigReady`      | State of finalizing the switch to the new containerd mode. Verifies that the containerd configuration has been successfully applied and contains only the new mode settings.                                               |
| `DeckhouseRegistrySwitchReady`    | State of switching Deckhouse and its components to use the new registry. `True` means Deckhouse successfully switched and is ready to operate.                                                                             |
| `InClusterProxyReady`             | State of In-Cluster Proxy readiness. Checks that the In-Cluster Proxy has started successfully and is running.                                                                                                             |
| `CleanupInClusterProxy`           | State of cleaning up the In-Cluster Proxy if it is not needed in the selected mode. Verifies that all related resources have been removed.                                                                                 |
| `NodeServicesReady`               | State of Node Services Manager and Static-Pod registry readiness. Verifies that the Node Services Manager is successfully launched and operational, and that the Static-Pod registry has been successfully deployed by it. |
| `CleanupNodeServices`             | State of cleaning up the Node Services Manager and Static-Pod registry if they are not needed in the selected mode. Verifies that all related resources have been removed.                                                 |
| `RegistryContainsRequiredImages`  | State of checking the registry for the presence of required images.                                                                                                                                                        |
| `Ready`                           | Overall state of registry readiness in the selected mode. Indicates that all other conditions are met and the `modul`e is ready to operate.                                                                                |
