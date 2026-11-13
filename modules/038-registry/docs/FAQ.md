---
title: "Registry module: FAQ"
description: "Frequently asked questions about the Deckhouse Platform registry module: migrating to the module, cache maintenance, and troubleshooting registry issues."
---

## How does the migration to the registry module work?

There are two ways a Deckhouse Platform (DP) cluster's image pull path can be managed:

- **the v1 implementation** (DP up to and including 1.77) — the `registry` section of the
  [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry), with the
  modes `Unmanaged`, `Direct`, `Proxy`, and `Local`;
- **the current implementation** — this module, configured through the `registry` ModuleConfig.

The migration is the handover of the pull path from the v1 implementation to the module.
It is not started by hand and has no command of its own: the module takes over automatically
after the cluster is upgraded to DP 1.78. The only condition is that by then
the v1 implementation must have released the pull path. Both implementations configure
the same thing on every node — which registry the container runtime pulls from and with which
credentials — so they never manage a cluster at the same time.

What has to be done for the handover, and when, depends on the mode the v1 implementation
is running in. The mode is set in the [`settings.registry.mode`](/modules/deckhouse/configuration.html#parameters-registry-mode) parameter of the `deckhouse`
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
Prepare the cluster **before** the upgrade: upgrading to DP 1.78 is blocked
while the cluster is running in `Proxy` or `Local` mode, or in `Direct` mode without a
configured `registry` ModuleConfig. DP 1.78 contains neither the components of the
v1 implementation nor the code that switches its modes, so all preparation happens on
DP 1.77.
{% endalert %}

`Direct` mode needs no mode switch. The module's settings are deliberately accepted already in DP
1.77: written before the upgrade, they are stored without effect and take effect on the
module's first reconciliation after it.

On nodes with containerd v1, a migration from or through `Unmanaged` restarts containerd. See
[Does the migration restart containerd?](#does-the-migration-restart-containerd).

The cluster records which implementation is actually running (see
[the next question](#which-implementation-is-my-cluster-running)). While the handover is
impossible, the upgrade to DP 1.78 is blocked, and the error message says
what to do.

## Which implementation is my cluster running?

When the module takes over the pull path, it records that in the `registry-v2-switch` secret.
Check whether the secret exists:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "current implementation" || echo "v1 implementation"
```

If the cluster is still on the v1 implementation, the module reports the reason on every
reconciliation and fires the
[`D8RegistryMigrationPending`](#what-do-the-registry-alerts-mean) alert. To see what it is
waiting for:

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

On DP 1.78 the update deletes the `registry-state` secret. The mode is then still shown by the
node configuration the v1 implementation left in the `registry-bashible-config` secret. This
command prints only the mode and the proxy endpoints, without credentials:

```bash
d8 k -n d8-system get secret registry-bashible-config -o jsonpath='{.data.config}' | base64 -d \
  | awk '/^(agent|mode):/ {print; next} /^proxyEndpoints:/ {p=1; print; next} p && /^ *- / {print; next} {p=0}'
```

An `agent:` line in the output, or the `registry.deckhouse.io/installer-config-sha256`
annotation on the secret, means the configuration was written by the current implementation
or by the installer of DP 1.78 or later, not by the v1 implementation. The annotation counts
only while the configuration is unchanged since the installer wrote it.

## How do I migrate from Unmanaged mode?

In `Unmanaged` mode the v1 implementation does not manage the pull path, so there is
nothing to prepare: the handover happens on its own.

1. If the cluster is switching to `Unmanaged` from another mode, wait for the transition to
   complete. The status must show `mode: Unmanaged` with no pending target mode:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to DP 1.78 (or, if it is already on it, just wait
   for the next reconciliation). The module takes over automatically, and the pull source does
   not change: the module's default mode is also `Unmanaged`, so the cluster keeps pulling from
   the same registry as before. On containerd v1 nodes, however, the takeover restarts containerd
   node by node if the v1 implementation left a node registry configuration, and enabling
   `mode: Managed` in the next step restarts it (a second time, if the takeover already did; see
   [Does the migration restart containerd?](#does-the-migration-restart-containerd)).

1. To have the module manage the pull path, set [`mode: Managed`](configuration.html#parameters-mode) in the `registry` ModuleConfig
   and specify the registry to pull from. A ready-made configuration for your cluster is
   published in the `registry-suggested-config` secret — see
   [enabling the module](examples.html#enabling-the-module).

## How do I migrate from Direct mode?

In `Direct` mode the nodes pull images through an in-cluster address served by the v1
implementation's proxy. The module serves the same address, so the migration is a direct
handover of that address: no switch through `Unmanaged` is needed, and no components restart.

1. Configure the module before the upgrade — without this configuration the upgrade is
   blocked. Take the values from the
   [`registry.direct`](/modules/deckhouse/configuration.html#parameters-registry-direct) section of the `deckhouse` ModuleConfig: `imagesRepo` splits into `host`
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

   DP 1.77 stores these settings without acting on them, so nothing changes in the
   cluster until the upgrade.

1. Upgrade the cluster. The handover happens on the module's next reconciliation, and pulls
   keep working throughout: the Service and the proxy of the v1 implementation keep
   serving the in-cluster address until the module's agent has taken it over on every node, and
   only then does the controller remove them.

1. Track the progress:

   ```bash
   d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 && echo "handed over"
   d8 k get registrynode -o custom-columns='NODE:.metadata.name,READY:.status.reconciled,SERVING:.status.proxyListening'
   ```

   For reference, the order and rough scale to expect: the handover is recorded a couple of
   minutes after the new version starts, the node agent appears on the nodes several minutes
   later, and the objects of the v1 implementation are removed shortly after that. Pulls
   keep working at every point in between.

## How do I migrate from Proxy mode?

`Proxy` mode keeps a proxy of its own, with its own certificates, on every node — state the
module cannot adopt. So the cluster must first be switched to `Unmanaged`, and this can only be
done before the upgrade.

1. In the `deckhouse` ModuleConfig, set [`registry.mode: Unmanaged`](/modules/deckhouse/configuration.html#parameters-registry-mode), keeping the same registry
   address and credentials — see
   [the mode switching examples](examples.html#examples-for-the-v1-implementation). All
   nodes are reconfigured to pull directly from the external registry, so the caching `Proxy`
   provided is lost until you bring it back in step 4.

1. Wait for the transition to complete — `mode: Unmanaged` with no pending target mode. A
   cluster caught mid-transition cannot be migrated, and the status shows which mode it is
   still heading to:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster. The handover happens on the module's next reconciliation, and the pull
   source does not change: in `Unmanaged` mode the module does not manage the pull path either.
   On containerd v1 nodes, however, the takeover restarts containerd node by node, and enabling
   `mode: Managed` in the next step restarts it again (see
   [Does the migration restart containerd?](#does-the-migration-restart-containerd)).

1. To get in-cluster caching back, set [`mode: Managed`](configuration.html#parameters-mode) with `storage.cache: true` and the same
   upstream registry. Note that the module's cache is built differently from `Proxy`: a single
   store with replicas on the master nodes instead of a proxy on every node. Before enabling it
   on a cluster with little free disk space on the control-plane nodes, read
   [how the cache is filled and cleaned up](#the-cache-keeps-growing-what-reclaims-it).

## How do I migrate an air-gapped cluster from Local mode?

In `Local` mode the cluster's registry lives inside the cluster itself, and there is no
external one. The standard procedure does not apply here: it goes through `Unmanaged`, where
every node pulls directly from an external registry — and this cluster has none.

The way through is to give the cluster an external registry temporarily: run it in the same
cluster, but outside the DP namespaces; switch the cluster to it; upgrade; let the
module's storage fill up; then remove it. The procedure has been verified end to end on a test
cluster.

### Disk requirements

Before starting, make sure the control-plane nodes have free space for **four image sets**.
Three copies exist at the peak — the `Local` store, the temporary registry, and the module's
storage being filled — and the fourth set is headroom the node must not run out of.

To get a sense of the scale, take an image set on the order of 13 GB (platform with modules):
it occupies about 21 GB in the temporary registry (two releases — the one the cluster runs and
the one it moves to) and grows the store from 13 to 21 GB, on top of the node's own image cache
and the system. A 100 GB control-plane node peaks at around 38 GB that way, while a 50 GB node
runs into pod eviction.

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

Steps 1–4 are performed before the upgrade, on DP 1.77, which still contains the v1
implementation.

1. Run a temporary OCI registry in a namespace of your own. Any registry implementation will
   do, but it must serve TLS with a certificate the cluster can verify. DP must not
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
   access to the DP registry, then `d8 mirror push` into the temporary one. This is the
   copy the disk budget above accounts for.

1. Point the v1 implementation at the temporary registry and switch it to `Unmanaged`:
   in the `deckhouse` ModuleConfig, set [`registry.mode: Unmanaged`](/modules/deckhouse/configuration.html#parameters-registry-mode) together with the address,
   the CA certificate, and the credentials of the temporary registry. The nodes start pulling
   from it. The `Local` store leaves the pull path, but its data stays where it is — under
   `/opt/deckhouse/registry` on the control-plane nodes.

1. Make sure the transition is complete and images really are pulled from the temporary
   registry — the rest of the migration rests on this step. The checks are the same as for any
   `Unmanaged` cluster:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Upgrade the cluster to DP 1.78. The handover happens on the module's
   next reconciliation; the cluster keeps pulling from the temporary registry throughout. On
   containerd v1 nodes the takeover restarts containerd node by node, and enabling
   `mode: Managed` in the next step restarts it again (see
   [Does the migration restart containerd?](#does-the-migration-restart-containerd)).

1. Enable the module: [`mode: Managed`](configuration.html#parameters-mode) with `storage.cache: true`, the temporary registry as
   `primary.upstream` — **and `storage.source` in the same edit**. The module's storage starts
   on the same host path the `Local` store used, so the images already on the disks are
   adopted: the fill verifies them and downloads only what is missing.

   Do not postpone specifying [`storage.source`](configuration.html#parameters-storage-source): without it, the upstream cannot be removed in the next
   step. A `Managed` configuration without `primary.upstream` is only accepted when
   `storage.source` is set, so the next step would be rejected with
   `'storage.source' is required when 'primary.upstream' is not set`.

   In `storage.source`, `bundleRef` is a name for the image set, and `expectedDigests` is the
   number of distinct digests in it. Count them in the bundle you pushed:

   ```bash
   for tar in <BUNDLE_DIR>/*.tar; do tar -xOf "$tar" --wildcards '*index.json'; done |
     jq -r '.manifests[]?.digest' | sort -u | wc -l
   ```

   Setting `storage.source` does not restart the storage: it is not part of the registry
   process's configuration. The registry restarts in the next step, when the upstream is removed.

1. Wait until the storage reports that it holds the whole set — `phase: Ready` with
   `safeToDropUpstream: true` — and remove `primary.upstream` from the `registry` ModuleConfig.
   The cluster is air-gapped again, now on the module. After a storage replica restarts,
   `safeToDropUpstream` can stay `false` until the leader's syncer has read its store, normally
   within two minutes, or about ten minutes if that first reading fails:

   ```bash
   d8 k get registrystorage registry -o jsonpath='{.status.phase} {.status.safeToDropUpstream}{"\n"}'
   ```

   Removing the upstream restarts the registry process. If the restarted registry spends more
   than a few tens of seconds reading its store, the RegistryStorage resource reports `Failed`
   with an error about reading the image set. This is expected; wait for it to return to `Ready`.

1. Delete the temporary registry and reclaim its disk space.

## Does the migration restart containerd?

On nodes with containerd v1 ([`cri.type: Containerd`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type)),
it does. Nodes with containerd v2 always take their registry settings from the
`/etc/containerd/registry.d` directory, and containerd is not restarted on them.

On a containerd v1 node, `/etc/containerd/config.toml` points containerd at
`/etc/containerd/registry.d` only while the cluster has a node registry configuration, the
`registry-bashible-config` secret. Without that secret, the file describes the registry with its
own `mirrors` and `configs` blocks. In `Unmanaged` mode the module removes the secret, including
the one the v1 implementation left behind. A change of `config.toml` restarts containerd,
and kubelet after it. Therefore, on containerd v1:

- every switch between `Unmanaged` and `Managed`, in either direction, restarts containerd once.
  From `Managed` to `Unmanaged`, the restart comes when the module stops serving, that is once no
  workload names the in-cluster address `registry.d8-system.svc:5001` any more, not when the
  ModuleConfig is changed;
- a cluster where the v1 implementation left the secret restarts it twice on the way
  through `Unmanaged`: when the module takes over after the upgrade, and again when
  `mode: Managed` is enabled. That is a cluster switched from `Proxy` or `Local` to `Unmanaged`
  with a registry address before the upgrade, and any other `Unmanaged` cluster with a registry
  address;
- a migration from `Direct` does not restart it: the node configuration is kept throughout.

To check whether the secret is there, run
`d8 k -n d8-system get secret registry-bashible-config`.

containerd is not restarted on all nodes at once. The change reaches them as a regular node update, which
waits for approval, and approval is granted one node at a time in each NodeGroup by default (see
[`update.maxConcurrent`](/modules/node-manager/cr.html#nodegroup-v1-spec-update-maxconcurrent)).
Running containers survive the restart. On the node being updated, image pulls in progress fail
and kubelet retries them, and open `exec`, `attach`, `port-forward` and `logs -f` sessions are
cut.

## What do the registry alerts mean?

Apart from the two about the node agent, `D8RegistryStorageLeaderAbsent` in an air-gapped cluster
and `D8RegistryMigrationPending`, none of these alerts mean that the cluster has stopped pulling
images. Most of them report a state where everything works, but not the way the configuration asks —
a state that is easy to miss otherwise.

`D8RegistryMigrationPending`
: The module has not taken over from the v1 implementation, used up to and including DP 1.77,
  and the alert's reason says why. If the reason names the `Direct` mode, configure the module as
  described in [migrating from Direct mode](#how-do-i-migrate-from-direct-mode): the module then
  takes over on its own. Any other reason (the `Proxy` or `Local` mode, a transition, no recorded
  mode, or a state that cannot be read) means a state that is not supported. A cluster gets there
  only through an update that bypassed the `registryImplementation` release requirement: a forced
  release, an edited image, or a jump from a release older than DP 1.77.1. DP 1.78 does not
  deploy the v1 implementation, and the update removed the components that managed its node
  services on the masters, so those services were stopped or keep running with nothing to manage
  them. Images already on a node keep working; pulling any other image may fail. Do not change
  the `registry` or `deckhouse` ModuleConfigs and do not delete the `registry-*` secrets in
  `d8-system`: those secrets keep the current implementation from being deployed next to what
  the v1 implementation left. Contact Deckhouse Platform technical support. See also
  [which implementation the cluster is running](#which-implementation-is-my-cluster-running).

`D8RegistryConfigInvalid`
: The configuration was rejected; the cluster keeps working with the previous one. The reason
  is in `.status.conditions` of the `registryconfig/registry` resource.

`D8RegistryNodeNotConverged`
: The agent on some nodes has not applied the configuration it was given. Those nodes keep
  pulling with their old configuration, and further changes will not reach them either.

`D8RegistryNodeRunningFromDisk`
: The agent on some nodes could not get a usable configuration from the API server and routes
  pulls using the copy stored on disk. Most often the credentials the configuration references
  cannot be read; a read of the configuration that failed while the status write after it went
  through, as on a short API server outage, looks the same. This fallback works as designed: pulls on those nodes succeed, so
  nothing else would report the problem — meanwhile their configuration can lag arbitrarily far
  behind the cluster's. The agent reports the fallback through the API server, so the alert does
  not cover an agent that cannot write there: a node cut off from the API server shows as
  `NotReady` instead, and an agent refused access on a `Ready` node is not reported at all.

`D8RegistryAgentNotReady`
: The agent on a node has not been ready for 15 minutes: it fails on start, does not answer, or
  is not running on the node at all. Until it is back, containerd on that node pulls without it:
  platform images and images from registries that need the cluster's credentials or CA fail,
  other images bypass the cache. See [where to look](#a-pull-is-failing-on-a-node-where-do-i-look).

`D8RegistryAgentRestartingTooOften`
: The agent on a node has restarted five times or more in an hour. Each restart drops the
  pulls in progress on the node. The container's last state and the log of the previous run say
  whether it exited on its own or was restarted for not answering.

`D8RegistryStorageIncomplete`
: Some cache replicas do not hold the whole expected image set. While an upstream is
  configured this does not affect pulls, but the cluster would not survive losing the
  upstream — and this is what blocks a transition to air-gap. A replica whose store the
  registry cannot write to (`NotWritable`) is reported by `D8RegistryStoreWritesRefused`
  instead.

`D8RegistryAirGapTransitionHeld`
: The upstream was removed from the configuration, but the module keeps using it: either the
  cache cannot serve the cluster alone yet, or the cache leader's report was not written by the
  `syncer` running on its node now (`syncerStartedAt` in the RegistryStorage status), or workloads
  in the `kube-system` and `d8-*` namespaces still reference the upstream (`upstreamReferences`).
  This is the safe outcome: dropping the upstream at that point would leave the nodes with
  nowhere to pull from. The alert does not resolve on its own if the cache has stopped filling,
  a rollout onto the in-cluster address is stuck, the leader's storage pod is not Ready, or that
  pod runs a `syncer` from before `syncerStartedAt` existed (during a module update).

`D8RegistryUpstreamProbeFailing`
: A change to the primary upstream was rejected, and the cluster keeps using the last working
  one. The `outcome` label tells the problems apart: `unreachable` — network or the registry
  itself; `auth` — usually an expired license key; `sentinel` — the registry responded and
  accepted the credentials, but does not contain the DP images (usually a wrong
  repository path).

`D8RegistryUpstreamRejected`
: A RegistryUpstream resource was not accepted, so pulls for the registry it names are not
  intercepted. The `reason` label says whether it conflicts with the primary registry or with
  another resource claiming the same name.

`D8RegistryStorageNotReclaimed`
: No replica has run garbage collection for a week. Collection is the only thing that removes
  data from the store, so if it has stopped, the disk will eventually fill up. See
  [what cleans the cache up](#the-cache-keeps-growing-what-reclaims-it).

`D8RegistryStoreCorrupted`
: A replica holds blobs whose content does not match their digest, and the repair the module
  started on its own has not fixed them for two hours. Pulls of the affected images from that
  replica fail. See [how to check and repair the images](#how-do-i-check-and-repair-images-in-the-cache).

`D8RegistryStaleCacheData`
: A node holds cache data that nothing uses. See
  [how to remove it](#how-do-i-remove-leftover-cache-data-from-a-node).

`D8RegistryStoreWritesRefused`
: The cache on a master has stopped accepting writes: its store reached `storage.size`
  (`BudgetExhausted`) or the space kept free on its filesystem (`ReserveExhausted`). The node is
  safe and the cache keeps serving what it holds; what stops is filling it. See
  [what happens when the store is full](#what-happens-when-the-caches-store-is-full).
  `NotWritable` means something else: the registry cannot create files in its tree in the
  store (`docker/registry/v2` and below, or the store's directory while that tree does not
  exist yet), because of their owner or mode, an attribute, a read-only mount or a security
  module. Neither garbage collection nor a larger `storage.size` helps. The replica's `error`
  in the RegistryStorage status names the directory and the command that fixes it.

`D8RegistryStoreNearlyFull`
: The store on a master holds over 85% of `storage.size`. The next release may not fit.

`D8RegistryStoreUnbounded`
: The cache is on and `storage.size` is not set, so the only limit is the space kept free on
  the store's filesystem.

`D8RegistryControllerDown`
: No replica of `registry-controller` is ready and answering Prometheus. Pulls are not
  affected, but no configuration change reaches the nodes, and most other alerts of the module
  are silent while this one fires: they read what the controller exports.
  `D8RegistryStorageLeaderAbsent` and the migration and drain alerts do not depend on the
  controller and keep working.

`D8RegistryStorageLeaderAbsent`
: No ready cache replica reports holding the leader lease. Nothing fills the cache, the other
  replicas have nobody to replicate from, and `d8 mirror push` has nowhere to go. The nodes
  still route pulls to the replica named as leader: with an upstream configured pulls are
  slower, in an air-gapped cluster images the nodes do not already hold cannot be pulled. If
  the Lease `registry-storage` in `d8-system` keeps being renewed, the leader is alive but its
  report is not scraped: either its pod is not ready, then find out which of its containers is
  not ready, or the scrape of that pod fails, then look at the log of its `kube-rbac-proxy`
  container. If the master that held the lease is lost and is not coming back, delete its Node
  object; otherwise look at the `error` the replicas report in the RegistryStorage status and
  at their `syncer` logs. A cache replica that is merely not ready is reported by
  `KubernetesStatefulSetReplicasUnavailable` of the `extended-monitoring` module, if it is
  enabled.

`D8RegistryStorageUpdateBlocked`
: The cache has a new revision, but for an hour the controller has been unable to replace the next
  replica. Existing replicas remain running, but the cache update cannot proceed. The `reason` label
  says what the controller is waiting for and whether anything needs doing. See [how the cache is updated](#how-is-the-cache-updated).

## How do I remove leftover cache data from a node?

When the cache is turned off, the data in the cache directory is intentionally kept:
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

If you are not going to turn the cache back on, remove the contents of the cache directory on the
node: `/var/lib/deckhouse/registry` on a node of a NodeGroup with `systemType: DeckhouseEngine`,
`/opt/deckhouse/registry` on other nodes. Keep the directory itself: the agent has it mounted, and
once it is removed the agent no longer sees the node's directory, so data a cache leaves there
later goes unreported until the agent restarts. To do this, follow these steps:

1. Open a root shell on the node over SSH. A node of a NodeGroup with `systemType: DeckhouseEngine` has
   no SSH: start a debug pod on it and switch to the node's root filesystem:

   ```bash
   d8 k debug node/<NODE> -it --image=<IMAGE>
   chroot /host
   ```

1. Remove the data. The command checks both directories and keeps the directories themselves:

   ```bash
   for dir in /opt/deckhouse/registry /var/lib/deckhouse/registry; do
     [ -d "$dir" ] && du -sh "$dir" && find "$dir" -mindepth 1 -delete
   done
   ```

## The cache keeps growing. What reclaims it?

Garbage collection, which every storage replica runs on its own store. The module starts it on a
schedule, once the cache holds a new image set, and when the store refuses writes; it can also be
started by hand.

It is the only mechanism that removes anything from the store. Every DP release adds new
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

It also removes blobs that no image references, such as those left by an interrupted fill or by a
collection whose cleanup failed. A collection removes them even when there are no images to delete.

The collector is deliberately cautious. In an air-gapped cluster, deleting a blob that is
still needed is unrecoverable without another `d8 mirror push`, while keeping an unneeded one
merely costs disk space. So a run that cannot determine what to keep — for example, when no
deployed release is found — does nothing at all.

To check the collection status and schedule, use these commands:

List the recent collections — what started them, how they went, and how much they freed. A finished
collection is kept for a day:

```bash
d8 k get registryoperation -o wide
```

Check the last collection time on each replica, and any errors:

```bash
d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
  'map({node, collectedAt, collectionError})'
```

Check the current collection schedule:

```bash
d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
```

### When it runs, and what happens during a run?

The registry's collector first computes the set of reachable blobs and then deletes the rest,
so a blob written between those two steps and not yet referenced by a manifest would be
deleted. A replica therefore collects only when its store is not being written to:

- a collection does not start while the cache is being filled or replicated, or while a
  `d8 mirror push` is in progress — it waits until the writing is over;
- the replica looks at its store again when its turn comes: if a fill or a push started while it
  waited, it gives the turn back and waits again;
- it looks once more right before removing blobs: if a push has started by then, the blobs are
  left for the next collection;
- while it collects, the replica stops filling and replicating its own store.

The replica keeps serving all the images it holds and fetching cache misses from the upstream.
Do not run `d8 mirror push` while a collection is running: check `d8 k get registryoperation`
first. A push that starts during the removal of blobs can fail with a visible error, and it can
also report success and leave an image incomplete: a layer the store already held is not uploaded
again, and the collection can remove it. Such an image fails to pull later with a missing blob.
If a push overlapped a collection, run it again after the collection finishes.

Only one replica collects at a time; the others work normally.

The schedule is in UTC. By default, collection is scheduled at a night hour — or, if the `master` node group has a
maintenance window, at its start, since that hour is already declared safe for disruption. To
set your own schedule, use the [`storage.garbageCollection`](configuration.html#parameters-storage-garbagecollection) parameter:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        schedule: "0 2 * * Sun"
```

A cron expression that cannot be parsed is rejected rather than guessed at: collecting at an
unexpected hour is worse than not collecting. For the same reason a changed schedule counts from
the moment it changed: a firing of the new schedule that has already passed is not made up for. The
same applies after collection is turned back on.

Collection also starts once every replica holds a new image set: after a Deckhouse Platform update, or after
a module is enabled or updated. That is the moment the images only the previous set needed become
garbage. Disabling a module does not change the set: its images are kept for when it is enabled again.

A store that refuses writes — over its budget, or down to the space kept free on its filesystem — can
never take in a new set, so it is collected without waiting for one: once per deployed Deckhouse Platform
release, and again after an update or after the store has accepted writes in between. If that
collection fails, it is tried again four hours after it ended.

Every collection is a [RegistryOperation](cr.html#registryoperation) object of type
`GarbageCollection`, whoever started it. Only one runs at a time: a firing of the schedule that comes
while another collection is running is skipped, and a collection for a new image set waits for the
running one to finish.

### Is a collection worth running now?

Each storage replica estimates how much a collection of its store would free and exports the estimate to Prometheus metrics:

- `d8_registry_storage_reclaimable_bytes`: how many bytes a collection would free now. Zero means a collection would free nothing.
- `d8_registry_storage_reclaimable_estimated_timestamp_seconds`: when the estimate was made.

Both carry the `node` label of the replica. Neither is exported before the replica's first estimate.

A replica makes the estimate:

- when the image set the cluster needs changes;
- after a collection on that replica;
- every 30 minutes otherwise, and not more often than once in 5 minutes.

Estimate is not performed:

- during the filling of the cache;
- while uploading images to the storage ('d8 mirror push');
- during garbage collection.

Therefore, when viewing an estimate, pay attention to its time. Storage that refuses to write is evaluated as usual: this is when garbage collection is most needed.

To see the estimate for every replica, run the following query in Prometheus:

```text
d8_registry_storage_reclaimable_bytes
```

### How to run garbage collection manually?

To run garbage collection manually, create a [RegistryOperation](cr.html#registryoperation) object of type `GarbageCollection`:

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

It runs even when automatic collection is turned off. Replicas collect one at a time, exactly as
on schedule. A replica starts only when its store is not being written to: while the cache is being
filled or a `d8 mirror push` is in progress, the replica waits, and the reason is shown in
`status.replicas[].reason`. A store that refuses writes is not waited for: a fill cannot go on until
the collection frees room. A replica still held back an hour after the operation was accepted fails
with the reason `DeadlineExceeded`. An operation created while another one is running fails with the
reason `AnotherOperationActive`.

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

If a replica's `syncer` container stops answering while the replica collects or waits for its turn,
for example because its master went `NotReady`, the replica fails with the reason `ReplicaLost`, so
that the operation does not hold every collection after it for a day:

- a replica that was collecting, once it has not held the Lease `registry-storage-gc` in the
  `d8-system` namespace for five minutes;
- a replica that was waiting, once its node has not been `Ready` for ten minutes. A master that is
  back sooner keeps its replica's turn.

A replica that waits on a node that stays `Ready` while its `syncer` does not answer is released by
the cancel label, by the 24-hour timeout (the reason `TimedOut`), or by deleting the operation:

```bash
d8 k delete registryoperation <NAME>
```

### Turning it off

To disable automatic garbage collection, set the value to `false` for the [`storage.garbageCollection`](configuration.html#parameters-storage-garbagecollection) parameter:

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

Garbage collection created [manually] (#how-to-run-garbage-collection-manually) is performed even if automatic is disabled.

## What happens when the cache's store is full?

Each master's store has two limits, and the registry checks both on every write:

- the budget, `storage.size` in the `registry` ModuleConfig — how much the store may hold;
- the reserve — how much of the filesystem under the store must stay free. On a filesystem
  shared with the node it is the kubelet's soft eviction threshold (10% of the filesystem, at
  most 40 GiB) plus a margin (10%, at most 20 GiB). On a disk of the store's own
  (`dedicatedFilesystem: true` in the replica's `store`) it is the margin alone. Both are taken
  of the filesystem's current size, so a disk grown under a running store moves the reserve
  without a restart. The reserve applies whatever `storage.size` says, including when it is not
  set.

The reserve is taken from the full size of the filesystem, while free space is counted the way
the kubelet counts it: without the blocks the filesystem keeps for root. On ext4 these are 5% by
default, so on a filesystem under 200 GiB the store and everything else on it get about 75% of
the disk rather than 80%. On a filesystem dedicated to the store, the root reservation can be
lowered with `tune2fs -m`.

When either limit is reached, the store refuses new writes and keeps serving everything it holds:

- a fill or a replication stops. Why the store refuses is in the replica's `store` field
  (`writable: false` and `reason`, see below), and `.status.replicas[].error` holds the error of
  the fill or replication that stopped;
- `d8 mirror push` gets `507 Insufficient Storage` with the reason and the way out, before the
  upload's data is sent. A blob larger than the space left can still be refused in the middle of
  its upload, with `500`; push again once there is room;
- a cache miss is served from the upstream while there is one, so the nodes do not notice: the
  layers the store cannot keep are streamed through without being stored, and a manifest the
  store cannot write fails at the store, so the agent takes the image from the upstream. In an
  air-gapped cluster a release that is not in the store cannot be installed.

The store keeps the deployed release and the previous one, for a rollback, together with the
modules the cluster runs and the catalog of modules the module sources offer. It also needs room
for the next release:

- a replica copies the next release ahead of the update only while the room it has left in the
  budget, and on the filesystem above the reserve, is at least a fifth of the budget (or of the
  filesystem, when `storage.size` is not set); otherwise the copy ahead is skipped, and the
  release is filled once it is deployed. With an upstream, this applies to every replica;
- in an air-gapped cluster, the next release arrives with `d8 mirror push` on the leader, and the
  garbage collection does not delete the deployed or the previous release to make room for it. The
  followers copy it from the leader under the same rule, so a follower short of room gets the
  release only once it is deployed.

So size `storage.size` for three releases together with the modules and the catalog. The store
holds the union of their images: a layer the releases share is stored once.

Nothing is deleted to make room. Space is freed by the garbage collection, or made by raising
`storage.size` or giving the store more disk.

To see where each replica stands:

```shell
d8 k get registrystorage registry -o json | jq '.status.replicas[] | {node, store}'
```

`writable: false` with `reason: BudgetExhausted` means `storage.size` is reached: raise it if the
disk has room, or let the collection reclaim what no deployed release needs.
`reason: ReserveExhausted` means the disk is. On a filesystem shared with the node, free space on
the node or move the store to a disk of its own. On a disk of its own, grow the disk or let the
collection reclaim space. `reason: NotWritable` is not a limit: the registry cannot create files
in the store, and `.status.replicas[].error` names the directory and the command that fixes it.

## The store's disk did not mount. What happens?

If the store was on a disk of its own (`dedicatedFilesystem: true`) last time and that disk is
not mounted now, for example after a reboot, the store starts on the node's own filesystem and
fills there. The replica reports this in `.status.replicas[].error` for as long as the disk is
missing, and keeps reporting it after the store has filled. A replica with an error does not
count as full:

- it does not stay the leader while another replica is full: that replica takes over;
- if it is the leader and no other replica is full, for example on a single master, the
  RegistryStorage phase is `Failed` and a transition to air-gap waits;
- after two hours `D8RegistryStorageIncomplete` fires.

To fix it, mount the disk before the kubelet starts (in `/etc/fstab` without `nofail`) and delete
the replica's pod on that node. To keep the store on the node's filesystem on purpose, delete the
`.filesystem-before` file in the store's directory on the node, then the replica's pod:

```shell
d8 k -n d8-system delete pod -l app=registry-storage --field-selector spec.nodeName=<NODE>
```

## A pull is failing on a node. Where do I look?

Start with the agent: it is on the path of every pull on the node. It is deployed as a static
pod, so it is available even when the cluster is not.

The kubelet probes the agent on the address the container runtime uses, `127.0.0.1:5001`, and
restarts it after a minute without an answer. An agent that stays not ready raises
[`D8RegistryAgentNotReady`](#what-do-the-registry-alerts-mean), and one that keeps restarting raises
[`D8RegistryAgentRestartingTooOften`](#what-do-the-registry-alerts-mean). Neither alert covers an
agent that answers but cannot reach the API server: that one is ready.

To check the agent's logs, use the command:

```bash
d8 k -n kube-system logs -l component=registry-agent --tail=100
```

To check the configuration the agent has, and whether it agrees with the cluster, use the command:

```bash
d8 k get registrynode <NODE> -o jsonpath='{.status}' | jq
```

The agent's metrics — its own view of the pulls passing through it. Prometheus collects them
through the node exporter of the `monitoring-kubernetes` module: every 30 seconds the agent writes
them to the node exporter's textfile directory, `/var/run/node-exporter-textfile/registry-agent.prom`.
The `d8_registry_agent_*` series therefore come with `job="node-exporter"` and the node's name in
the `node` label, for example:

```text
sum by (node, kind, result) (rate(d8_registry_agent_requests_total[5m]))
```

Nothing scrapes the agent directly: it is a static pod precisely because it has to work when the
API server does not, and a `kube-rbac-proxy` beside it would authenticate against that same API
server. Without `monitoring-kubernetes` the series are not collected. On the node itself the same
metrics, together with the agent's Go runtime metrics, are served on the loopback address:

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

containerd can also pull past the agent, and this is by design. `_default/hosts.toml` sets no
`server`, so containerd keeps the original registry as the last host and goes there directly
when the agent does not answer, or answers `404` or another error status. This way images from
public registries can still be pulled while the agent is down, but the direct attempt gets nothing
the agent adds:

- no cluster credentials, only the pod's own pull secret, if it has one;
- no RegistryUpstream routes or mirrors, and no cache;
- no certificate authority of a private registry, so such a registry usually fails with `x509`;
- no in-cluster address: `registry.d8-system.svc:5001` does not resolve on the node, so platform
  images cannot be pulled this way at all.

Which error containerd reports when a pull fails depends on the step that failed and on the
containerd version. While resolving the image reference, a tag or a digest:

- containerd 1.7 (`Containerd`) reports the first error it got other than `404`. If the agent
  did not answer or answered with an error status, that is the agent's error, even though the
  direct attempt was made after it: a refused connection or a TLS handshake timeout to the agent's
  address, or an error status such as `502 Bad Gateway`. The error of the direct attempt
  (`no such host`, `401`, `x509`) is reported only when the agent answered `404`, and then it
  hides that the agent did not find the image.
- containerd 2.2 (`ContainerdV2`) reports an error status other than `404` ahead of a `404`, and
  a `404` ahead of a failed connection; of two errors of the same kind, it reports the agent's. So
  an error status from the agent, such as `502 Bad Gateway`, is always what shows. A failed
  connection to the agent shows only when the direct attempt failed to connect too, and a failed
  connection of the direct attempt (`no such host`, `x509`) never shows. Otherwise the answer to
  the direct attempt shows: its error status, such as `403`, or `not found` for its `404`. An
  agent's `404` is reported as `not found` unless the registry answered the direct attempt with
  an error status.

Either way, check the agent's log and metrics first. When the direct attempt answers `404` too,
as for a tag missing from a public registry, the pull is reported as `not found`. While fetching
the manifest by digest, the image config and the layers, both versions report the agent's error
whatever it was, `404` included: a layer the agent does not have is reported as `not found` even
when the direct attempt then failed with another error.

While the agent does not answer, pulls are also slow to fail or succeed: on a test cluster with
the agent frozen, pulls of platform images failed after about 20 seconds (a TLS handshake timeout
to the agent), and images from public registries arrived over the direct path after about 50
seconds.

## Modules from other ModuleSources: what the cache holds and what to do before air-gap

The cache holds every module the cluster runs, whichever ModuleSource it was installed from, and
counts it towards completeness: a cluster does not go air-gap while a module it runs is missing
from the store.

- A module whose ModuleSource is on the upstream's registry is filled from the upstream, like the
  platform itself.
- A module whose ModuleSource is on a registry of its own is filled from that registry, with the
  credentials and certificate authority the ModuleSource carries.

The platform's own modules — ModuleSource `deckhouse` — are stored under `system/deckhouse/modules`.
Every other ModuleSource has a path of its own in the in-cluster registry,
`system/deckhouse/module-sources/<ModuleSource>`: its modules under `<path>/<module>`, and its list of
modules as the tags of `<path>`. A path apart, because the platform's is the upstream's as long as an
upstream is configured, and a module another source ships under a name the platform also has would
be replaced there by the upstream's build.

That path is the only place a node can pull the source's modules from once the upstream is gone, but
the module's pods keep referring to their ModuleSource's registry until the source is changed. So
before removing the upstream, point every such ModuleSource at its path, with the registry settings
the platform's own source uses:

```shell
NAME=<ModuleSource>
REGISTRY=$(d8 k get modulesource deckhouse -o json | jq -c --arg name "$NAME" \
  '.spec.registry | .repo = "registry.d8-system.svc:5001/system/deckhouse/module-sources/" + $name')
d8 k patch modulesource "$NAME" --type merge -p "{\"spec\":{\"registry\":$REGISTRY}}"
```

After that the module's images are rendered with the in-cluster address and are served from the
cache.

## What does destroying an air-gapped cloud cluster need from the store?

`dhctl destroy` of a cloud cluster whose provider is not built into the installer, such as
Deckhouse Virtualization Platform, needs that provider's bundle: its schemas, OpenTofu modules
and plugins. dhctl takes the bundle from its download directory (`--download-dir`, `/tmp/dhctl`
by default) when it is there, and pulls it from the registry the cluster names otherwise. An
air-gapped cluster names no upstream, so the bundle is pulled from the cluster's own store,
through dhctl's SSH connection to a master. With the store empty or missing that image, destroy
stops at `Get cluster configuration` with `MANIFEST_UNKNOWN`. The simplest way out is to run
dhctl in a container that already has this bundle downloaded, such as the one a previous operation
on this cluster ran in. Otherwise keep the store until the cluster is destroyed, or bring the bundle
yourself: on a machine with access to the public registry, pull the digest the error names from the
platform's image repository there, the one the cluster was installed from (for example,
`registry.deckhouse.io/deckhouse/<edition>`; `system/deckhouse` in the error's address is only the
store's name for that repository). A `d8 mirror` bundle of the release holds the image too. Unpack
the image into the directory the error names.

## How do I push a new Deckhouse Platform release or a new module version into an air-gapped cluster?

Once the upstream is removed from the configuration and the cluster has no internet access,
`d8 mirror push` is the only way new images get into the in-cluster cache. The cache leader accepts
the push, and the other replicas copy the images from it. As soon as a release is in the cache, the
cluster sees it the same way it would see it in the upstream and updates by the usual rules: the
release channel, the update windows, and manual approval if it is enabled.

You don't need to bring the upstream back for this, and you don't need to change
[`storage.source`](configuration.html#parameters-storage-source) either. The cache's completeness is
checked against the image set the cluster needs, not against `expectedDigests`. Garbage collection
keeps releases newer than the deployed one, so a pushed release stays in the store until it is
deployed.

### Step 1. Pull the images on a machine with internet access

Get the Deckhouse Platform edition and version running in the cluster:

```bash
d8 k -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {"core.deckhouse.io/edition","core.deckhouse.io/version"}'
```

Pull the Deckhouse Platform releases starting from the cluster's minor version. Minor versions are installed one
after another, so the bundle has to contain every one in between, and `--since-version` pulls them:

```bash
d8 mirror pull --source='registry.deckhouse.io/deckhouse/<EDITION>' --license='<LICENSE_KEY>' \
  --since-version='<X.Y>' ./d8-bundle
```

To update only a module without pulling the platform, give its current version. The command pulls
that version, the newer ones, and the versions on the release channels:

```bash
d8 mirror pull --source='registry.deckhouse.io/deckhouse/<EDITION>' --license='<LICENSE_KEY>' \
  --no-platform --include-module='<MODULE>@<VERSION>' ./d8-bundle
```

The [Updating DP in air-gapped environment](/products/kubernetes-platform/documentation/v1/guides/airgapped-update.html)
guide explains how to make the bundle smaller and how to build the list of modules.

Move the bundle to the machine you will run `d8 mirror push` from. Make sure the cache's store has
room for it: a push into a full store gets `507 Insufficient Storage` (see
[What happens when the cache's store is full?](#what-happens-when-the-caches-store-is-full)).

### Step 2. Check the publication endpoint

Images get into the cache only through the publication endpoint: the `registry-push` Ingress in the
`d8-system` namespace, which passes the push on to the cache leader. A push sent straight to a
master node, around the Ingress, is refused: the port that accepts writes there takes them only from
the ingress controller and from the cache itself (see
[What protects the ports on control-plane nodes](/modules/registry/#what-protects-the-ports-on-control-plane-nodes)).

Check whether the cluster has the `registry-push` Ingress and whether an ingress controller serves it:

```bash
d8 k -n d8-system get ingress registry-push -o jsonpath='{.spec.ingressClassName}{"\t"}{.spec.rules[0].host}{"\n"}'
d8 k get ingressclass
```

- The Ingress exists and its class is among the IngressClasses: go on to step 3.
- The Ingress exists, but its class is not among the IngressClasses: set the
  [`ingressClass`](configuration.html#parameters-ingressclass) parameter of the `registry` ModuleConfig
  to the class of an existing controller, then go on to step 3.
- There is no Ingress: it is created only when
  [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate)
  is set and the `cert-manager` module is enabled. Set the parameter and enable the module: the
  Ingress appears on the next run of the `registry` module. If the cluster cannot have the Ingress,
  set [`primary.upstream`](configuration.html#parameters-primary-upstream) to a registry the cluster
  can reach that holds the images, let the cache fill from it, and remove the upstream again.

Run the `d8 k` commands in the next steps where you have access to the cluster's API. If the bundle
is on another machine, carry the values over to it.

### Step 3. Push the images

```bash
HOST=$(d8 k -n d8-system get ingress registry-push -o jsonpath='{.spec.rules[0].host}')
PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
d8 mirror push ./d8-bundle "$HOST/system/deckhouse" \
  --registry-login="$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
  --registry-password="$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
```

What the push needs:

- The name in `$HOST` resolves to the ingress controller's address on the machine you push from.
- If [`whitelistSourceRanges`](configuration.html#parameters-whitelistsourceranges) is set, that
  machine's address is in the list.
- The machine trusts the Ingress certificate. Let's Encrypt can't issue one for an air-gapped cluster,
  so such clusters usually use their own ClusterIssuer or the `CustomCertificate` mode (the
  [`https`](configuration.html#parameters-https) parameter). If the machine doesn't trust the
  certificate authority, pass its certificate in the `SSL_CERT_FILE` environment variable.
- The cache has a leader. While there is none, the push is refused; look at the replicas in the
  RegistryStorage status (see
  [How do I check the state of the in-cluster cache?](#how-do-i-check-the-state-of-the-in-cluster-cache)).
  If the leader changed during the push, for example because a master node rebooted, run the
  command again.

Do not push while a garbage collection is running: check `d8 k get registryoperation` first. A push
that overlaps a collection can fail, and it can also report success and leave an image incomplete.
If a push overlapped a collection, run it again after the collection finishes (see
[When it runs, and what happens during a run?](#when-it-runs-and-what-happens-during-a-run)).

### Step 4. Check that the release has reached the cluster

Check the state of the cache. The other replicas copy the new release from the leader before it is
deployed:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Check that the cluster sees the new Deckhouse Platform release or module release:

```bash
d8 k get deckhousereleases
d8 k get modulereleases
```

From here on the release is deployed according to the [Deckhouse Platform update settings](/modules/deckhouse/configuration.html#parameters-update)
and the module update settings.

## Image availability alerts from `extended-monitoring` for the in-cluster registry

With the module in charge, every image reference in the cluster names `registry.d8-system.svc:5001`.
Nodes pull by that address through the node agent on the loopback, not through cluster DNS. The
image availability exporter of the `extended-monitoring` module checks images from a pod, where
the name is looked up in cluster DNS instead, so it reports every such image as
`…ImageAvailabilityUnknownError`:

- without the in-cluster cache, or while no storage replica is ready, the name does not resolve
  from a pod: `no such host`;
- with the cache, the name is a headless Service that resolves to the master nodes running ready
  storage replicas. The storage answers there with a certificate issued by the module's own
  certificate authority, which the exporter does not trust: `x509: certificate signed by unknown
  authority`.

Nothing is wrong with the images: what the exporter would detect is covered by this module's own
alerts — `D8RegistryStorageIncomplete`, `D8RegistryUpstreamProbeFailing`,
`D8RegistryNodeNotConverged`. Exclude the in-cluster address from the exporter's checks:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: extended-monitoring
spec:
  version: 2
  settings:
    imageAvailability:
      ignoredImages:
      - 'registry\.d8-system\.svc:5001/.*'
```

Each entry is a regular expression. Images from other registries are still checked.

## How do I check the state of the in-cluster cache?

The state of the cache is published in the status of the RegistryStorage resource. To view it, use the command:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq
```

Description of the response fields:

- `replicas` is the only place where completeness information is reported, and each entry is the replica's
  own account of itself. A replica reporting `full: true` alongside an `error` is not
  complete: `full` says what it holds, and the error says whether its last pass finished, or whether
  its registry or disk has stopped working (see `leader`). `syncerStartedAt` is when the syncer that
  wrote the entry started, set once that syncer has read its own store; the upstream is dropped only
  when the leader's entry carries the start of the syncer running in its pod now.
- `leader` is the replica that fills from the upstream and serves as the replication source
  for the others. The election is deliberately not symmetric: once a replica holds the whole
  expected set, a leader that cannot complete it by itself steps aside for that replica. A leader
  cannot complete the set by itself when there is no upstream, when its last pass failed, when its
  store refuses writes, or when it no longer renews its lease. This is what keeps an air-gapped
  cluster from deadlocking with an empty leader and a full follower. A leader that is still completing the set from the upstream keeps leading:
  a follower copies from it and may finish first, and moving the lease then would gain nothing.
  A replica on a node unreachable for about two minutes is not counted in the election, and a replica whose
  registry stops answering or whose disk stops accepting writes reports it in `healthError` and `error`
  and steps aside for a full replica, if there is one. A leader whose store has lost the set (the
  deployed release or most of a sample of the set is missing on its disk, or `totalDigests` is at most
  half of its `verifiedDigests`) steps aside for a replica full for the current set, reporting
  `full: false` and `verifiedDigests: 0`, and copies the set from it.
  With no such replica it keeps leading, reports `full: false` and `verifiedDigests: 0` all the same,
  and fills the set again from the upstream (in an air-gapped cluster, it counts its store again, and
  the set comes back only with a new `d8 mirror push`).

## Which images are in the cache?

Every image of the cache, by the digest of its manifest, is listed by the RegistryImage resource
of the `registry.deckhouse.io` group. It is not stored anywhere: each request asks every replica of
the cache, and each replica answers from an index it keeps in memory and rebuilds by reading its
store every two minutes and after every operation. The answer is never older than that, and
reading it puts no load on the disks of the master nodes.

```bash
d8 k get registryimages
d8 k get registryimages -o wide
d8 k get registryimages -l registry.deckhouse.io/module=prometheus
d8 k get registryimages -l registry.deckhouse.io/state=Corrupted
d8 k get registryimages -l registry.deckhouse.io/in-set=false
d8 k get registryimage sha256-<digest> -o yaml
d8 k explain registryimages
```

What an image says:

- `owners` — every place the image is used: the module and the image name from
  `images_digests.json`, and the Deckhouse Platform releases or module versions that declare it.
  One digest can be used by several modules, and all of them are listed.
- `inSet` — whether a release or a module of the cluster declares the image. An image outside the
  set was pulled through the cache on request, or was left by an older release; the garbage
  collection removes it. Such images cannot be removed by hand.
- `state` and `replicas[].state` — where the image is on each replica:
  - `Present` — the replica holds the manifest and every blob of the image;
  - `Missing` — an image of the set the replica does not hold at all;
  - `Incomplete` — the replica holds the manifest without some of its blobs, which a pull-through
    cache leaves after it served the manifest alone;
  - `Corrupted` — a blob of the image does not match its digest;
  - `Unknown` — the replica did not answer, or has not read its store since it started. When no
    replica can answer, the request fails with `ServiceUnavailable` and names each replica's reason,
    rather than listing no images.
- `size` — the full size of the image: its manifest and every layer, each counted once.
- `reclaimableSize` — how much the cache would free on a replica if this image alone were removed.

An image is made of layers, and a layer that several images share is stored once. So the sizes of all
images add up to more than the cache takes on disk, and removing one image frees only the layers no
other image uses: `reclaimableSize`, not `size`. The images of the platform share few layers, so
the two are usually close.

The counts by state are also in the status of RegistryStorage, under `replicas[].images`, and stay
there while registry-controller is not running. While it is not running, the `registry.deckhouse.io`
group is not served: `kubectl api-resources` reports an error for it, and namespaces being deleted stay
`Terminating` with the `NamespaceDeletionDiscoveryFailure` condition until a replica of the controller
answers again. The `D8RegistryControllerDown` alert fires then.

Reading RegistryImage needs the `get` and `list` verbs on `registryimages` of the
`registry.deckhouse.io` group; the RBACv2 capability `registry.view` grants them.

### Which images take the most space?

Sort the list by `size` or `reclaimableSize`. kubectl sorts in ascending order, so the largest images
are at the end of the output, and images without a size, such as `Missing` ones, are at the start.
Sorting works together with any label selector.

```bash
# The 20 largest images.
d8 k get registryimages --sort-by=.size | tail -n 20
# The 20 images that take the most space on their own: layers no other image uses.
# With -o wide both sizes are shown.
d8 k get registryimages --sort-by=.reclaimableSize -o wide | tail -n 20
# The largest images outside the set, which the garbage collection removes.
d8 k get registryimages -l registry.deckhouse.io/in-set=false --sort-by=.reclaimableSize -o wide | tail -n 20
# The images of one module by size.
d8 k get registryimages -l registry.deckhouse.io/module=prometheus --sort-by=.size
```

## How do I check and repair images in the cache?

Every replica reads its whole store in the background about once an hour and compares every blob
with its digest. It reads slowly, about 4 MiB/s on a store of 15 GB and never more than 16 MiB/s,
and tells the kernel to drop what it read from the page cache, so that etcd and the API server on
the same master do not notice. Pulls are not blocked. When the last full pass finished is in
`RegistryStorage.status.replicas[].scrubbedAt`.

A blob that does not match its digest is recorded in `RegistryStorage.status.replicas[].corruptedBlobs`,
and the images that use it read `Corrupted`. What happens next needs nobody:

- the replica is no longer `full` while an image of the set uses the broken blob. A leader then
  hands the lease to a replica that holds the set whole, and the nodes pull from it;
- the module creates a RegistryOperation of type `Repair` for the broken blobs as soon as no other
  operation runs. The repair downloads a whole copy of each broken blob from another replica, or,
  while the cluster has an upstream, from the upstream, checks it against its digest and only then
  puts it in place of the broken one. Nothing is removed: a blob for which no whole copy is found
  stays as it was;
- a repair takes up to 50 blobs. A repair that did not fix everything is repeated four hours later,
  with the next 50 when there are more, and after two hours the `D8RegistryStoreCorrupted` alert
  fires.

To check now rather than wait for the background check, create a `Verify`. It reads at up to
32 MiB/s, a store of 15 GB in about eight minutes per replica, one replica at a time:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: RegistryOperation
metadata:
  generateName: verify-
spec:
  type: Verify
  # Optional: only these images or blobs, and only these replicas.
  verify:
    digests: ["sha256:<digest>"]
    nodes: ["<master-node>"]
```

To repair images by hand, for example after pushing a release again with `d8 mirror push`, create
a `Repair` with their digests (up to 50):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: RegistryOperation
metadata:
  generateName: repair-
spec:
  type: Repair
  repair:
    digests: ["sha256:<digest>"]
```

A repair of a whole image answers `Intact` when every blob matches its digest: an image is addressed
by its content, and downloading it again would bring the same bytes. The outcome for each digest is
in `status.replicas[].repair.digests` of the operation. Only one operation runs at a time; one created
while a garbage collection runs fails with `AnotherOperationActive` and has to be created again.

Creating operations needs the RBACv2 capability `registry.edit`.

## How is the cache updated?

A new Deckhouse Platform release can change the images of the cache itself, the `registry-storage`
StatefulSet in `d8-system`. The StatefulSet uses the `OnDelete` update strategy, and its replicas are
replaced by `registry-controller`: one at a time, followers first and the leader last, and only while
every other replica is ready. A replica on the old revision that has never started or is in
`CrashLoopBackOff` is replaced at once.

A replaced replica pulls its new images after it is deleted, from the cache it was part of. So
the controller replaces a replica only while something else can serve them:

- a follower: the leader keeps serving;
- the leader: once a follower holds the whole image set. The lease then goes to whichever replica
  holding the whole set takes it first: a follower holding it or the leader's own replacement, which
  comes back on the same node over the same store;
- any replica: the upstream registry, if the cluster has one.

### One master node in an air-gapped cluster

With a single replica and no upstream, nothing else can serve the images. The new images
therefore have to be on the master before the replica is replaced. The module keeps them there
with the `registry-storage-image-holder` DaemonSet: on every master it runs each image of the
cache's pod as `/pause`. When the images change, the holder pulls the new ones from the running
cache, and as long as it runs them the kubelet does not delete them from the node. Once the
holder on the master runs every new image, the controller replaces the replica, and the new pod
starts from the images already on the node.

While the only replica of the cache restarts, the nodes cannot pull images from it. How long this
takes depends on stopping the old replica and starting the new one. The kubelet retries failed pulls
with a back-off of up to 5 minutes, so some pods may start with an extra delay after the cache is
back.

The holder requests 10m CPU and 10Mi of memory per image on each master, 40m and 40Mi in total. It
holds the images on every master, but the controller only uses it when the cache has one replica
and no upstream.

### Several master nodes in an air-gapped cluster

The leader is replaced last, and only once a follower holds the whole image set. Otherwise the
lease would move to a follower with images missing, and the images only the old leader had would
be gone from the cluster. The image holder does not replace that check. A follower counts only if
it was found full against the same set as the leader: after an update that changes the Deckhouse Platform
release or the modules, a replaced follower still reports the set it held before until it has
copied the new one. Normally this means waiting while a follower copies the set from the leader.
If the leader itself does not hold the whole set, the followers cannot complete it either: they
copy from the leader. Then the update continues only once the Deckhouse Platform release and the
modules the cluster uses are pushed into the cache.

### What the UpdateBlocked event means

While the controller waits, it writes an `UpdateBlocked` event to the StatefulSet with the replica,
the reason and what ends the wait. It writes the event again when what the wait depends on changes,
and otherwise every five minutes while the wait lasts:

```bash
d8 k -n d8-system get events --field-selector involvedObject.name=registry-storage,reason=UpdateBlocked
```

The same reason is the `reason` label of the `d8_registry_storage_update_blocked` metric, and the
`D8RegistryStorageUpdateBlocked` alert fires if the wait lasts longer than an hour:

- `HolderNotReady` — one replica, and the holder on its master does not run every new image yet.
  If its pod is in `ImagePullBackOff`, the cache does not hold an image: push the Deckhouse Platform
  release the cluster is updating to with `d8 mirror push`, and the holder pulls it on its next
  retry. If its pod is in `CrashLoopBackOff` or `CreateContainerError`, the holder has the image
  but cannot run it, and a push does not help: the cause is in the events of the pod. Otherwise the
  holder is still rolling or pulling, and nothing needs to be done.
- `SingleReplicaWithoutHolder` — one replica, and the holder DaemonSet has no pod on its master.
  For a moment this is normal while the DaemonSet creates its pod on the node. If it lasts, find out why the
  DaemonSet does not schedule a pod there:
  `d8 k -n d8-system describe daemonset registry-storage-image-holder`.
- `LeaderWithoutFullFollower` — only the leader remains to be replaced, but no follower holds the whole set. If
  the leader holds it, wait for a follower to finish copying. If the leader reports that it does
  not hold it, pull the Deckhouse Platform release and the modules the cluster uses with
  `d8 mirror pull`, as described in the
  [air-gapped update guide](/products/kubernetes-platform/documentation/v1/guides/airgapped-update.html),
  and push them into the cache with `d8 mirror push`, as in step 2 of
  [Going air-gapped](examples.html#going-air-gapped). If the leader has not reported yet,
  reports an error or cannot tell which set the cluster needs, a push does not help, and the update
  continues only after a pass of the leader succeeds: look at the log of the `syncer` container of
  the leader's pod.
- `LeaderNotServing` — the storage lease names a master without a serving replica. It moves on its
  own once the lease expires.
- `StorageUnknown` — the `RegistryStorage` resource does not exist yet. The controller creates it.
  If it does not appear, check the log of `registry-controller`.

To see what the holders and the replicas are doing:

```bash
d8 k -n d8-system get pods -l app=registry-storage-image-holder -o wide
d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq
```

## Who can read from and write to the in-cluster cache?

The cache's registry accepts three accounts. Each of them is limited to the platform's path,
`system/deckhouse`, and everything under it: that is where the cache keeps the platform, its modules,
and the modules of other ModuleSources. A request for any other repository is refused, whatever the
account.

| Account | Who uses it | What it may do |
|---|---|---|
| `registry-ro` | the node agent on every node | pull; list the store's repositories |
| `registry-sync` | the cache's own pods only | pull, push and delete: filling from the upstream, replication, garbage collection |
| `registry-rw` | `d8 mirror push`, through the publication endpoint | pull and push |

- The read account is held by every node, so it can change nothing the nodes pull.
- The sync account is the one that writes into the store from inside the cluster. Its password is
  used only by the cache's pods: the registry controller and `registry-packages-proxy` read the
  Secret `registry-storage-client`, which carries the certificate authority, the read account and the
  replicas' addresses and nothing else.
- The publication account is for the platform administrator. It is how a new release reaches an
  air-gapped cluster, so it can replace the platform's images by design: whoever can read the Secret
  `registry-storage-push` in `d8-system` can replace what every node pulls. Grant read access to that
  Secret only to those who publish the platform. The account can neither delete images nor list the
  store's repositories: images are removed by the garbage collection.

Push to the platform's path, `<publication endpoint>/system/deckhouse`, as in the
[air-gap example](examples.html#going-air-gapped); a push to any other path is refused.

## The v1 implementation

The information below applies to a cluster still running the v1 implementation of the module (DP up to and including 1.77), configured through the
`deckhouse` ModuleConfig.

### How to Migrate to the registry module?

This procedure moves a cluster from the deprecated format (without the `registry` module) to the
v1 implementation. It is not a step of the migration to the current implementation: a cluster in
the deprecated format needs no preparation for the upgrade to DP 1.78. See
[how the migration works](#how-does-the-migration-to-the-registry-module-work).

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
