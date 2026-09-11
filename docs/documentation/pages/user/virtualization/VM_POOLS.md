---
title: "Virtual machine pools"
permalink: en/user/virtualization/vm-pools.html
description: "Virtual machine pools: creating identical replicas, scaling a pool, deleting specific replicas, and reclaimable disks."
search: VM pool, VirtualMachinePool, replicas, pool scaling, reclaim
---

{% alert level="warning" %}
Available in commercial DP editions.
{% endalert %}

The [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool) resource maintains a given number of identical virtual machines and lets you scale them through the `scale` subresource, HorizontalPodAutoscaler (HPA), or KEDA. The `virtualMachineTemplate.spec` field matches the regular `VirtualMachineSpec`, so a replica is no different from a manually created virtual machine.

{% alert level="warning" %}
The `Legacy` OS type isn't supported in a pool, because replicas are differentiated by initialization, which these operating systems don't have, so every replica would be a byte-for-byte copy of one disk, and for Windows guest operating systems that also means the same SID on the network. A pool template with `osType: Legacy` is rejected. Create such virtual machines individually.
{% endalert %}

The following example shows how to create a virtual machine pool:

{% tabs pool-create %}

{% tab "Using the CLI" %}

Create a pool with the number of replicas you need and a virtual machine template. Pool disks are described in two blocks:

- `virtualDiskTemplates` describes each replica disk once, setting the `reclaim` policy, the size, and the data source.
- The `blockDeviceRefs` of the template references these disks by name with `kind: VirtualDisk` and sets the device order, that is, the boot order, exactly as in a regular [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

Every `virtualDiskTemplates` entry has to appear in `blockDeviceRefs` exactly once, otherwise the module rejects the pool. Disk template names are unique.

Besides replica disks, `blockDeviceRefs` can list shared [VirtualImage](/modules/virtualization/cr.html#virtualimage) and [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) images, for example a single ISO or CD-ROM for all replicas. Such images are attached read-only, there's one of them for the whole pool, and they don't need an entry in `virtualDiskTemplates`.

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachinePool
metadata:
  name: runners
  namespace: ci
spec:
  replicas: 3
  scaleDownPolicy: NewestFirst
  virtualMachineTemplate:
    spec:
      runPolicy: AlwaysOn
      virtualMachineClassName: generic
      cpu:
        cores: 2
      memory:
        size: 4Gi
      # Cloud-init: each replica configures itself at first boot (identically for all).
      provisioning:
        type: UserData
        userData: |
          #cloud-config
          users:
            - name: cloud
              sudo: ALL=(ALL) NOPASSWD:ALL
              ssh_authorized_keys:
                - <SSH_PUBLIC_KEY>
      # Devices and boot order (the first one is bootable). VirtualDisk entries
      # reference virtualDiskTemplates by name (per-replica, resolved by the controller);
      # VirtualImage/ClusterVirtualImage is a shared read-only image for all replicas.
      blockDeviceRefs:
        - kind: VirtualDisk
          name: root          # boot disk
        - kind: VirtualDisk
          name: cache
        - kind: ClusterVirtualImage
          name: tools-iso      # shared CD-ROM, attached to every replica
  # Per-replica disk parameters (reclaim/size/source). Each of them has to be listed above.
  virtualDiskTemplates:
    # Writable root disk: one per replica, cloned from an image, deleted along with the replica.
    - name: root
      reclaim:
        onScaleDown: Delete
      spec:
        persistentVolumeClaim:
          size: 30Gi
        dataSource:
          type: ObjectRef
          objectRef:
            kind: VirtualImage
            name: ubuntu
    # Reusable cache: survives a scale-down and is reattached on a scale-up.
    - name: cache
      reclaim:
        onScaleDown: Retain
        keep: 5
        ttl: 30m
      spec:
        persistentVolumeClaim:
          size: 50Gi
EOF
```

Replicas are named `<POOL>-<RANDOM>`. Disks follow the same scheme, and a per-replica disk (`Delete`) is named `<REPLICA>-<TEMPLATE>` (for example, `runners-1b2e84-root`), while a reusable one (`Retain`) gets the `<POOL>-<TEMPLATE>-<RANDOM>` name. To view the replicas, run `d8 k get vm -l vmpool.virtualization.deckhouse.io/pool=runners`.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **VM pools**.
1. Click **Create**.
1. In the **Create resource** window that opens, enter the pool name in the **Name** field.
1. On the **Configuration** tab, set the number of replicas in the **Replicas** field and the replica deletion policy in the **Scale Down Policy** field.
1. In the **Virtual Disk Templates** block, describe the replica disks, and in the **Virtual Machine Template** block, describe the virtual machine template.
1. Click **Apply**.

> The pool form is built from the [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool) resource specification, so the field names match the resource parameters. You can paste a ready specification on the **YAML** tab.

{% endtab %}

{% endtabs %}

## Attaching a shared CD-ROM (or any shared image) to all replicas

Besides per-replica disks, `blockDeviceRefs` can reference read-only images, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) or [VirtualImage](/modules/virtualization/cr.html#virtualimage). Such an image is shared, and all replicas attach the same file, for example an ISO with tools or drivers. Images aren't listed in `virtualDiskTemplates` (they have no per-replica state) and aren't part of the bijection.

Add the image to `blockDeviceRefs` at the position you need in the boot order. For an installation ISO, put it before the disk, and for a CD-ROM with tools, after it:

```yaml
spec:
  virtualMachineTemplate:
    spec:
      blockDeviceRefs:
        - kind: VirtualDisk           # Writable per-replica root disk, boots first.
          name: root
        - kind: ClusterVirtualImage   # Shared read-only CD-ROM, attached to every replica.
          name: tools-iso
  virtualDiskTemplates:
    - name: root
      spec:
        persistentVolumeClaim:
          size: 30Gi
        dataSource:
          type: ObjectRef
          objectRef:
            kind: ClusterVirtualImage
            name: ubuntu
```

An image is attached to existing replicas the same way as any other device, and a change to `blockDeviceRefs` applies to a live replica the next time it's recreated (rotation or scale-up).

## Scaling the pool

The number of replicas in a pool changes either manually or automatically, by an autoscaler.

{% tabs pool-scale %}

{% tab "Using the CLI" %}

A pool supports the standard `scale` subresource, compatible with manual replica count changes and with autoscalers.

To change the number of replicas manually, run the following command:

```bash
d8 k scale virtualmachinepool/runners -n ci --replicas=8
```

The pool publishes `status.selector`, so HPA reads CPU and memory metrics straight from the replicas without extra plumbing:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: runners
  namespace: ci
spec:
  scaleTargetRef:
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachinePool
    name: runners
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

Besides CPU and memory, the pool works with custom metrics (`Pods`/`External` through `custom.metrics.k8s.io`/`external.metrics.k8s.io`) and with KEDA, for example to scale by the length of an external queue. With `scaleDownPolicy: Explicit`, an autoscaler can only increase the number of replicas, an unaddressed scale-down through the `scale` subresource is rejected, and replicas are removed by name.

The `spec.scaleDownPolicy` field determines which replica is deleted on an unaddressed scale-down:

- `NewestFirst`: The youngest replicas are deleted first.
- `OldestFirst`: The oldest replicas are deleted first.
- `Explicit`: An unaddressed scale-down is forbidden; replicas can be removed only by name. Use it when only the caller knows which replica can be safely removed (for example, an idle one).

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **VM pools**.
1. Select the pool you need from the list and click its name.
1. On the **Configuration** tab, set the new value in the **Replicas** field.
1. Click **Apply**.
1. The scaling progress is shown in the pool list, in the **Status** and **Ready** columns.

{% endtab %}

{% endtabs %}

## Deleting specific replicas

By default, when a pool scales down, the controller picks which replica to delete itself.

To remove exactly the replicas you specify (and scale the pool down by that number), use the `scaleDownWith` subresource:

```bash
d8 k create --raw \
  /apis/subresources.virtualization.deckhouse.io/v1alpha2/namespaces/ci/virtualmachinepools/runners/scaledownwith \
  -f - <<'EOF'
{"targets": ["runners-1b2e84", "runners-9c0d11"]}
EOF
```

A plain `d8 k delete vm` doesn't scale the pool down, because the controller treats it as a lost replica and creates a replacement.

## Reusable disks (reclaim)

The `reclaim` policy sets what happens to a replica disk when the replica is removed from the pool.

The `reclaim.onScaleDown` parameter of a `virtualDiskTemplates` element defines this behavior. `reclaim` is optional; if it isn't set, the disk is treated as `Delete`.

- `Delete` (default): The disk belongs to the virtual machine and is deleted along with it; nothing is left after the replica.
- `Retain`: The disk belongs to the pool, survives the replica, and is reattached to the next one on a scale-up. It suits state that's expensive to recreate and has to survive VM recreation, so that scaling back up is warm rather than cold.

`keep` and `ttl` configure the pool of free `Retain` disks (they apply only to `Retain`):

- `keep`: How many recently freed disks to always keep warm for an instant scale-up. `ttl` doesn't apply to them.
- `ttl`: How long a free disk lives beyond the warm buffer before garbage collection.

Examples:

```yaml
# Ephemeral disk: deleted along with the replica (Delete by default).
- name: root
  spec:
    persistentVolumeClaim: { size: 30Gi }
    dataSource: { type: ObjectRef, objectRef: { kind: VirtualImage, name: ubuntu } }

# Reusable disk: keep 3 warm for a fast scale-up, collect the rest after 1h of idling.
- name: cache
  reclaim:
    onScaleDown: Retain
    keep: 3
    ttl: 1h
  spec:
    persistentVolumeClaim: { size: 100Gi }

# Reusable disk without a limit: always reused, never deleted automatically (no ttl).
- name: data
  reclaim:
    onScaleDown: Retain
  spec:
    persistentVolumeClaim: { size: 20Gi }
```

Invalid combinations are rejected on creation and modification. The `keep` and `ttl` parameters are allowed only with `Retain`, and `keep > 0` requires `ttl`, because without `ttl` nothing is collected and `keep` has no effect. A `Retain` disk without `ttl` keeps all freed disks indefinitely; limit it with `ttl` if that isn't what you need.

## Pool limitations and specifics

Here are the limitations and non-obvious pool behaviors worth remembering during operation.

- Removing an entry from `virtualDiskTemplates` deletes its disks. For `Retain` disks, this destroys reusable data, so remove a template only when it's no longer needed.
- The pool maintains the number of replicas, not their health. An existing but unhealthy VM isn't recreated, a restart at the VM level brings it back. A `Stopped` replica is preserved rather than replaced, and only a fully deleted replica is recreated.
- `Retain` disks are shared between replicas. On a scale-up, a new replica can get a freed disk of another replica along with its data; there's no hard binding between a replica and a disk.
- A change to `virtualDiskTemplates[].spec` affects only new disks, except for `size`, which grows existing ones (shrinking isn't allowed). `dataSource`, `storageClassName`, and the rest don't apply to disks that already exist.
- Each replica has its own copy of every disk from `virtualDiskTemplates`. A shared read-only image, [VirtualImage](/modules/virtualization/cr.html#virtualimage) or [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), for example a single ISO, can be attached to all replicas by listing it in the `blockDeviceRefs` of the template, while a writable disk isn't shared between replicas.
- An edit to the `blockDeviceRefs` of the template (reordering, adding, or removing a shared image) applies to new replicas; live replicas keep their current devices until they're recreated (rotation or scale-up), as with other template changes that require a restart.
- Template changes that require a restart apply only after the replica restarts, according to [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) in the template.
