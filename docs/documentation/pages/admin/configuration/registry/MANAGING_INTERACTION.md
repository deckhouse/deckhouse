---
title: Managing the registry in Deckhouse Platform-managed clusters
permalink: en/admin/configuration/registry/managing-interaction.html
description: "Managing registry interaction settings in the Deckhouse Platform. Deckhouse Platform component registry interaction modes."
---

In [clusters fully managed by Deckhouse Platform](../../../glossary.html#dp-managed-cluster) (Deckhouse Platform), you can configure the download paths:

- images of Deckhouse Platform component containers;
- images of containers from additional repositories (vendor repositories or your own).

The pull paths for container images are managed using the [`registry`](/modules/registry/) module.

## Implementations of container image pull path management

Deckhouse Platform supports two implementations of the `registry` module for managing container image download paths:

- **The current implementation**. Used starting with Deckhouse Platform 1.78. Container image pull paths are configured through the [`registry` ModuleConfig](/modules/registry/configuration.html). The implementation is described in the ["Current implementation"](#current-implementation) section.
- **The v1 implementation**. Used in Deckhouse Platform up to and including 1.77. The [Deckhouse Platform component registry](../../../glossary.html#component-registry) is configured through the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). The implementation is described in the ["V1 implementation"](#v1-implementation) section.

The two implementations never manage a cluster at the same time: only one of them is active at any given moment.
A cluster that has never run the v1 implementation uses the current one right away.
A cluster running the v1 implementation keeps using it until the conditions for switching to the current one are met — control is then handed over to the current implementation automatically, with no separate command. The switching conditions and step-by-step instructions for each mode of the v1 implementation are given in the ["Migrating from the v1 implementation to the current one"](#migrating-from-the-v1-implementation-to-the-current-one) section below.

{% alert level="warning" %}
Starting from version 1.78 Deckhouse Platform no longer uses the v1 implementation of the module. The cluster switches to the current implementation after the upgrade to Deckhouse Platform 1.78, and the preparation for that is done before the upgrade, on Deckhouse Platform 1.77.
{% endalert %}

{% alert level="danger" %}
Prepare your cluster in advance for the transition to the current implementation. The update to Deckhouse Platform 1.78, which no longer contains the v1 implementation, is blocked as long as the cluster is running in `Proxy` or `Local` mode of the v1 implementation, or in `Direct` mode without a configured `registry` in ModuleConfig.
{% endalert %}

You can find out which implementation of the module the cluster uses by checking whether the `registry-v2-switch` secret exists. To do this, follow the [instructions](#checking-which-implementation-the-cluster-uses).

## Current implementation

In the current implementation, container image pull paths are configured through the [`registry` ModuleConfig](/modules/registry/configuration.html).

### Modes of operation

In the current implementation, Deckhouse Platform supports the following container image pull path management modes (the mode is set with the [`mode`](/modules/registry/cr.html#registryconfig-v1alpha1-spec-mode) parameter in the `registry` ModuleConfig):

- `Unmanaged` (the default mode). The `registry` module does not manage the pull paths of Deckhouse Platform component images: the cluster pulls images from the registry it was installed with (the registry is specified during cluster bootstrap in the [`deckhouse`](../../../reference/api/cr.html#initconfiguration-deckhouse) parameter in InitConfiguration).
- `Managed`. In this mode, the `registry` module manages the pull paths for container images. The following independent settings are used in this mode:

  - [`primary.upstream`](/modules/registry/latest/configuration.html#parameters-primary-upstream) — the container image registry DP component images are pulled from. If the parameter is not set, the cluster is considered isolated (air-gapped). In an isolated cluster, the only source of container images is the [in-cluster cache](../../../glossary.html#in-cluster-cache), which is filled with the [`d8 mirror push`](../../../cli/d8/reference/#d8-mirror-push) command.
  - [`storage.cache`](/modules/registry/latest/configuration.html#parameters-storage-cache) — controls the in-cluster cache on the [master nodes](../../../glossary.html#master-node). If the parameter is enabled, a container image registry is deployed on the master nodes, and all nodes pull images from it, with the upstream registry staying a fallback path while the cache is being filled.

The [`primary.upstream`](/modules/registry/latest/configuration.html#parameters-primary-upstream) and [`storage.cache`](/modules/registry/latest/configuration.html#parameters-storage-cache) parameters together cover every supported configuration:

| `primary.upstream` | `storage.cache` | What the cluster does |
|---|---|---|
| Set | `false` | Nodes pull images directly from the upstream registry. Nothing is deployed on the master nodes |
| Set | `true` | The cache acts as a pass-through: filled from the upstream registry on demand and ahead of time |
| Not set | `true` | The cluster is isolated (air-gapped). The cache is the only source. The [`d8 mirror push`](../../../cli/d8/reference/#d8-mirror-push) command is used to fill it |
| Not set | `false` | The configuration is rejected: nodes would have nowhere to pull images from |

You can reconfigure cache enablement and disablement, change the image storage location, and change your credentials at any time. However, when removing an upstream registry to transition to an isolated state, the module waits until the entire expected set of images appears in the intra-cluster cache—only then does it stop using the upstream registry. If the upstream registry stopped being used before the in-cluster cache held the whole expected image set, nodes could be left with no image source while the cache is being filled.

For more details on the architecture of the current implementation (the node agent, the in-cluster cache, leader replica election, how isolated clusters work), see the [`registry` module documentation](/modules/registry/latest/#current-implementation).

### Installer behavior when no container image registry is configured

The installer creates the `registry` ModuleConfig with `mode: Managed` and `primary.upstream` pointing to the public Deckhouse Open/CE registry `registry.deckhouse.io/deckhouse/ce`, and warns about it in its output when the installation configuration specifies no container image registry in any of the following places:

- [InitConfiguration](../../../reference/api/cr.html#initconfiguration-deckhouse)
- The `registry` section of the `deckhouse` ModuleConfig
- The `registry` ModuleConfig (the ModuleConfig is absent)

This applies only when the installation configuration contains a ClusterConfiguration and the container runtime is managed by Deckhouse Platform (`defaultCRI` is `Containerd` or `ContainerdV2`). In a Managed Kubernetes cluster (without ClusterConfiguration) and with `defaultCRI: NotManaged`, the installer does not create this ModuleConfig.

Under the same conditions, the installer rejects a configuration in which the `registry` ModuleConfig sets `mode: Unmanaged` (or omits `mode`, which means the same) or `enabled: false` and no registry is specified elsewhere. In this case, the nodes would have nowhere to pull images from. Specify the registry from which nodes pull images directly in one of the following ways:

- Via the [`deckhouse`](../../../reference/api/cr.html#initconfiguration-deckhouse) parameter in InitConfiguration (`imagesRepo` and `registryDockerCfg`).
- By setting `mode: Managed` with `primary.upstream` in the `registry` ModuleConfig. A private or mirror registry populated with `d8 mirror push` can serve as `primary.upstream`. To install an air-gapped cluster from an image bundle instead, set `enabled: true` and `mode: Managed` with `storage.cache: true` and `storage.source`, without `primary.upstream`, and pass the bundle to the installer with the `--img-bundle-path` flag.

The installer also rejects a `registry` ModuleConfig that specifies `primary` or `storage` without `mode: Managed`, as the module schema does: in the default `Unmanaged` mode, these settings have no effect.

### Garbage collection

Deckhouse Platform has a garbage collection mechanism for the container image registry. Each registry replica runs the operation on its own store. Garbage collection does not block image pulls from the store.

Garbage collection starts in the following cases:

- Once a day at night (at 3:17 a.m. Moscow Standard Time). Garbage collection is enabled by default. It is scheduled at night because filling the replica's store is paused during the operation. A `d8 mirror push` command started during garbage collection can therefore fail.
- When all replicas have received a new image set: after a Deckhouse Platform update or after the `registry` module is enabled or updated.
- When the image store refuses writes because it has exceeded its designated storage limit or reached the reserved free space threshold on its filesystem.

For details about when garbage collection starts and what happens during the operation, see the [documentation for the `registry` module](/modules/registry/latest/faq.html#when-it-runs-and-what-happens-during-a-run).

You can also [start garbage collection manually](#running-garbage-collection-manually).

If you need to disable or configure garbage collection, use the [`storage.garbageCollection`](/modules/registry/latest/configuration.html#parameters-storage-garbagecollection) parameter in the `registry` ModuleConfig.

{% alert level="warning" %}
Disabling garbage collection only makes sense if the storage uses a disk with sufficient capacity to accommodate unlimited storage growth.
{% endalert %}

To view the status and schedule of garbage collection, use the commands below.

- List the recent collections — what started them, how they went, and how much they freed. A finished
collection is kept for a day:

  ```bash
  d8 k get registryoperation -o wide
  ```

- View the time of the last garbage collection on each replica and any errors, if any:

  ```bash
  d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
    'map({node, collectedAt, collectionError})'
  ```

- View the current garbage collection schedule:

  ```bash
  d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
  ```

Garbage collection is the only mechanism that removes data from the registry. Each [Deckhouse Platform release](../../../glossary.html#dp-release) adds new images, so without garbage collection, the storage of a cluster that has been running for a long time will eventually fill up, and new images will no longer be added to it. However, for example, an isolated cluster with a full storage cannot be updated.

Collection removes the images of the releases the cluster has moved past. It keeps:

- the deployed release and the previous one, so that a rollback does not have to re-download
  anything;
- everything newer than the deployed release — an update in progress or, in an air-gapped
  cluster, a release pushed on purpose;
- every tag that is not a version: [release channel](../../../glossary.html#release-channel) names such as `stable`, floating tags,
  anything pushed by hand. The collector cannot know what these mean, so it does not touch
  them.

It also removes blobs that no image references, such as those left by an interrupted fill or by a
collection whose cleanup failed. A collection removes them even when there are no images to delete.

The collector is deliberately cautious. In an isolated cluster, deleting a blob that is still needed is unrecoverable without another `d8 mirror push`, while keeping an unneeded one merely costs disk space. So a run that cannot determine what to keep — for example, when no deployed release is found — does nothing at all.

For more information about how the garbage collector works, see [the `registry` module documentation](/modules/registry/latest/faq.html#the-cache-keeps-growing-what-reclaims-it).

#### Running garbage collection manually

{% alert level="info" %}
Garbage collection created manually runs even when automatic collection is disabled.
{% endalert %}

Before starting garbage collection manually, assess whether it is needed. See the ["Is a collection worth running now?"](/modules/registry/latest/faq.html#is-a-collection-worth-running-now) section of the `registry` module FAQ.

To start garbage collection manually, create a [RegistryOperation](/modules/registry/latest/cr.html#registryoperation) object of type `GarbageCollection`:

```bash
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: RegistryOperation
metadata:
  generateName: gc-manual-
spec:
  type: GarbageCollection
  cause: Free space before pushing a new release.
EOF
```

It runs even when automatic collection is disabled. Replicas collect one at a time, exactly as on schedule. A replica starts only when its store is not being written to: while the cache is being filled or a `d8 mirror push` is in progress, the replica waits, and the reason is shown in `status.replicas[].reason`. A store that refuses writes is not waited for: a fill cannot go on until the collection frees room. A replica still held back an hour after the operation was accepted fails with the reason `DeadlineExceeded`. An operation created while another one is running fails with the reason `AnotherOperationActive`.

Follow the operation and see what it removed:

```bash
d8 k get registryoperation <NAME> -o jsonpath='{.status}' | jq \
  '{phase, reason, message, garbageCollection}'
```

To cancel an operation, including one a replica is already running, set the cancel label on it:

```bash
d8 k label registryoperation <NAME> registry.deckhouse.io/cancel=true
```

What was deleted before the cancellation stays deleted.

If a replica's `syncer` container stops answering while the replica collects or waits for its turn, for example because its master went `NotReady`, the replica fails with the reason `ReplicaLost`, so that the operation does not hold every collection after it for a day:

- A replica that was collecting, once it has not held the Lease `registry-storage-gc` in the `d8-system` namespace for five minutes.
- A replica that was waiting, once its node has not been `Ready` for ten minutes. A master that is back sooner keeps its replica's turn.

A replica that waits on a node that stays `Ready` while its `syncer` does not answer is released by the cancel label, by the 24-hour timeout (the reason `TimedOut`), or by deleting the operation:

```bash
d8 k delete registryoperation <NAME>
```

### Additional container image registries

[`primary.upstream`](/modules/registry/latest/configuration.html#parameters-primary-upstream) is the only image registry configured in the `registry` ModuleConfig itself. It is used to pull DP component images. If you need additional registries (a vendor's, one needed by some module, or your own), add them using the [RegistryUpstream](/modules/registry/latest/cr.html#registryupstream) custom resource.

An example of adding an additional registry is given in the ["Adding an additional container image registry"](#adding-an-additional-container-image-registry) section.

### Requirements for using the current implementation

Before enabling container image pull path management with the current implementation of the `registry` module, make sure the cluster meets the following requirements:

- Nodes use containerd or containerd v2, set with the [`defaultCRI`](../../../reference/api/cr.html#clusterconfiguration-defaultcri) parameter in [ClusterConfiguration](../../../glossary.html#cluster-configuration).
- The cluster is fully managed by Deckhouse Platform. The current implementation of the module does not work in Managed Kubernetes clusters — use [switching to a third-party registry](third-party.html) instead.

### Examples of configuring the current implementation

{% alert level="warning" %}
If a module's image wasn't pulled again and the module wasn't reinstalled while switching to the current implementation, follow the [instructions](../../../faq.html#what-should-i-do-if-the-module-image-did-not-download-and-the-mo) to resolve the issue.
{% endalert %}

#### Configuring pull path management for Deckhouse Platform component images

To enable pull path management for DP component container images with the `registry` module, enable the module in `Managed` mode and specify the image registry details in [`primary.upstream`](/modules/registry/latest/configuration.html#parameters-primary-upstream). Example of the `registry` ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  version: 1
  enabled: true
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        scheme: HTTPS
        auth:
          license: <LICENSE_KEY> # Replace with your license key.
```

Instead of creating the ModuleConfig from scratch, you can use a ready-made one.
The module publishes a ready-made configuration for your cluster, with the address, path, scheme, certificate authority, and credentials of the registry the cluster is already pulling images from.

To obtain a preconfigured ModuleConfig, run the following command:

```bash
d8 k -n d8-system get secret registry-suggested-config -o jsonpath='{.data.registry-mc\.yaml}' | base64 -d
```

Review it and apply it — this avoids mistakes that are easy to make when retyping these values by hand (a truncated path, the wrong certificate authority, and so on).

To see how the change takes effect, use the commands below.

- View the state of the image registry configuration in Deckhouse Platform:

  ```bash
  d8 k get registryconfig registry -o jsonpath='{.status}' | jq
  ```

- Verify the implementation of the registry configuration on the nodes to ensure that everything has been successfully configured and to determine which backends are currently active:

  ```bash
  d8 k get registrynodes -o custom-columns=\
  NODE:.metadata.name,APPLIED:.status.observedGeneration,OK:.status.reconciled,BACKENDS:.status.activeBackends
  ```

#### Enabling the in-cluster cache

{% alert level="info" %}
The in-cluster cache uses the disks of the master nodes and works in both [static](../../../glossary.html#static-cluster) and [cloud](../../../glossary.html#cloud-cluster) clusters managed by Deckhouse Platform. In a cloud cluster, that is the master node's root disk unless a separate disk is mounted at the cache directory (`/opt/deckhouse/registry`, or `/var/lib/deckhouse/registry` when the master NodeGroup has `systemType: DeckhouseEngine`). Size that disk for [`storage.size`](/modules/registry/configuration.html#parameters-storage-size) and the reserve the store keeps free for the node, on top of what the node itself uses. For details, see ["Requirements for using the module"](/modules/registry/#requirements-for-using-the-module) in the `registry` module documentation.
{% endalert %}

To enable the in-cluster cache, add the [`storage.cache`](/modules/registry/latest/configuration.html#parameters-storage-cache) setting to the `registry` ModuleConfig and specify the storage size:

```yaml
spec:
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        auth:
          license: <LICENSE_KEY> # Replace with your license key.
    storage:
      cache: true
      size: 50Gi
```

Nothing on the nodes is reconfigured: the container runtime already turns to the agent for any registry, and the agent starts using the cache first. The upstream registry is used as a fallback path. So missing data in the cache right after the cache is enabled means a slower pull, not a failed one.

To check how full the in-cluster cache is, use the following command:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Turning the cache off is the same change in reverse, and just as safe. The blobs on disk are left untouched, and when the cache is turned on again, it is refilled from the data already accumulated. If you turn the in-cluster cache off and do not plan to turn it on again, remove the leftover cache data from the cluster nodes. For detailed instructions, see the [`registry` module FAQ](/modules/registry/latest/faq.html#how-do-i-remove-leftover-cache-data-from-a-node).

#### Switching the cluster into an air-gapped state

An isolated cluster has no upstream registry (it is not set in [`primary.upstream`](/modules/registry/latest/configuration.html#parameters-primary-upstream)). For such a cluster, the cache is the only source of images. The [`d8 mirror push`](../../../cli/d8/reference/#d8-mirror-push) command is used to fill the storage. To verify that all required images are present in the cache, specify the expected set of images [in the `storage.source` parameter](/modules/registry/latest/configuration.html#parameters-storage-source)

To move the cluster to air-gapped, follow these steps:

1. Pull the images on a machine that has internet access:

   ```bash
   d8 mirror pull --license <LICENSE_KEY> ./d8-bundle
   ```

1. Push them into the cluster through the publication endpoint:

   ```bash
   PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
   d8 mirror push ./d8-bundle registry.example.com/system/deckhouse \
     --registry-login "$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
     --registry-password "$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
   ```

   The publication endpoint (the `registry` domain built from [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate), `registry.example.com` here) exists only while the cache is enabled, `publicDomainTemplate` is set and the `cert-manager` module is enabled. For details, see ["Air-gapped clusters"](/modules/registry/#air-gapped-clusters) in the `registry` module documentation.

1. Describe the expected image set in the cache and remove the `upstream` from the `registry` ModuleConfig:

   ```yaml
   spec:
     settings:
       mode: Managed
       storage:
         cache: true
         size: 50Gi
         source:
           bundleRef: d8-mirror-bundle
           expectedDigests: 459
   ```

The upstream registry is removed from the nodes not at the moment the configuration is edited, but once the cache leader holds the whole expected image set — otherwise all nodes could be left with no image source. When the upstream is removed, the registry restarts, and the RegistryStorage resource may briefly report `Failed`: this is expected, wait for it to return to `Ready`. To check the status of the transition, use the following commands.

Check whether the cache leader holds the whole expected image set:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{safeToDropUpstream,fill}'
```

Check whether the cluster is isolated (while `effectiveUpstream` is set, the cluster is using it; if it is empty, the cluster is isolated):

```bash
d8 k get registryconfig registry -o jsonpath='{.status.effectiveUpstream}' | jq
```

#### Adding an additional container image registry

{% alert level="info" %}
Requests to additional registries are always routed through the node agent (credentials and the certificate authority are kept in one place), and images from them are not cached.
{% endalert %}

To add an additional image registry (not for DP [system components](../../../glossary.html#system-component)), use the [RegistryUpstream](/modules/registry/latest/cr.html#registryupstream) resource. Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: RegistryUpstream
metadata:
  name: virtualization-images
spec:
  match: images.virtualization.example.com
  upstream:
    host: vendor.example.com
    path: /virtualization
    auth:
      username: robot
      password: <PASSWORD>
```

After creating the RegistryUpstream, check that it has been accepted. The object is rejected in the following cases:

- It conflicts with the primary registry or another object claiming the same `match` value. The object is rejected rather than merged.
- It contains an invalid field, such as a `ca` that is not a PEM certificate.
- Adding the object exceeds either of the following limits:
  - Settings from no more than 256 RegistryUpstream objects can be applied at a time.
  - The combined size of the accepted objects' `spec` fields in JSON format must not exceed 256 KiB.

To check whether the object has been accepted, run the following command:

```bash
d8 k get registryupstreams -o custom-columns=\
NAME:.metadata.name,MATCH:.spec.match,ACCEPTED:.status.conditions[0].status,REASON:.status.conditions[0].reason
```

In the example above, after the additional registry is added, image pull requests for `images.virtualization.example.com` are routed by the agent on every node to `vendor.example.com/virtualization`, with the credentials and certificate authority held by the cluster rather than by each workload individually. Nothing on the nodes is reconfigured for this.

#### Pulling from a private container image registry without declaring it

To load images from a storage module that is unknown to the module, no additional declarations are required. The node agent proxies such requests untouched, along with whatever credentials the request already carried. So an ordinary `imagePullSecret` works exactly as it does in a cluster where the module was never enabled.

To create a secret with the credentials of the private container image registry, run:

```bash
d8 k create secret docker-registry my-private-registry \
  --docker-server=private.example.com \
  --docker-username=robot \
  --docker-password=<PASSWORD>
```

To use this secret when pulling images, specify it in the pod's `imagePullSecrets`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example
spec:
  imagePullSecrets:
  - name: my-private-registry
  containers:
  - name: app
    image: private.example.com/team/app:v1
```

It is worth creating a [RegistryUpstream](/modules/registry/latest/cr.html#registryupstream) for the registry only if you want the cluster to hold the credentials instead of every workload doing so individually, or if the registry needs a certificate authority the nodes do not have.

#### Turning off pull path management

To turn off pull path management, set [`mode: Unmanaged`](/modules/registry/latest/configuration.html#parameters-mode) in the `registry` ModuleConfig:

```yaml
spec:
  settings:
    mode: Unmanaged
```

After `mode: Unmanaged` is set, pull path management through the `registry` module is turned off. The cluster resumes downloading images from the container registry specified in the `deckhouse-registry` secret—that is, from the same source from which images were downloaded before the module was enabled. Cache data on the master nodes is deliberately preserved: when pull path management is turned on again, the cache is topped up only with the images missing from the container image registry rather than filled from scratch. If the cache is no longer needed, free up the space it occupies by following the [instructions](/modules/registry/latest/faq.html#how-do-i-remove-leftover-cache-data-from-a-node) in the `registry` module documentation.

## V1 implementation

{% alert level="warning" %}
The v1 implementation of the `registry` module described below was used in Deckhouse Platform up to and including 1.77. Deckhouse Platform 1.78 no longer contains it. For new clusters, and for existing ones at the first opportunity, use the [current implementation](#current-implementation).
{% endalert %}

In the v1 implementation, Deckhouse Platform supports several management modes for the platform component registry settings: `Unmanaged`, `Direct`, `Proxy`, and `Local`. In `Direct`, `Proxy`, and `Local` modes, a fixed virtual address is used to access the [Deckhouse Platform component registry](../../../glossary.html#component-registry). This eliminates the need to restart all control plane components and re-download images when the platform component registry changes.

In `Unmanaged` mode, no virtual address is used. The external registry is accessed directly: if the registry changes, all Deckhouse Platform components are restarted.

Switching between modes and registries is done via the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). The switch occurs automatically (see the switching examples below for more details).

The architecture of the v1 implementation's modes is described in the ["Registry module"](../../../architecture/deckhouse/registry.html) section.

Features of the Deckhouse Platform component registry settings management modes:

- `Direct`: Provides direct access to an **external** registry via the fixed virtual address `registry.d8-system.svc:5001/system/deckhouse`. The fixed address prevents images from being re-downloaded and components from being restarted when registry parameters change.
- `Proxy`: Uses an **internal caching proxy registry** that accesses an **external** registry. The caching proxy registry runs on control-plane (master) nodes. This mode reduces the number of requests to the external registry by caching images. The internal registry is accessed via the fixed virtual address `registry.d8-system.svc:5001/system/deckhouse`, similar to the `Direct` mode.
- `Local`: Uses a local **internal** registry, with the registry running on control-plane (master) nodes. This mode allows the cluster to operate in an isolated environment. The internal registry is accessed via the fixed virtual address `registry.d8-system.svc:5001/system/deckhouse`, similar to the `Direct` and `Proxy` modes.
- `Unmanaged` (configurable mode): Works without an internal registry. No virtual address is used. The **external** registry is accessed directly.

{% alert level="warning" %}
This document refers to the configurable `Unmanaged` mode (of the v1 implementation of the `registry` module) and the non-configurable `Unmanaged` mode (the fully deprecated format, without the `registry` module).

The non-configurable `Unmanaged` mode does not use the `registry` module at all (for example, in Managed Kubernetes clusters). The Deckhouse Platform component registry configuration parameters are set during cluster installation or, in a deployed cluster, using the `helper change registry` utility (deprecated).
{% endalert %}

### Restrictions on Deckhouse Platform component registry settings

There are a number of restrictions and considerations regarding cluster installation, the v1 implementation's usage conditions, and switching between its modes.

#### Restrictions during cluster installation

The following restrictions apply when installing a cluster:

- A new cluster cannot be installed with the v1 implementation: the installer refuses `Proxy` and `Local` in the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). A `Direct` section there is read only as the registry address, and the cluster is installed in `Unmanaged` mode. To install a cluster whose pull path the `registry` module manages, use the [current implementation](#current-implementation).
- To launch a cluster in the non-configurable `Unmanaged` mode (Legacy, without using the `registry` module), registry parameters must be specified in [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo).

#### Operating conditions restrictions

To manage Deckhouse Platform component registry settings using the v1 implementation, the following conditions must be met:

- CRI containerd or containerd v2 must be used on cluster nodes. To configure CRI, refer to the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) configuration.
- The cluster must be fully managed by Deckhouse Platform. In Managed Kubernetes clusters, managing Deckhouse Platform component registry settings via the `registry` module is not available.

#### Restrictions on mode switching

Mode switching restrictions are as follows:

- Changing registry parameters and switching modes is only available after the bootstrap phase is fully complete.
- For the first switch, you must migrate user registry configurations. For more details, see the ["Registry Module: FAQ"](/modules/registry/faq.html) section.
- Switching to the non-configurable `Unmanaged` mode (without using the `registry` module) is only available from the configurable `Unmanaged` mode. For more details, see the ["Registry Module: FAQ"](/modules/registry/faq.html) section.

### Examples of switching between the v1 implementation's modes

{% alert level="warning" %}
If a module's image wasn't pulled and the module wasn't reinstalled during the switch, follow the [instructions](../../../faq.html#what-should-i-do-if-the-module-image-did-not-download-and-the-mo) to resolve the issue.
{% endalert %}

#### Switching to Unmanaged mode

To switch an already running cluster to `Unmanaged` mode, follow these steps:

{% alert level="danger" %}
Changing the registry in `Unmanaged` mode will restart all Deckhouse Platform components.
{% endalert %}

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Ensure the Deckhouse Platform queue is empty and has no errors:

   ```shell
   d8 system queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Set the `Unmanaged` mode settings in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged). If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module configuration for correct setup.

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
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key.
   ```

1. Check the registry mode switch status in the `registry-state` secret using [the instruction](#checking-the-registry-mode-switch-status-in-the-v1-implementation).

   Example output:

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

1. If you need to disable automatic Deckhouse Platform updates from the configured image registry, remove the `releaseChannel` parameter from the `deckhouse` module configuration.
   After that, automatic platform updates will be disabled, and you will need to manage the Deckhouse Platform version manually.

If you need to switch to the legacy registry management method (without the `registry` module), see the [instruction](#migration-to-the-deprecated-registry-management-format-without-the-registry-module).

{% alert level="warning" %}
Managing the Deckhouse Platform component image registry without the `registry` module is a deprecated management format.
{% endalert %}

## Migrating from the v1 implementation to the current one

This section describes the transition from the v1 implementation of the `registry` module to the current one. A cluster that does not use the `registry` module at all (the fully deprecated format) has nothing to hand over, since the v1 implementation never configured its nodes: it needs no preparation for the upgrade to Deckhouse Platform 1.78 and keeps pulling images from the registry it was installed with.

The migration is the handover of the pull path from the v1 implementation to the current one.
It does not need to be started manually, and there is no separate command for it: the `registry` module takes over control automatically after the cluster is upgraded to Deckhouse Platform 1.78. The only condition is that by then the v1 implementation must have released the pull path. Both implementations configure the same thing on every node — which container image registry the container runtime pulls images from and with which credentials — so they never manage a cluster at the same time.

### Determining the migration method and steps

What needs to be done to hand over pull path management to the current implementation of the `registry` module, and when, depends on the mode the v1 implementation is running in. The mode is set in the `settings.registry.mode` parameter of the `deckhouse` ModuleConfig. To view the mode, use the following command:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.registry.mode}'
```

Empty output means `Unmanaged` mode: the registry settings of this cluster have never been set. Such a cluster needs no preliminary actions for the migration — the handover happens automatically.

The actions required to migrate from the v1 implementation to the current one are described in the table:

| Mode of the v1 implementation | What to do | When |
|---|---|---|
| `Unmanaged` | [Nothing](#migrating-from-unmanaged-mode) — the handover happens on its own | — |
| `Direct` | [Configure the `registry` ModuleConfig](#migrating-from-direct-mode): set `mode: Managed` and `primary.upstream` | Before upgrading to Deckhouse Platform 1.78 |
| `Proxy` | [Switch the cluster to `Unmanaged`](#migrating-from-proxy-mode) | Before upgrading to Deckhouse Platform 1.78 |
| `Local` | Follow the separate [procedure for air-gapped clusters](#migrating-an-air-gapped-cluster-from-local-mode) | Before upgrading to Deckhouse Platform 1.78 |

{% alert level="warning" %}
Deckhouse Platform 1.78 no longer contains the v1 implementation of the module. The cluster switches to the current implementation after the upgrade to Deckhouse Platform 1.78, and the preparation for that is done before the upgrade, on Deckhouse Platform 1.77.
{% endalert %}

{% alert level="danger" %}
Prepare your cluster in advance for the transition to the current implementation. The update to Deckhouse Platform 1.78, which no longer contains the v1 implementation, is blocked as long as the cluster is running in `Proxy` or `Local` mode of the v1 implementation, or in `Direct` mode without a configured `registry` in ModuleConfig.
{% endalert %}

When the v1 implementation is used in `Direct` mode, no mode switch is needed. The `registry` ModuleConfig settings are deliberately accepted already in Deckhouse Platform 1.77: written before the upgrade, they are stored without effect and take effect on the module's first reconciliation after the upgrade.

### Checking which implementation the cluster uses

When the `registry` module takes over pull path management, it records that in the `registry-v2-switch` secret. Check whether the secret exists:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "current implementation" || echo "v1 implementation"
```

If the cluster is still on the v1 implementation, the module reports the reason on every reconciliation, and the [`D8RegistryMigrationPending`](../../../reference/alerts.html#registry-d8registrymigrationpending) alert fires in the cluster. To see which actions are required, use the following command:

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

### Migrating from Unmanaged mode

In `Unmanaged` mode, the v1 implementation does not manage the pull path, so no preliminary preparation for switching to the current implementation is required — the handover happens automatically.

The migration procedure to the current implementation in this case is as follows:

1. If the cluster on the v1 implementation is being switched to `Unmanaged` from another mode, wait for the transition to complete. The status must show `mode: Unmanaged` with no pending target mode.

   To check, use the following command:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to Deckhouse Platform 1.78 (or, if it has already been upgraded, wait for the next reconciliation). The module takes over image pull path management automatically, and the pull source does not change. The default mode of the `registry` module in the current implementation is also `Unmanaged`, so the cluster keeps pulling images from the same registry as before. On containerd v1 nodes, however, the takeover restarts containerd node by node if the v1 implementation left a node registry configuration, and enabling `mode: Managed` in the next step restarts it (a second time, if the takeover already did; for details, see ["Does the migration restart containerd?"](/modules/registry/faq.html#does-the-migration-restart-containerd)).

1. To have the module manage the pull paths, set [`mode: Managed`](/modules/registry/configuration.html#parameters-mode) in the `registry` ModuleConfig and specify the registry to pull images from. A ready-made configuration for your cluster is published in the `registry-suggested-config` secret. For a configuration example, see the ["Configuring pull path management for Deckhouse Platform component images"](#configuring-pull-path-management-for-deckhouse-platform-component-images) section.

### Migrating from Direct mode

In `Direct` mode of the v1 implementation, nodes pull images through an in-cluster address served by the v1 implementation's proxy. The current implementation serves the same address, so the migration is a direct handover of the address: no switch through `Unmanaged` is needed, and no components are restarted.

The migration procedure to the current implementation in this case is as follows:

1. Configure the `registry` ModuleConfig before the upgrade — without this configuration the upgrade is blocked. To fill in the `registry` ModuleConfig, take the values from the [`registry.direct` section](/modules/deckhouse/configuration.html#parameters-registry-direct) of the `deckhouse` ModuleConfig:
   - split the `imagesRepo` value from the `deckhouse` ModuleConfig into `host` and `path` and specify them in the corresponding fields of the `registry` ModuleConfig;
   - use the credentials specified in the `deckhouse` ModuleConfig: the same `license` (or `username`/`password`), and `ca` if the registry requires one.

   Example of the `registry` ModuleConfig for the switch:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: true
     version: 1
     settings:
       mode: Managed
       primary:
         upstream:
           scheme: HTTPS
           host: registry.deckhouse.io
           path: /deckhouse/ee
           auth:
             license: <LICENSE_KEY> # Replace with your license key.
   ```

   The v1 implementation stores these settings but does not act on them, so nothing changes in the cluster until the upgrade.

1. Upgrade the cluster to Deckhouse Platform 1.78. The handover of container image pull path management happens on the next reconciliation, and image pulls keep working throughout: the Service and the proxy of the v1 implementation keep serving the in-cluster address until the agent of the current implementation takes it over on every node, and only then are they removed.

1. Track the progress of the handover using the following commands.

   - Check whether path management has been handed over to the current implementation:

     ```bash
     d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 && echo "handed over"
     ```

   - Check the state of the image registry nodes:

     ```bash
     d8 k get registrynode -o custom-columns='NODE:.metadata.name,READY:.status.reconciled,SERVING:.status.proxyListening'
     ```

   The handover is recorded a few minutes after the new version starts, the agent appears on the nodes a few minutes later, and the objects of the v1 implementation are removed shortly after that. Image pulls keep working at each of these stages.

### Migrating from Proxy mode

When the v1 implementation is used in `Proxy` mode, a proxy of its own, with its own certificates, is deployed on every node. The current implementation cannot adopt this state. So the cluster must first be switched to `Unmanaged`, and this can only be done before the upgrade.

The migration procedure to the current implementation in this case is as follows:

1. In the `deckhouse` ModuleConfig, set `registry.mode: Unmanaged`, keeping the same registry address and credentials (for an example of the `deckhouse` ModuleConfig, see the ["Switching to Unmanaged mode"](#switching-to-unmanaged-mode) section). All nodes are reconfigured to pull directly from the external registry, so the caching provided in `Proxy` mode is lost until the last step.

1. Wait for the transition to complete — `mode: Unmanaged` with no pending target mode. A cluster caught mid-transition cannot be migrated; the status shows which mode it is still switching to:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to Deckhouse Platform 1.78. The handover happens on the next reconciliation, and the pull source does not change: in `Unmanaged` mode, the current implementation does not manage the pull path either. On containerd v1 nodes, however, the takeover restarts containerd node by node, and enabling `mode: Managed` in the next step restarts it again (for details, see ["Does the migration restart containerd?"](/modules/registry/faq.html#does-the-migration-restart-containerd)).

1. To get in-cluster caching back, set [`mode: Managed`](/modules/registry/configuration.html#parameters-mode) with [`storage.cache: true`](/modules/registry/configuration.html#parameters-storage-cache) and the same external registry. Note that the cache of the current implementation is built differently from `Proxy` mode of the v1 implementation: a single store with replicas on the [master nodes](../../../glossary.html#master-node) instead of a proxy on every node. Before enabling it on a cluster with little free disk space on the master nodes, read [how the cache is filled and cleaned up](/modules/registry/faq.html#the-cache-keeps-growing-what-reclaims-it).

### Migrating an air-gapped cluster from Local mode

In `Local` mode of the v1 implementation, the cluster has no external image registry — its role is played by a registry inside the cluster itself. So the standard migration procedure does not work here, since it goes through `Unmanaged` mode, where every node pulls images directly from an external registry, and such a cluster has nowhere to pull them from.

An external registry has to be connected to the cluster for the duration of the migration. It runs in the same cluster, but outside the Deckhouse Platform namespaces. The cluster is switched to it and upgraded, and once the registry of the current implementation is filled with images, the temporary registry is removed.

{% alert level="warning" %}
Before starting, make sure the master nodes have free disk space for **four image sets**: at the peak of the migration, three copies of the set exist at the same time — the `Local` store, the temporary registry, and the registry of the current implementation being filled — and the space for the fourth set is headroom the node must not run out of. The detailed disk space calculation and the verification procedure are described in the ["Disk requirements"](/modules/registry/faq.html#disk-requirements) section of the `registry` module documentation.
{% endalert %}

Migration procedure:

1. Run a temporary external registry in its own namespace, outside Deckhouse Platform management. Any container image registry implementation will do, but it must serve TLS with a certificate the cluster can verify, and Deckhouse Platform must not manage it: whatever happens to the module's objects, the temporary container image registry must keep working.

   In addition, the container image registry must be reachable **at the same address from two places**: in step 3, nodes pull images from it, and in step 6, the `registry` module's syncer running in a pod reads it. The certificate must cover the chosen address. Options tested on a single cluster:

   | Address | From a node | From a pod |
   |---|---|---|
   | A `hostNetwork` port on the node's own IP (`<NODE_IP>:5000`) | works | works |
   | A Service name (`<SERVICE>.<NAMESPACE>.svc:<PORT>`) | does not resolve | works |
   | A NodePort on the node's IP | works | does not work: `operation not permitted` |

   Use the first option: run the container image registry with `hostNetwork: true` on one node, address it by that node's IP, and add that IP to the certificate's SAN — then the whole migration goes through a single address. A Service name looks neater, but stops working in step 3, because the node's container runtime does not resolve names through the cluster DNS.
1. Load the image set of the current version into the temporary registry: `d8 mirror pull` on a machine with internet access, then `d8 mirror push` into the temporary registry.
1. In the `deckhouse` ModuleConfig, set `registry.mode: Unmanaged` with the address, CA certificate, and credentials of the temporary registry. The data of the `Local` store stays where it is, in the `/opt/deckhouse/registry` directory on the master nodes.
1. Make sure the transition is complete and images are really pulled from the temporary registry (`mode: Unmanaged` with no pending target mode in the `registry-state` secret):

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to Deckhouse Platform 1.78. The handover happens on the next reconciliation, and the cluster keeps pulling images from the temporary registry throughout. On containerd v1 nodes the takeover restarts containerd node by node, and enabling `mode: Managed` in the next step restarts it again (see [Does the migration restart containerd?](/modules/registry/latest/faq.html#does-the-migration-restart-containerd)).
1. Enable the current implementation: set [`mode: Managed`](/modules/registry/latest/configuration.html#parameters-mode) with [`storage.cache: true`](/modules/registry/latest/configuration.html#parameters-storage-cache), specify the temporary registry as [`primary.upstream`](/modules/registry/latest/configuration.html#parameters-primary-upstream) — **and add [`storage.source`](/modules/registry/latest/configuration.html#parameters-storage-source) in the same edit** (without it, the next step is rejected). The registry of the current implementation uses the same host path that `Local` used, so the images already on disk are not downloaded again.
1. Wait until the storage reports that it holds the whole image set (`phase: Ready` and `safeToDropUpstream: true`), and remove `primary.upstream` from the `registry` ModuleConfig. The cluster is isolated again — now on the current implementation:

   ```bash
   d8 k get registrystorage registry -o jsonpath='{.status.phase} {.status.safeToDropUpstream}{"\n"}'
   ```

   Removing the upstream restarts the registry, and the RegistryStorage resource may briefly report `Failed`: this is expected, wait for it to return to `Ready`.

1. Delete the temporary registry and reclaim its disk space.

## Migration to registry management format using the registry module

{% alert level="info" %}
This procedure applies to Deckhouse Platform 1.77 and earlier only: it moves a cluster from the deprecated format (without the `registry` module, configured only via `InitConfiguration`) to the **v1 implementation** of the module, which Deckhouse Platform 1.78 no longer contains. It is not a step of the migration to the current implementation: a cluster in the deprecated format needs no preparation for the upgrade to Deckhouse Platform 1.78. See the ["Migrating from the v1 implementation to the current one"](#migrating-from-the-v1-implementation-to-the-current-one) section.
{% endalert %}

During the migration, containerd v1 will be switched to the new registry configuration scheme.
containerd v2 uses the new scheme by default. For more details, see the section [describing configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry).

To view the container runtime type used by default on cluster nodes (in NodeGroups), use the following command:

```shell
d8 system edit cluster-configuration
```

The container runtime type is specified in the `defaultCRI` parameter.

If containerd v2 is used as the container runtime, follow the [containerd v2](#for-containerd-v2) migration instructions.

If containerd v1 is used as the container runtime, you can choose one of the following methods to migrate to the `registry` module’s image registry management format:

- Without switching to containerd v2. In this case, follow the instructions [for containerd v1](#for-containerd-v1).
- With switching to containerd v2. In this case:
  
  1. Verify whether you can [switch to containerd v2](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/migrating.html), and if so, perform the switch.
  1. After migrating, follow the instructions [for containerd v2](#for-containerd-v2).

### For containerd v2

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the `deckhouse` ModuleConfig. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/configuration.html) module configuration for correct setup.

   You can view the current registry settings using the following command:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Specify these settings when configuring the `Unmanaged` mode:

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
           license: <LICENSE_KEY> # Replace with your license key.
   ```

1. Wait for the switch to complete. Example [status output](#checking-the-registry-mode-switch-status-in-the-v1-implementation):

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

If you need to add configurations for an additional image repository, refer to the section ["New way to add configuration for an additional registry"](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/node-customization.html#new-method).

### For containerd v1

{% alert level="danger" %}

- During the switch, containerd v1 will be restarted.
- During the switch, containerd v1 will be migrated to the new [Deckhouse Platform component registry](../../../glossary.html#component-registry) configuration scheme.
- During the switch, [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) for containerd v1 will be temporarily unavailable.
{% endalert %}

1. Make sure that nodes with containerd v1 do not have any [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) located in the `/etc/containerd/conf.d` directory.

1. If configurations are present, you need to migrate to the new registry configuration format in containerd. To do this, add new configuration files to the `/etc/containerd/registry.d` directory. These configurations will take effect after switching to the `registry` module. To add configurations, prepare a NodeGroupConfiguration. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry). Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # The step can be arbitrary, as restarting the containerd service is not required.
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
   # For HTTPS.
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag

   # For HTTP.
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the `deckhouse` ModuleConfig. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/configuration.html) module configuration for correct setup.

   You can view the current registry settings using the following command:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Specify these settings when configuring the `Unmanaged` mode:

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
           license: <LICENSE_KEY> # Replace with your license key.
   ```

1. After applying, wait for the following message in the [switch status](#checking-the-registry-mode-switch-status-in-the-v1-implementation):

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
     # This step must be completed before '032_configure_containerd.sh'.
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

1. After removing the old configurations, make sure that the switch has resumed. Example of the [switch status](#checking-the-registry-mode-switch-status-in-the-v1-implementation):

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Wait for the switch to complete. Example of the [switch status](#checking-the-registry-mode-switch-status-in-the-v1-implementation):

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

## Migration to the deprecated registry management format (without the registry module)

{% alert level="danger" %}

- This is a deprecated [Deckhouse Platform component registry](../../../glossary.html#component-registry) management format.
- During the switch, containerd v1 will be restarted.
- During the switch, containerd v1 will be migrated to the legacy registry configuration scheme.
- During the switch, [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) for containerd v1 will be temporarily unavailable.
{% endalert %}

1. Switch the `registry` module to configurable `Unmanaged` mode. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module configuration for correct setup.

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
           license: <LICENSE_KEY> # Replace with your license key.
   ```

1. Check the switch status using the [instruction](#checking-the-registry-mode-switch-status-in-the-v1-implementation). Example output:

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

1. Switch the registry to the non-configurable `Unmanaged` mode (without `registry` module). Example configuration:

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

1. Check the switch status using the [instruction](#checking-the-registry-mode-switch-status-in-the-v1-implementation). Example output:

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
     # To add a file before the '032_configure_containerd.sh' step.
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
                 auth = "<BASE64_AUTH_STRING>"
       EOF
   ```

1. Delete the `registry-bashible-config` secret. This will trigger containerd v1 to switch back to the legacy registry format:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. After deletion, wait for the switch to complete. Use the [instruction](#checking-the-registry-mode-switch-status-in-the-v1-implementation) to track the progress. Example output:

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

1. If containerd v1 is used, apply the previously prepared NodeGroupConfiguration with custom registry configurations.

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

## Checking the registry mode switch status in the v1 implementation

The status of the [Deckhouse Platform component registry](../../../glossary.html#component-registry) mode switch in the v1 implementation can be retrieved using the following command:

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
| `ContainerdConfigPreflightReady`  | State of the containerd configuration [preflight check](../../../glossary.html#preflight-checks). Verifies there are no custom containerd auth configurations on the nodes                                                                                           |
| `TransitionContainerdConfigReady` | State of preparing the containerd configuration for the new mode. Verifies that the configuration contains both the old and new mode settings                                                                             |
| `FinalContainerdConfigReady`      | State of finalizing the switch to the new containerd mode. Verifies that the containerd configuration has been successfully applied and contains only the new mode settings                                               |
| `DeckhouseRegistrySwitchReady`    | State of switching Deckhouse Platform and its components to use the new registry. `True` means Deckhouse Platform successfully switched and is ready to operate                                                                             |
| `InClusterProxyReady`             | State of In-Cluster Proxy readiness. Checks that the In-Cluster Proxy has started successfully and is running                                                                                                             |
| `CleanupInClusterProxy`           | State of cleaning up the In-Cluster Proxy if it is not needed in the selected mode. Verifies that all related resources have been removed                                                                                 |
| `NodeServicesReady`               | State of Node Services Manager and Static-Pod registry readiness. Verifies that the Node Services Manager is successfully launched and operational, and that the Static-Pod registry has been successfully deployed by it |
| `CleanupNodeServices`             | State of cleaning up the Node Services Manager and Static-Pod registry if they are not needed in the selected mode. Verifies that all related resources have been removed                                                 |
| `RegistryContainsRequiredImages`  | State of checking the registry for the presence of required images                                                                                                                                                        |
| `Ready`                           | Overall state of registry readiness in the selected mode. Indicates that all other conditions are met and the module is ready to operate                                                                                  |
