---
title: "Registry Module"
description: "How a Deckhouse Platform cluster pulls container images: through an in-cluster cache, from an upstream registry, or fully air-gapped."
---

The module is used to manage the pull paths for container images of Deckhouse Platform (DP) components, and for images from additional registries (vendor or your own).

The current and previous implementations of the module are described below.

{% alert level="info" %}
A cluster where the previous implementation of this module has never run uses the current one right away. A cluster where the previous implementation has run keeps using it until it is switched to the `Unmanaged` state, after which the migration completes on its own. The process is described in [How does the migration to the registry module work](faq.html#how-does-the-migration-to-the-registry-module-work).
{% endalert %}

## Current implementation

### Modes of operation

The module can run in one of the following modes (selected with the [`mode`](configuration.html#parameters-mode) parameter):

- `Unmanaged` (the default mode). In this mode the module does not manage the pull paths of DP components. With [`mode: Unmanaged`](configuration.html#parameters-mode) the cluster keeps pulling images from the registry it was installed with (set during cluster bootstrap in the [`deckhouse`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse) parameter in InitConfiguration). None of the module's components are created in this mode.
- `Managed`. In this mode the module manages the pull paths for container images. The cluster administrator hands the pull path to the module by setting the `mode: Managed` parameter.

  In this mode (when the module manages the pull paths) the following independent settings are used:

  - [`primary.upstream`](configuration.html#parameters-primary-upstream) — the registry images are pulled from. If not set, the cluster is considered air-gapped: the only source becomes the in-cluster cache, filled with the `d8 mirror push` command.
  - [`storage.cache`](configuration.html#parameters-storage-cache) — controls the in-cluster cache on the control-plane nodes. If set, a registry is deployed on the control-plane nodes. All nodes pull images from it, with the upstream staying a fallback path while the cache is being filled.

  Turning the cache on or off, changing the registry, changing credentials — each of these is an ordinary reconfiguration, possible at any time and in any order. Exactly one change has a condition attached: when the [upstream](configuration.html#parameters-primary-upstream) is removed to go air-gapped, the module waits until the in-cluster cache holds the whole expected image set — only then do images stop being pulled from the registry set in `primary.upstream`. While this is in progress, the [`D8RegistryAirGapTransitionHeld`](faq.html#what-do-the-registry-alerts-mean) alert fires. If the switch to the in-cluster cache happened right after the upstream was removed, every node could be left with nowhere to pull images from.

  The cache reclaims its own disk space. Nothing else ever removes data from it — every release adds a slice of the repository — so a nightly sweep removes the slices of releases the cluster has already passed through, keeping the deployed release and the one before it. One replica goes read-only while this runs, which is why the sweep defaults to a nighttime hour. For details, see the [`storage.garbageCollection`](configuration.html#parameters-storage-garbagecollection) parameter and the FAQ section [The cache keeps growing. What reclaims it](faq.html#the-cache-keeps-growing-what-reclaims-it).

The [`primary.upstream`](configuration.html#parameters-primary-upstream) and [`storage.cache`](configuration.html#parameters-storage-cache) parameters together cover every supported configuration:

| `primary.upstream` | `storage.cache` | What the cluster does |
|---|---|---|
| set | `false` | Nodes pull images directly from the upstream. Nothing is deployed on the control-plane nodes |
| set | `true` | The cache acts as a pass-through: filled from the upstream on demand and ahead of time |
| not set | `true` | Air-gapped. The cache is the only source, and `d8 mirror push` is the only way in |
| not set | `false` | The configuration is rejected: nodes would have nowhere to pull images from |

### Additional registries

`primary.upstream` is deliberately the only registry configurable through the module. It is used for the images of DP components. Additional registries (a vendor's, needed by some module, or your own) are declared as a separate [RegistryUpstream](cr.html#registryupstream) resource. Example:

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

This approach — declaring an additional registry as a separate resource — was chosen because a module or a user adding their own registry should not have to edit a ModuleConfig they don't own. Such registries are always transit: pulls are routed through the node agent so that credentials and CA certificates live in one place, but nothing from them is ever cached.

### Architecture

<!--- Source: mermaid code from docs/internal/MANAGED.md --->

#### The node agent

On every node of the cluster, an agent (registry-agent) is deployed as a static pod. The container runtime is pointed at it (`127.0.0.1:5001`, through containerd's `_default` fallback directory) and gets access to registries through it, passing the original registry name along in the request.

This achieves the following:

- **Node configuration is static.** Adding a registry, turning the in-cluster cache on, rotating credentials — none of this changes the configuration on any node, because the container runtime accesses registries through the agent.
- **Registry credentials stay out of the runtime's configuration files.** They are held by the agent and attached to every request.
- **Credentials are not stored in the module's resources.** A RegistryNode, a RegistryStorage, and the recorded effective upstream name a key in one Secret instead of carrying the credential itself. These are cluster-scoped resources, and the permission to read them is granted to every node, so a credential inside any of them would be readable through the API by every kubelet in the cluster. Each component's access is narrowed to that one Secret by name, and the credential it resolved is kept together with its own copy of the routing.
- **Pulling images works when the API server is unavailable.** The agent keeps a copy of its routing on disk and falls back to it, because the images it serves include the ones needed to repair a broken control plane. A node that could not reach the API server uses the routing it was installed with.

An unconfigured registry is proxied untouched, along with whatever credentials the request already carried — so an ordinary `imagePullSecret` for a third-party registry keeps working, with nothing needing to be declared for it.

#### The in-cluster cache

With [`storage.cache`](configuration.html#parameters-storage-cache) enabled, a registry runs on each control-plane node. It stores blobs (the files an image is made of) in the `/opt/deckhouse/registry` directory. One control-plane node holds the lease and fills from the upstream; the others replicate data from the leading replica (the one holding the lease) ahead of time, so losing the leader does not mean filling from scratch. For details on how the leading replica is chosen, see [Leader election](#leader-election).

Agents reach a replica by the node's address, not by the service name `registry.d8-system.svc:5001`.
`registry.d8-system.svc:5001` is what every image reference in the cluster is built from, and what a request is matched against. But the agent runs in the host's network namespace, where the cluster DNS name does not resolve.

That address (which agents use to reach a replica) takes effect in two steps, and the order matters. Nodes are given it as soon as
the module starts managing the pull path, through the same bashible rollout that installs the
agent. The image references of the platform's own components move only after every node's agent
has applied the configuration it was given: nothing resolves this address except the agent,
so re-rendering the cluster onto it any earlier would point every workload at something that
cannot be pulled yet. Until then, references keep naming the registry the cluster was
installed with — which is also where they return if the module is set to `Unmanaged` or
disabled. Whether this step has happened is visible from the presence of the ConfigMap
`registry-image-address` in `d8-system`.

The Deckhouse controller pulls through the same agent — at `127.0.0.1:5001` — its own release
channel and the module sources it owns. A different address, but the same agent: the
controller is a process and has to dial a connection, whereas an image reference is resolved by
the container runtime, which a drop-in redirects to the agent, so it can
name a service that nothing ever connects to. This is exactly what makes changing the registry a single
change: the configuration is edited in one place, the agents are reconfigured, and both what the nodes pull
and what the controller fetches follow — with no registry address for the
controller written down anywhere, and so none that could be left pointing at the old one.

{% alert level="warning" %}
Use a separate disk for the cache (`/opt/deckhouse/registry`) and for etcd data. Sharing one
disk degrades etcd while the cache is being filled.
{% endalert %}

##### Leader election

Only a replica that already holds the whole expected image set can become the leader. The replica currently holding the lease steps aside once the expected set appears on another replica. A replica that does not hold the full expected image set cannot become the leader.

This rules out problems when the cluster goes air-gapped. If a replica without the full expected image set could become the leader, an air-gapped cluster could get stuck: a `d8 mirror push` request would land on it (because the ingress picked it), and:

- the other replicas could not replicate the expected image set (since the leader does not have it);
- the leader could not obtain the full image set either (since the cluster is air-gapped).

#### Air-gapped clusters

If no upstream is set in [`primary.upstream`](configuration.html#parameters-primary-upstream), the in-cluster cache is the only source of images, and
[`d8 mirror push`](/products/kubernetes-platform/documentation/v1/cli/d8/) is the only way into the registry. It arrives through a publication endpoint that requires
a client certificate from the ingress in addition to credentials: this is the one path that can replace an image, and
a leaked password must not be enough to use it.

The endpoint exists only in an air-gapped cluster. A cluster that has image caching enabled but is not air-gapped does not
get an internet-facing write point into the registry that it never asked for.

### Requirements for using the module

Before enabling the `registry` module to manage the pull paths for container images, make sure the cluster meets the following requirements:

- Nodes use containerd or containerd v2, set with the
  [`defaultCRI`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri)
  parameter in ClusterConfiguration.
- The cluster is fully managed by DP. The module does not work in Managed Kubernetes.
- The in-cluster cache uses disks on the control-plane nodes and is supported on static clusters.

## The previous implementation

The previous implementation of the module is described below; it is planned to be phased out in the future. It is kept
because clusters are still running on it, and moving to the new configuration is work for the administrator that has to be
done before the cluster is upgraded, not after.

{% alert level="danger" %}
Prepare the cluster BEFORE upgrading to this release: the upgrade is blocked while the cluster uses the previous
implementation and the module is running in `Proxy` or `Local` mode, or in `Direct` mode without a configured
`registry` ModuleConfig.
{% endalert %}

Installing DP or upgrading to this release does not create any objects of the previous implementation. Switching the
previous implementation's modes is performed by the code of the previous release — so all preparation happens before moving to this release. Preparation has the following specifics:

- If the cluster uses the previous implementation of the `registry` module running in `Proxy` mode, it must be switched to `Unmanaged` mode.
- If a cluster with the previous implementation of the `registry` module is air-gapped (the module runs in `Local` mode), switching straight to `Unmanaged` mode does not work. Air-gapped clusters have a separate procedure, described in
[How do I migrate an air-gapped cluster from Local mode](faq.html#how-do-i-migrate-an-air-gapped-cluster-from-local-mode).
- If the cluster uses the previous implementation of the `registry` module running in `Direct` mode, no mode switch is needed — only this module's configuration, written
before the upgrade. Its objects are kept deliberately — the previous release marks them to survive
its own removal — and they keep serving the in-cluster address nodes pull through, until this module's agent has taken that address over. The procedure is described in
[migrating from Direct mode](faq.html#how-do-i-migrate-from-direct-mode).

In the previous implementation of the module, working with the registry is configured not through the module's own settings, but through the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry), and it works as a state machine with four modes:

- `Direct` — direct access to an external registry through the fixed address
  `registry.d8-system.svc:5001/system/deckhouse`. The fixed address is what keeps Deckhouse
  images from being re-downloaded, and components from restarting, when registry
  parameters change.
- `Proxy` — an internal caching proxy registry on the control-plane nodes, reachable at that same
  fixed address; it reduces the number of requests to the external registry.
- `Local` — a full local copy of the registry inside the cluster, for air-gapped environments.
- `Unmanaged` — no in-cluster registry; the cluster reaches the external one directly. Comes in
  a configurable form, managed through the `deckhouse` ModuleConfig, and a deprecated
  non-configurable form, set at
  [installation](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo).

Of the four previous-implementation modes listed above, two survive an upgrade to this release, for different reasons. `Unmanaged` is the
mode whose objects served nothing to begin with, so removing them takes nothing away, and the
handover happens on its own. `Direct` survives because the address its nodes pull images
through is also served by the `registry` module: the previous release marks the objects behind that address
so that they outlive its own removal, and they keep serving it until the
agent takes it over — meaning such a cluster needs this module's configuration rather than a mode
switch. Both paths are described in [how does the migration to the `registry` module work](faq.html#how-does-the-migration-to-the-registry-module-work). The two
implementations never manage a cluster at the same time: whichever one is active decides what gets
created at all, so there is no state where both configure the same node.

`Proxy` and `Local` modes leave nodes in a state this release cannot correctly account for or carry over. So when the previous implementation of the module is running in these modes, it is switched to `Unmanaged` first. And that switch is only possible on a DP version that still contains the old `registry` implementation — that is, before upgrading to a version where that implementation is gone. The
`D8RegistryMigrationPreflightBlocked` alert reports the same check from inside the cluster when the migration is not possible.

### Mode switching restrictions

The restrictions on switching modes in the previous implementation of the module are as follows:

- Changing registry parameters and switching modes is only available after the bootstrap phase
  is fully complete.
- For the first switch, migrate the user's registry configurations first — the procedure is described in the
  [FAQ](faq.html).
- Switching to the non-configurable `Unmanaged` mode is only available from `Unmanaged`.
- Switching between `Local` and `Proxy` is only possible through an intermediate `Direct` or
  `Unmanaged` mode. For example: `Local` → `Direct` → `Proxy`.
- Bootstrap in `Local` and `Proxy` modes is supported only on static clusters.

### Direct mode architecture

In `Direct` mode, requests to the registry are processed without intermediate caching. CRI requests
are redirected based on the containerd configuration. Components that access the registry
directly — `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others
— go through the in-cluster proxy on the control-plane nodes.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](images/direct-en.png)

### Proxy mode architecture

{% alert level="warning" %}
It is recommended to use separate disks for storing registry data
(`/opt/deckhouse/registry`) and etcd data. Using a single disk may lead to etcd performance
degradation while the registry is operating.
{% endalert %}

The caching proxy registry runs as static pods on the control-plane nodes and stores cached
data in `/opt/deckhouse/registry`. A load balancer sits in front of it on each node, and the containerd
configuration points at that load balancer. Components that access the registry
directly go through the caching proxy registry.

<!--- Source: mermaid code from docs/internal/PROXY.md --->
![proxy](images/proxy-en.png)

### Local mode architecture

{% alert level="warning" %}
It is recommended to use separate disks for storing registry data
(`/opt/deckhouse/registry`) and etcd data. Using a single disk may lead to etcd performance
degradation while the registry is operating.
{% endalert %}

`Local` mode keeps a full copy of the registry inside the cluster, synchronized between replicas
on the control-plane nodes and populated with the
[`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) tool using
`d8 mirror push`/`d8 mirror pull`. Otherwise it works the same way as the caching proxy.

<!--- Source: mermaid code from docs/internal/LOCAL.md --->
![local](images/local-en.png)
