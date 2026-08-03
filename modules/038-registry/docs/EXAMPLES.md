---
title: "Module registry: usage example"
description: "Examples for switching between registry modes in Deckhouse Kubernets Platform."
---

{% alert level="warning" %}
If, during the switching process, the image of a module did not reload and the module did not reinstall, use the [instructions](/products/kubernetes-platform/documentation/v1/faq.html#what-should-i-do-if-the-module-image-did-not-download-and-the-mo) to resolve the issue.
{% endalert %}

## Enabling the module

To have the module manage how the cluster pulls images, set `mode: Managed` and name the
registry to pull from:

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

Watch it take effect:

```bash
d8 k get registryconfig registry -o jsonpath='{.status}' | jq
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,APPLIED:.status.observedGeneration,OK:.status.reconciled,BACKENDS:.status.activeBackends
```

## Turning the in-cluster cache on

Add `storage.cache` and a size for the store:

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

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Turning it back off is the same change in reverse, and just as safe. The blobs on disk are left
alone, so turning it on again refills from what is already there rather than from scratch — see
[how to reclaim that space](faq.html#a-node-still-holds-cache-data-nothing-uses)
if you do not intend to.

## Going air-gapped

An air-gapped cluster has no upstream: the cache is the only source of images, and
`d8 mirror push` is the way in. Because completeness has to be decidable before the cache can
be trusted alone, `storage.source` says what the expected image set is.

1. Pull the images somewhere with internet access:

   ```bash
   d8 mirror pull --license <LICENSE_KEY> ./d8-bundle
   ```

1. Push them into the cluster, through the publication endpoint:

   ```bash
   PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
   d8 mirror push ./d8-bundle registry.example.com/system/deckhouse \
     --username "$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
     --password "$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
   ```

1. Declare what the cache is expected to hold, and remove the upstream:

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
is removed once the cache leader holds the whole expected set — this is the one transition that
could otherwise leave every node with nowhere to pull from, so it waits, and says so:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{safeToDropUpstream,fill}'
d8 k get registryconfig registry -o jsonpath='{.status.effectiveUpstream}' | jq
```

While `effectiveUpstream` is still set, the cluster is using it. When it becomes empty, the
cluster is air-gapped.

## Adding another registry

A registry that is not the source of Deckhouse component images is declared as its own
resource, not as another field in the ModuleConfig:

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

Check that it was accepted — a conflict with the primary registry, or with another resource
claiming the same name, is refused rather than merged:

```bash
d8 k get registryupstreams -o custom-columns=\
NAME:.metadata.name,MATCH:.spec.match,ACCEPTED:.status.conditions[0].status,REASON:.status.conditions[0].reason
```

## Pulling from a private registry without declaring it

Nothing needs to be declared. The node agent forwards a registry it does not know about
untouched, along with whatever credentials the pull already carried, so an ordinary
`imagePullSecret` behaves exactly as it would on a cluster where this module was never enabled:

```bash
d8 k create secret docker-registry my-private-registry \
  --docker-server=private.example.com \
  --docker-username=robot \
  --docker-password=<PASSWORD>
```

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

Declare a `RegistryUpstream` instead when you want the cluster to hold the credentials, or when
the registry needs a certificate authority the nodes do not have.

## Turning the module off again

Set the mode back to `Unmanaged`:

```yaml
spec:
  settings:
    mode: Unmanaged
```

The module then manages nothing: its components are removed, the node configuration it wrote is
withdrawn, and the cluster goes back to pulling from the registry recorded in the
`deckhouse-registry` secret — which is where it was pulling from before the module was ever
enabled.

Cache data on the control-plane nodes is deliberately left behind, so that turning the cache on
again refills from what is already there. See
[how to reclaim that space](faq.html#a-node-still-holds-cache-data-nothing-uses).

## Examples for the previous implementation

Everything below applies to a cluster still running the implementation configured through the
`deckhouse` ModuleConfig. See [how to complete the migration](faq.html#how-do-i-complete-the-migration).

### Switching to the `Direct` Mode

To switch an already running cluster to `Direct` mode, follow these steps:

{% alert level="danger" %}
The first switch from `Unmanaged` to `Direct` mode will result in a full restart of all DKP components.
{% endalert %}

1. Before switching, perform the [migration to use the `registry` module](faq.html#how-to-migrate-to-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
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

1. Set the `Direct` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-direct). If you're using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

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
         mode: Direct
         direct:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
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
   mode: Direct
   target_mode: Direct
   ```

### Switching to the `Proxy` Mode

To switch an already running cluster to `Proxy` mode, follow these steps:

{% alert level="danger" %}
- The first switch from `Unmanaged` to `Proxy` mode will result in a full restart of all DKP components.
- Switching from `Local` mode to `Proxy` mode is not available. To switch from `Local` mode, you must switch the registry to another available mode (for example: `Direct`).
{% endalert %}

1. Before switching, perform the [migration to use the `registry` module](faq.html#how-to-migrate-to-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
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

1. Set the `Proxy` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-proxy). If you're using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

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
         mode: Proxy
         proxy:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
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
   mode: Proxy
   target_mode: Proxy
   ```

### Switching to the `Local` Mode

To switch an already running cluster to `Local` mode, follow these steps:

{% alert level="danger" %}
- The first switch from `Unmanaged` to `Local` mode will result in a full restart of all DKP components.
- Switching from `Proxy` mode to `Local` mode is not available. To switch from `Proxy` mode, you must switch the registry to another available mode (for example: `Direct`).
{% endalert %}

1. Before switching, perform the [migration to use the `registry` module](faq.html#how-to-migrate-to-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
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

1. Prepare archives with DKP images of the current version. To do this, use the `d8 mirror` command.

   Example:

   ```bash
   TAG=$(
    d8 k -n d8-system get deployment/deckhouse -o yaml \
    | yq -r '.spec.template.spec.containers[] | select(.name == "deckhouse").image | split(":")[-1]'
   ) && echo "TAG: $TAG"

   EDITION=$(
    d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller global values -o yaml \
    | yq .deckhouseEdition
   ) && echo "EDITION: $EDITION"
   ```

   ```bash
   d8 mirror pull \
   --license="<LICENSE_KEY>" \
   --source="registry.deckhouse.io/deckhouse/$EDITION" \
   --deckhouse-tag="$TAG" \
   /home/user/d8-bundle
   ```

1. Set the `Local` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-mode).

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
         mode: Local
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](faq.html#how-to-check-the-registry-mode-switch-status). In the status, you need to wait for the `RegistryContainsRequiredImages` check to start. The condition will show the absence or presence of images in the running local registry.

   Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: |-
       Mode: Default
       master-1: 0 of 166 items processed, 166 items with errors:
       - source: module/control-plane-manager/control-plane-manager133
         image: 10.128.0.5:5001/system/deckhouse@sha256:00202db19b40930f764edab5695f450cf709d50736e012055393447b3379414a
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:00202db19b40930f764edab5695f450cf709d50736e012055393447b3379414a: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)
       - source: module/cloud-provider-yandex/cloud-metrics-exporter
         image: 10.128.0.5:5001/system/deckhouse@sha256:05517a86fcf0ec4a62d14ed7dc4f9ffd91c05716b8b0e28263da59edf11f0fad
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:05517a86fcf0ec4a62d14ed7dc4f9ffd91c05716b8b0ed86d6a1f465f4556fb8: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)
       - source: module/control-plane-manager/kube-controller-manager132
         image: 10.128.0.5:5001/system/deckhouse@sha256:13f24cc717698682267ed2b428e7399b145a4d8ffe96ad1b7a0b3269b17c7e61
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:13f24cc717698682267ed2b428e7399b145a4d8ffe96ad1b7a0b3269b17c7e61: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)

         ...and more
     reason: Processing
     status: "False"
     type: RegistryContainsRequiredImages
   ```

1. Upload the images to the local registry using the `d8 mirror` command. Image upload to the local registry is performed via Ingress at `registry.${PUBLIC_DOMAIN}`.

   Get the password for the read-write user of the local registry:

   ```bash
   $ d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
   name: rw
   password: KFVxXZGuqKkkumPz
   passwordHash: $2a$10$Phjbr6iinLf00ZZDD2Y7O.p9H3nDOgYzFmpYKW5eydGvIsdaHQY0a
   ```

   Upload images to the local registry:

   ```bash
   d8 mirror push \
   --registry-login="rw" \
   --registry-password="KFVxXZGuqKkkumPz" \
   /home/user/d8-bundle \
   registry.${PUBLIC_DOMAIN}/system/deckhouse
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](faq.html#how-to-check-the-registry-mode-switch-status). After uploading the images, the `RegistryContainsRequiredImages` status should be in the `Ready` state.

   Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: |-
       Mode: Default
       master-1: all 166 items are checked
     reason: Ready
     status: "True"
     type: RegistryContainsRequiredImages
   hash: ..
   mode: Direct
   target_mode: Local
   ```

1. Wait for the switch to complete. To check the switch status, use [this guide](faq.html#how-to-check-the-registry-mode-switch-status).

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
   mode: Local
   target_mode: Local
   ```

### Switching to the `Unmanaged` Mode

To switch an already running cluster to `Unmanaged` mode, follow these steps:

{% alert level="danger" %}
Changing the registry in `Unmanaged` mode will result in a full restart of all DKP components.
{% endalert %}

1. Before switching, perform the [migration to use the `registry` module](faq.html#how-to-migrate-to-the-registry-module).

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
           license: <LICENSE_KEY> # Replace with your license key
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
