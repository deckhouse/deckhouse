---
title: "Registry module: FAQ"
description: "Frequently asked questions about the Deckhouse Kubernetes Platform registry module: migrating to the module, cache maintenance, and troubleshooting registry issues."
---

## How does the migration to the registry module work?

There are two ways a Deckhouse Kubernetes Platform (DKP) cluster's image pull path can be managed:

- **the previous implementation** — the `registry` section of the
  [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry), with the
  modes `Unmanaged`, `Direct`, `Proxy`, and `Local`;
- **the current implementation** — this module, configured through the `registry` ModuleConfig.

The migration is the handover of the pull path from the previous implementation to the module.
It is not started by hand and has no command of its own: the module takes over automatically
after the cluster is upgraded to the release that ships it. The only condition is that by then
the previous implementation must have released the pull path. Both implementations configure
the same thing on every node — which registry the container runtime pulls from and with which
credentials — so they never manage a cluster at the same time.

What has to be done for the handover, and when, depends on the mode the previous implementation
is running in. The mode is set in the `settings.registry.mode` parameter of the `deckhouse`
ModuleConfig:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.registry.mode}'
```

Empty output means `Unmanaged` mode: the registry settings of this cluster have never been
touched. Such a cluster — like any other cluster in `Unmanaged` — needs no action for the
migration: the handover happens on its own.

| Mode | What to do | When |
|---|---|---|
| `Unmanaged` | [Nothing](#how-do-i-migrate-from-unmanaged-mode) — the handover happens on its own | — |
| `Direct` | [Configure the module](#how-do-i-migrate-from-direct-mode): `mode: Managed` with `primary.upstream` | Before the upgrade |
| `Proxy` | [Switch the cluster to `Unmanaged`](#how-do-i-migrate-from-proxy-mode) | Before the upgrade |
| `Local` | [Follow the procedure for air-gapped clusters](#how-do-i-migrate-an-air-gapped-cluster-from-local-mode) | Before the upgrade |

{% alert level="danger" %}
Prepare the cluster **before** the upgrade: upgrading to the release with the module is blocked
while the cluster is running in `Proxy` or `Local` mode, or in `Direct` mode without a
configured `registry` ModuleConfig. The new release contains neither the components of the
previous implementation nor the code that switches its modes, so all preparation happens on the
current release.
{% endalert %}

`Direct` mode needs no mode switch. The module's settings are deliberately accepted one release
early: written before the upgrade, they are stored without effect and take effect on the
module's first reconciliation after it.

The cluster records which implementation is actually running (see
[the next question](#which-implementation-is-my-cluster-running)). While the handover is
impossible, the upgrade to the release with the module is blocked, and the error message says
what to do.

## Which implementation is my cluster running?

When the module takes over the pull path, it records that in the `registry-v2-switch` secret.
Check whether the secret exists:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "current implementation" || echo "previous implementation"
```

If the cluster is still on the previous implementation, the module reports the reason on every
reconciliation and fires the
[`D8RegistryMigrationPending`](#what-do-the-registry-alerts-mean) alert. To see what it is
waiting for:

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

## How do I migrate from Unmanaged mode?

In `Unmanaged` mode the previous implementation does not manage the pull path, so there is
nothing to prepare: the handover happens on its own.

1. If the cluster is switching to `Unmanaged` from another mode, wait for the transition to
   complete. The status must show `mode: Unmanaged` with no pending target mode:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to the release with the module (or, if it is already on it, just wait
   for the next reconciliation). The module takes over automatically, and the behavior does not
   change: the module's default mode is also `Unmanaged`, so the cluster keeps pulling from the
   same registry as before.

1. To have the module manage the pull path, set `mode: Managed` in the `registry` ModuleConfig
   and specify the registry to pull from. A ready-made configuration for your cluster is
   published in the `registry-suggested-config` secret — see
   [enabling the module](examples.html#enabling-the-module).

## How do I migrate from Direct mode?

In `Direct` mode the nodes pull images through an in-cluster address served by the previous
implementation's proxy. The module serves the same address, so the migration is a direct
handover of that address: no switch through `Unmanaged` is needed, and no components restart.

1. Configure the module before the upgrade — without this configuration the upgrade is
   blocked. Take the values from the
   `registry.direct` section of the `deckhouse` ModuleConfig: `imagesRepo` splits into `host`
   and `path`, the credentials are the same `license` (or `username`/`password`), plus `ca` if
   the registry requires one:

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
             license: <LICENSE_KEY>
   ```

   The previous release stores these settings without acting on them, so nothing changes in the
   cluster until the upgrade.

1. Upgrade the cluster. The handover happens on the module's next reconciliation, and pulls
   keep working throughout: the Service and the proxy of the previous implementation keep
   serving the in-cluster address until the module's agent has taken it over on every node, and
   only then does the controller remove them.

1. Track the progress:

   ```bash
   d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 && echo "handed over"
   d8 k get registrynode -o custom-columns='NODE:.metadata.name,READY:.status.reconciled,SERVING:.status.proxyListening'
   ```

   For reference, timings measured on a test cluster: the handover was recorded about two
   minutes after the new version started, the node agent appeared on the nodes about seven
   minutes later, and the objects of the previous implementation were removed a minute after
   that. Pulls worked at every point in between.

## How do I migrate from Proxy mode?

`Proxy` mode keeps a proxy of its own, with its own certificates, on every node — state the
module cannot adopt. So the cluster must first be switched to `Unmanaged`, and this can only be
done before the upgrade.

1. In the `deckhouse` ModuleConfig, set `registry.mode: Unmanaged`, keeping the same registry
   address and credentials — see
   [the mode switching examples](examples.html#examples-for-the-previous-implementation). All
   nodes are reconfigured to pull directly from the external registry, so the caching `Proxy`
   provided is lost until you bring it back in step 4.

1. Wait for the transition to complete — `mode: Unmanaged` with no pending target mode. A
   cluster caught mid-transition cannot be migrated, and the status shows which mode it is
   still heading to:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster. The handover happens on the module's next reconciliation and does not
   change the behavior: in `Unmanaged` mode the module does not manage the pull path either.

1. To get in-cluster caching back, set `mode: Managed` with `storage.cache: true` and the same
   upstream registry. Note that the module's cache is built differently from `Proxy`: a single
   store with replicas on the master nodes instead of a proxy on every node. Before enabling it
   on a cluster with little free disk space on the control-plane nodes, read
   [how the cache is filled and cleaned up](#the-cache-keeps-growing-what-reclaims-it).

## How do I migrate an air-gapped cluster from Local mode?

In `Local` mode the cluster's registry lives inside the cluster itself, and there is no
external one. The standard procedure does not apply here: it goes through `Unmanaged`, where
every node pulls directly from an external registry — and this cluster has none.

The way through is to give the cluster an external registry temporarily: run it in the same
cluster, but outside the DKP namespaces; switch the cluster to it; upgrade; let the
module's storage fill up; then remove it. The procedure has been verified end to end on a test
cluster.

### Disk requirements

Before starting, make sure the control-plane nodes have free space for **four image sets**.
Three copies exist at the peak — the `Local` store, the temporary registry, and the module's
storage being filled — and the fourth set is headroom the node must not run out of.

For reference, measured on a test migration: a 13 GiB image set (platform with modules) took
21 GiB in the temporary registry (two releases: the one the cluster ran and the one it moved
to) and grew the store from 13.0 to 21.4 GiB — in addition to the node's own image cache and
the system. On a 100 GiB control-plane node the peak usage was 38 GiB; on a 50 GiB node the
same migration ran into pod eviction.

The headroom is critical. When free disk space on a control-plane node runs low, kubelet
starts evicting pods and deleting images it considers unused — and in a cluster whose registry
runs inside it, the deleted image can be the registry's own. The store is then left with no
process to serve it, and the node with nowhere to pull that image from. In the test this
deadlock lasted 99 minutes and was resolved only by loading images onto the node manually.
Plan the disk space in advance and monitor it while the migration runs.

The images already on the control-plane disks are adopted by the module's storage — verified
in place rather than downloaded again (see step 6). What adoption cannot do is merge two
different releases into one set: the store ends up holding both the release the old cluster
ran and the one the new cluster runs, which is where the peak size comes from.

### Procedure

Steps 1–4 are performed before the upgrade, on the release that still contains the previous
implementation.

1. Run a temporary OCI registry in a namespace of your own. Any registry implementation will
   do, but it must serve TLS with a certificate the cluster can verify. DKP must not
   manage it: whatever happens to the module's own objects, the temporary registry must keep
   working.

   The registry must be reachable **at one address from two places**: in step 3 the nodes pull
   from it, and in step 6 the module's syncer reads it from inside a pod. The certificate must
   cover the chosen address. Options tested on one cluster:

   | Address | From a node | From a pod |
   |---|---|---|
   | `hostNetwork` port at the node's own IP (`<NODE_IP>:5000`) | works | works |
   | Service name (`<SERVICE>.<NAMESPACE>.svc:<PORT>`) | does not resolve | works |
   | NodePort at a node's IP | works | fails: `operation not permitted` |

   Use the first option: run the registry with `hostNetwork: true` on one node, address it by
   that node's IP, and include the IP in the certificate's SAN. The whole migration then stays
   on a single address. A Service name looks tidier but breaks in step 3: the container
   runtime on a node does not resolve cluster DNS.

1. Load the image set into the temporary registry: run `d8 mirror pull` on a machine with
   access to the DKP registry, then `d8 mirror push` into the temporary one. This is the
   copy the disk budget above accounts for.

1. Point the previous implementation at the temporary registry and switch it to `Unmanaged`:
   in the `deckhouse` ModuleConfig, set `registry.mode: Unmanaged` together with the address,
   the CA certificate, and the credentials of the temporary registry. The nodes start pulling
   from it. The `Local` store leaves the pull path, but its data stays where it is — under
   `/opt/deckhouse/registry` on the control-plane nodes.

1. Make sure the transition is complete and images really are pulled from the temporary
   registry — the rest of the migration rests on this step. The checks are the same as for any
   `Unmanaged` cluster:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to the release with the module. The handover happens on the module's
   next reconciliation; the cluster keeps pulling from the temporary registry throughout.

1. Enable the module: `mode: Managed` with `storage.cache: true`, the temporary registry as
   `primary.upstream` — **and `storage.source` in the same edit**. The module's storage starts
   on the same host path the `Local` store used, so the images already on the disks are
   adopted: the fill verifies them and downloads only what is missing.

   Do not postpone `storage.source`: without it, the upstream cannot be removed in the next
   step. A `Managed` configuration without `primary.upstream` is only accepted when
   `storage.source` is set, so the next step would be rejected with
   `'storage.source' is required when 'primary.upstream' is not set`.

   In `storage.source`, `bundleRef` is a name for the image set, and `expectedDigests` is the
   number of distinct digests in it. Count them in the bundle you pushed:

   ```bash
   for tar in <BUNDLE_DIR>/*.tar; do tar -xOf "$tar" --wildcards '*index.json'; done |
     jq -r '.manifests[]?.digest' | sort -u | wc -l
   ```

   Changing these settings restarts the registry process, so the RegistryStorage resource reports
   `Failed` for about a minute with an error about reading its own store. This is expected;
   wait for it to return to `Ready`.

1. Wait until the storage reports that it holds the whole set — `phase: Ready` with
   `safeToDropUpstream: true` — and remove `primary.upstream` from the `registry` ModuleConfig.
   The cluster is air-gapped again, now on the module:

   ```bash
   d8 k get registrystorage registry -o jsonpath='{.status.phase} {.status.safeToDropUpstream}{"\n"}'
   ```

1. Delete the temporary registry and reclaim its disk space.

## What do the registry alerts mean?

None of these alerts mean that the cluster has stopped pulling images. Most of them report a
state where everything works, but not the way the configuration asks — a state that is easy to
miss otherwise.

`D8RegistryMigrationPending`
: The cluster is still running the previous implementation. Nothing is degraded; the migration
  is not complete. See [how the migration works](#how-does-the-migration-to-the-registry-module-work).

`D8RegistryConfigInvalid`
: The configuration was rejected; the cluster keeps working with the previous one. The reason
  is in `.status.conditions` of the `registryconfig/registry` resource.

`D8RegistryNodeNotConverged`
: The agent on some nodes has not applied the configuration it was given. Those nodes keep
  pulling with their old configuration, and further changes will not reach them either.

`D8RegistryNodeRunningFromDisk`
: Some nodes cannot reach the API server and route pulls using the copy of the configuration
  stored on disk. This fallback works as designed: pulls on those nodes succeed, so nothing
  else would report the problem — meanwhile their configuration can lag arbitrarily far behind
  the cluster's.

`D8RegistryStorageIncomplete`
: Some cache replicas do not hold the whole expected image set. While an upstream is
  configured this does not affect pulls, but the cluster would not survive losing the
  upstream — and this is what blocks a transition to air-gap.

`D8RegistryAirGapTransitionHeld`
: The upstream was removed from the configuration, but the module keeps using it, because the
  cache cannot serve the cluster alone yet. This is the safe outcome: dropping the upstream
  with an incomplete cache would leave the nodes with nowhere to pull from. The alert does not
  resolve on its own if the cache has stopped filling.

`D8RegistryUpstreamProbeFailing`
: A change to the primary upstream was rejected, and the cluster keeps using the last working
  one. The `outcome` label tells the problems apart: `unreachable` — network or the registry
  itself; `auth` — usually an expired license key; `sentinel` — the registry responded and
  accepted the credentials, but does not contain the DKP images (usually a wrong
  repository path).

`D8RegistryUpstreamRejected`
: A RegistryUpstream resource was not accepted, so pulls for the registry it names are not
  intercepted. The `reason` label says whether it conflicts with the primary registry or with
  another resource claiming the same name.

`D8RegistryStorageNotReclaimed`
: No replica has run garbage collection for a week. Collection is the only thing that removes
  data from the store, so if it has stopped, the disk will eventually fill up. See
  [what cleans the cache up](#the-cache-keeps-growing-what-reclaims-it).

`D8RegistryStaleCacheData`
: A node holds cache data that nothing uses. See
  [how to remove it](#how-do-i-remove-leftover-cache-data-from-a-node).

## How do I remove leftover cache data from a node?

When the cache is turned off, the data under `/opt/deckhouse/registry` is intentionally kept:
if the cache is turned back on, it refills from what is already on disk instead of downloading
everything again — over a slow link that saves hours. Deleting the data automatically would
make turning the cache off irreversible, so the module leaves the decision to you: the agent
measures the leftover data and fires
[`D8RegistryStaleCacheData`](#what-do-the-registry-alerts-mean).

Check how much space the data takes:

```bash
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,STALE:.status.staleStorageDataBytes
```

If you are not going to turn the cache back on, remove the directory on the node:

```bash
ssh <NODE> 'du -sh /opt/deckhouse/registry && sudo rm -rf /opt/deckhouse/registry'
```

## The cache keeps growing. What reclaims it?

Garbage collection, run on a schedule by the storage replicas themselves.

It is the only mechanism that removes anything from the store. Every DKP release adds new
images, so without collection the store of a long-lived cluster eventually fills up and stops
accepting writes — and an air-gapped cluster with a full store cannot be updated.

Collection removes the images of the releases the cluster has moved past. It keeps:

- the deployed release and the previous one, so that a rollback does not have to re-download
  anything;
- everything newer than the deployed release — an update in progress or, in an air-gapped
  cluster, a release pushed on purpose;
- every tag that is not a version: release channel names such as `stable`, floating tags,
  anything pushed by hand. The collector cannot know what these mean, so it does not touch
  them.

The collector is deliberately cautious. In an air-gapped cluster, deleting a blob that is
still needed is unrecoverable without another `d8 mirror push`, while keeping an unneeded one
merely costs disk space. So a run that cannot determine what to keep — for example, when no
deployed release is found — does nothing at all.

To check the collection status and schedule:

```bash
d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
  'map({node, collectedAt, collectionError})'
d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
```

### When it runs, and why a replica goes read-only

The registry's collector first computes the set of reachable blobs and then deletes the rest,
so a blob uploaded between those two steps would be deleted. Collection is therefore only safe
on a store nothing is writing to, and the replica rejects writes for the duration.

The replica keeps serving all the images it holds. What it cannot do while collecting:

- store the result of a cache miss — the node agent falls back to the upstream, so the pull is
  slower, not failed;
- accept a `d8 mirror push` — the push fails with a visible error and can be retried.

Only one replica collects at a time; the others work normally.

By default, collection is scheduled at a night hour — or, if the `master` node group has a
maintenance window, at its start, since that hour is already declared safe for disruption. To
set your own schedule:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        schedule: "0 2 * * Sun"
```

A cron expression that cannot be parsed is rejected rather than guessed at: collecting at an
unexpected hour is worse than not collecting.

### Turning it off

Garbage collection is turned off in the module settings:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        enabled: false
```

This only makes sense with a disk large enough that unbounded growth of the store never
becomes a problem. Note that
[`D8RegistryStorageNotReclaimed`](#what-do-the-registry-alerts-mean) still fires a week after
the last collection: from the outside, "turned off" and "silently stopped" look the same.

## A pull is failing on a node. Where do I look?

Start with the agent: it is on the path of every pull on the node. It runs as a static pod, so
it is available even when the cluster is not.

The agent's logs:

```bash
d8 k -n kube-system logs -l component=registry-agent --tail=100
```

The configuration the agent has, and whether it agrees with the cluster:

```bash
d8 k get registrynode <NODE> -o jsonpath='{.status}' | jq
```

The agent's metrics — its own view of the pulls passing through it. They are read directly
from the node rather than through Prometheus: the agent is a static pod precisely because it
has to work when the API server does not, and a kube-rbac-proxy beside it would authenticate
against that same API server:

```bash
ssh <NODE> 'curl -s http://127.0.0.1:4286/metrics | grep d8_registry_agent'
```

The configuration given to the container runtime. It is a single file, regardless of how many
registries are configured:

```bash
ssh <NODE> 'cat /etc/containerd/registry.d/_default/hosts.toml'
```

If this file is missing, the agent has not applied a configuration yet: nothing on the node
can pull, and the reason is in the agent's log. If the file is present and pulls still fail,
the failure is beyond the agent — the metrics above name the target that failed and the error.

## How do I check the state of the in-cluster cache?

The state of the cache is published in the status of the RegistryStorage resource:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq
```

Notes on the fields:

- `replicas` is the only place where completeness is reported, and each entry is the replica's
  own account of itself. A replica reporting `full: true` alongside an `error` is not
  complete: `full` says what it holds, and the error says whether its last pass finished.
- `leader` is the replica that fills from the upstream and serves as the replication source
  for the others. The election is deliberately not symmetric: only a replica holding the whole
  expected set stands for leadership, and a leader steps aside when another replica becomes
  complete. This is what keeps an air-gapped cluster from deadlocking with an empty leader and
  a full follower.

## The previous implementation

Everything below applies to a cluster still running the implementation configured through the
`deckhouse` ModuleConfig.

### How to Migrate to the registry module?

During the migration, Containerd v1 will switch to the new registry configuration format.
Containerd v2 uses the new format by default. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry).

#### For containerd v2

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the ModuleConfig `deckhouse`. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

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

1. Wait for the switch to complete. Example [status output](#how-to-check-the-registry-mode-switch-status):

   ```console
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

1. If configurations are present, you need to migrate to the new registry configuration format in containerd. To do this, add new configuration files to the `/etc/containerd/registry.d` directory. These configurations will take effect after switching to the `registry` module. To add configurations, prepare a NodeGroupConfiguration. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry). Example:

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

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the ModuleConfig `deckhouse`. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

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

1. After applying, wait for the following message to appear in the [switch status](#how-to-check-the-registry-mode-switch-status):

   Example output:

   ```console
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

1. After removing the old configurations, make sure that the switch has resumed. Example of the [switch status](#how-to-check-the-registry-mode-switch-status):

   ```console
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Wait for the switch to complete. Example of the [switch status](#how-to-check-the-registry-mode-switch-status):

   ```console
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

1. Check the switch status using the [instruction](#how-to-check-the-registry-mode-switch-status). Example output:

   ```console
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

1. Check the switch status using the [instruction](#how-to-check-the-registry-mode-switch-status). Example output:

   ```console
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

1. After deletion, wait for the switch to complete. Use the [instruction](#how-to-check-the-registry-mode-switch-status) to track the progress. Example output:

   ```console
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

### How to check the registry mode switch status?

The status of the registry mode switch can be retrieved using the following command:

<!-- TODO(nabokihms): replace with d8 subcommand when available -->
```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Example output:

```console
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
