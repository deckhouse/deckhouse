---
title: "Module registry: configuration"
description: "How to configure where a Deckhouse Kubernetes Platform cluster pulls its images from."
---

This module decides where the cluster pulls images from, and how.

Its default is to decide nothing. Enabling the module changes nothing on its own: with
`mode: Unmanaged` — the default — no components are created, no node configuration is
written, and the cluster keeps pulling from the registry it was installed with. You turn it
on deliberately, by setting `mode: Managed` together with the registry to pull from.

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
          license: <LICENSE_KEY>
```

There is no choice of implementation to make. A cluster that has never run the previous
implementation of this module uses the current one from the start. A cluster that has runs
the previous one until it is brought to its `Unmanaged` state, after which the migration
completes on its own — see [how to complete the migration](faq.html#how-do-i-complete-the-migration).

{% alert level="info" %}
Settings without `mode: Managed` are rejected rather than ignored. A configuration that is
written down but has no effect is worse than an error: the cluster keeps pulling the way it
did, and nothing says why.
{% endalert %}

## The two things you decide

Everything else follows from two independent answers.

**Where images come from.** `primary.upstream` names one authoritative registry for
Deckhouse component images. Omit it and the cluster is air-gapped: the in-cluster cache
becomes the only source, filled with `d8 mirror push`.

**Whether pulls go through an in-cluster cache.** `storage.cache` deploys a registry on the
control-plane nodes. Every node then reads from it, with the upstream kept as a fallback
while it fills.

Together they cover every supported arrangement:

| `primary.upstream` | `storage.cache` | What the cluster does |
|---|---|---|
| set | `false` | Nodes pull straight from the upstream. Nothing is deployed on the control plane. |
| set | `true` | A pass-through cache, filled from the upstream on demand and ahead of time. |
| omitted | `true` | Air-gapped. The cache is the only source; `d8 mirror push` is the way in. |
| omitted | `false` | Rejected — the nodes would have nowhere to pull from. |

Turning the cache on or off is a safe, idempotent reconfiguration; you can do it at any
time. Removing the upstream while the cache is on is the one change with a condition
attached: it takes effect only once the cache holds the whole expected image set, so nodes
are never cut off. Until then the module keeps using the upstream and says so — see
[`D8RegistryAirGapTransitionHeld`](faq.html#what-do-the-registry-alerts-mean).

The cache reclaims its own disk. Nothing else ever removes anything from it — every release adds
a slice of the repository — so a nightly collection deletes the slices belonging to releases the
cluster has moved past, keeping the deployed release and the one before it. It puts one replica
read-only while it runs, which is why it defaults to a quiet hour; see
[`storage.garbageCollection`](#parameters-storage-garbagecollection) and the
[FAQ](faq.html#the-cache-keeps-growing-what-reclaims-it).

{% alert level="warning" %}
Use a separate disk for the cache (`/opt/deckhouse/registry`) and for etcd data. Sharing one
disk degrades etcd while the cache is being filled.
{% endalert %}

## Additional registries

`primary.upstream` is deliberately the only registry configured here, and it is for
Deckhouse component images. Anything else — a vendor registry a module needs, a registry of
your own — is declared as a separate [RegistryUpstream](cr.html#registryupstream) resource:

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

A resource rather than another field, because a module or a user bringing their own registry
must not have to edit a ModuleConfig owned by someone else. These are always transit: they
are routed through the node agent so that credentials and certificate authorities live in
one place, but they are never cached.

## Pulling from a registry the module knows nothing about

You do not have to declare anything. Once the module manages the pull path, the node agent
is what the container runtime asks about every registry — including registries this module
has never heard of — and an unconfigured one is forwarded untouched, with whatever
credentials the pull already carried.

So an ordinary `imagePullSecret` works exactly as it would on a cluster where this module
was never enabled:

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

Declare a `RegistryUpstream` only when you want the cluster to hold the credentials rather
than each workload, or when the registry needs a certificate authority the nodes do not
have.

<!-- SCHEMA -->
