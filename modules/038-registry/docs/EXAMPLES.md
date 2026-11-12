---
title: "Module registry: usage examples"
description: "Examples for switching between registry modes in Deckhouse Platform."
---

{% alert level="warning" %}
If, during the switching process, the image of a module did not reload and the module did not reinstall, use the [instructions](/products/kubernetes-platform/documentation/v1/faq.html#what-should-i-do-if-the-module-image-did-not-download-and-the-mo) to resolve the issue.
{% endalert %}

## Enabling the module

To have the module manage how the cluster pulls images, set [`mode: Managed`](configuration.html#parameters-mode) and specify the registry to pull from:

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

The module publishes a ready-made version of this for your cluster, with the address, path,
scheme, certificate authority and credentials of the registry it is already pulling from:

```bash
d8 k -n d8-system get secret registry-suggested-config -o jsonpath='{.data.registry-mc\.yaml}' | base64 -d
```

Review it, then apply it. It saves a transcription rather than typing: those values are spread
across a secret and a docker configuration blob, and retyping them by hand is where a
truncated path or the wrong authority comes from — on the one setting that decides whether the
cluster can pull at all.

To see the change take effect, use the following commands:

```bash
d8 k get registryconfig registry -o jsonpath='{.status}' | jq
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,APPLIED:.status.observedGeneration,OK:.status.reconciled,BACKENDS:.status.activeBackends
```

## Turning the in-cluster cache on

To turn on the in-cluster cache, add the module's [`storage.cache`](configuration.html#parameters-storage-cache) ModuleConfig setting and specify a size for the store:

```yaml
spec:
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        auth:
          license: <LICENSE_KEY>
    storage:
      cache: true
      size: 50Gi
```

Nothing on any node is reconfigured. The container runtime already asks the node agent about
every registry, and the agent starts preferring the cache with the upstream as a fallback — so
a cache miss is a slower pull rather than a failed one, from the first moment.

To check how full the in-cluster cache is, use the following command:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Turning the in-cluster cache off again is the same change in reverse, and just as safe. The blobs on disk are left
alone, so turning it on again refills from what is already there rather than from scratch — see
[how to reclaim that space](faq.html#how-do-i-remove-leftover-cache-data-from-a-node)
if you do not intend to.

## Going air-gapped

An air-gapped cluster has no upstream (not set in the [`primary.upstream`](configuration.html#parameters-primary-upstream) parameter). For such a cluster, the cache is the only source of images, and
`d8 mirror push` is the way in. Because completeness has to be decidable before the cache can
be trusted alone, [`storage.source`](configuration.html#parameters-storage-source) must describe the expected image set.

To move a cluster to air-gap, follow these steps:

1. Pull the images on a machine that has internet access:

   ```bash
   d8 mirror pull --license <LICENSE_KEY> ./d8-bundle
   ```

1. Push them into the cluster, through the publication endpoint:

   ```bash
   PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
   d8 mirror push ./d8-bundle registry.example.com/system/deckhouse \
     --registry-login "$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
     --registry-password "$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
   ```

   The publication endpoint exists only while the cache is enabled, `publicDomainTemplate` is set
   and the `cert-manager` module is enabled; see [Air-gapped clusters](/modules/registry/#air-gapped-clusters).

1. Describe the expected set of images in the cache, and remove the upstream:

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

The upstream is not removed from the nodes the moment you remove it from the configuration. It
is removed once the cache leader holds the whole expected set and nothing in the platform
namespaces (`kube-system` and `d8-*`) still references the upstream — this is the one transition
that could otherwise leave every node with nowhere to pull from, so it waits, and says so.
When the upstream is removed, the registry restarts, and the RegistryStorage resource may briefly
report `Failed`: this is expected, wait for it to return to `Ready`.

{% alert level="warning" %}
Workloads in other namespaces are not counted. If the images of your own workloads reference the
upstream address, move them onto the in-cluster address before removing the upstream: once it is
removed, they have nowhere to pull from.
{% endalert %}

To check the status of the transition, use the following commands:

Check whether the cache leader holds the whole expected set of images, and how many platform
workloads still name the upstream (`upstreamReferences`):

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{safeToDropUpstream,fill,upstreamReferences}'
```

A nonzero `upstreamReferences` means that a rollout of platform workloads onto the in-cluster
address has not finished. The `registry-controller` log names up to five of the referencing
objects each time the count changes. To see the last such line, run:

```bash
d8 k -n d8-system logs -l app=registry-controller -c registry-controller --prefix --tail=-1 | grep 'still names the upstream' | tail -1
```

Get the value of `effectiveUpstream`:

```bash
d8 k get registryconfig registry -o jsonpath='{.status.effectiveUpstream}' | jq
```

While `effectiveUpstream` is still set, the cluster is using it. When it becomes empty, the
cluster is air-gapped.

If the transition does not finish, find what holds it:

- The cache is not complete (`safeToDropUpstream: false` with the leader not `full`): wait for the
  fill, or check the `error` of the leader in `.status.replicas`.
- `upstreamReferences` stays above zero: finish or fix the rollout of the objects named in the
  `registry-controller` log.
- The leader is `full` and `upstreamReferences` is `0`: the upstream is held until the `syncer`
  running on the leader's node has read its store and stamped its report with `syncerStartedAt`,
  normally within two minutes of its start. This does not end while the leader's storage pod is
  not Ready, or while that pod runs a `syncer` from before this field existed (during a module
  update). Deleting the leader's storage pod is safe while the upstream is held: the new `syncer`
  reads its store and stamps its report, normally within two minutes. To see the leader's report
  and the reason the controller gives:

  ```bash
  d8 k get registrystorage registry -o json | jq '.status as $s | $s.replicas[] | select(.node == $s.leader) | {node,full,error,syncerStartedAt}'
  d8 k -n d8-system logs -l app=registry-controller -c registry-controller --prefix --tail=-1 | grep 'the upstream is held' | tail -1
  ```

To stay on the upstream instead, put the same `primary.upstream` back into the ModuleConfig: the
cluster has been using it all along and keeps doing so.

## Adding another registry

A registry that is not the source of DP component images is declared as its own
resource — [RegistryUpstream](cr.html#registryupstream) — rather than another field in the ModuleConfig. Example:

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
    scheme: HTTPS
    auth:
      username: robot
      password: <PASSWORD>
```

Pulls naming `images.virtualization.example.com` are then routed to `vendor.example.com/virtualization`
by the agent on every node, with the credentials and certificate authority held by the cluster
rather than by each workload. Nothing on any node is reconfigured for this.

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

## Pulling from a private registry without declaring it

Nothing needs to be declared to pull from a registry the module does not know about. When the
module manages how images are pulled, the node agent is what the container runtime turns to for
any registry, including ones the module has no information about. An unconfigured registry is
proxied untouched, along with whatever credentials the pull request already carried.

So an ordinary `imagePullSecret` behaves exactly as it would on a cluster where this module was
never enabled.

To create a secret with the credentials of the private registry, run:

```bash
d8 k create secret docker-registry my-private-registry \
  --docker-server=private.example.com \
  --docker-username=robot \
  --docker-password=<PASSWORD>
```

To use this secret when pulling images, add it to the pod's `imagePullSecrets`:

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

It is worth creating a `RegistryUpstream` only if you want the cluster to hold the credentials
instead of every workload doing so individually, or if the registry needs a certificate authority
the nodes do not have.

## Turning the module off again

To have the module stop managing how images are pulled, set the [`mode`](configuration.html#parameters-mode) parameter to `Unmanaged`:

```yaml
spec:
  settings:
    mode: Unmanaged
```

The module then manages nothing: its components are removed, the node configuration it wrote is
withdrawn, and the cluster goes back to pulling from the registry recorded in the
`deckhouse-registry` secret — which is where it was pulling from before the module was ever
enabled.

Cache data on the control-plane nodes is deliberately left behind: turning the cache on again
fills in only the missing images from the registry, rather than starting from scratch. If you do
not plan to use the cache again, see
[how to reclaim that space](faq.html#how-do-i-remove-leftover-cache-data-from-a-node).

## Examples for the v1 implementation

Everything below applies to a cluster still running the v1 implementation
(DP up to and including 1.77), configured through the `deckhouse` ModuleConfig.
See [how the migration works](faq.html#how-does-the-migration-to-the-registry-module-work).

### Switching to the `Unmanaged` Mode

To switch an already running cluster to `Unmanaged` mode, follow these steps:

{% alert level="danger" %}
Changing the registry in `Unmanaged` mode will result in a full restart of all DP components.
{% endalert %}

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

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

1. Set the `Unmanaged` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged). If you're using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

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

1. Check the registry switch status in the `registry-state` secret using [this guide](faq.html#how-to-check-the-registry-mode-switch-status).

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

1. If you need to switch back to the old registry management method, refer to the [instruction](faq.html#how-to-migrate-back-from-the-registry-module).

{% alert level="warning" %}
This is a deprecated format for registry management.
{% endalert %}
