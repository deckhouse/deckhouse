---
title: "Virtual machine pools"
permalink: en/virtualization-platform/documentation/user/resource-management/virtual-machine-pools.html
---

{% alert level="warning" %}
Available in the EE and SE+ editions.
{% endalert %}

The [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool) resource maintains a requested number of identical virtual machines and lets you scale them via the `scale` subresource, a HorizontalPodAutoscaler (HPA), or KEDA. Its `virtualMachineTemplate.spec` is an ordinary `VirtualMachineSpec`, so a replica is no different from a manually created virtual machine.

Create a pool with the desired number of replicas and a template. Each per-replica disk is described once in `virtualDiskTemplates` (reclaim policy, size, data source), and the template's `blockDeviceRefs` references those disks — by name, with `kind: VirtualDisk` — to set the device (boot) order, exactly as in a plain `VirtualMachine`. Every `virtualDiskTemplates` entry must be referenced exactly once (admission enforces this bijection; disk-template names are unique). Alongside the per-replica disks you may list shared read-only images (`VirtualImage`/`ClusterVirtualImage`) — for example a common ISO/CD-ROM attached to every replica — they are not per-replica and need no `virtualDiskTemplates` entry.

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
      # Cloud-init: every replica self-configures on first boot (same for all).
      provisioning:
        type: UserData
        userData: |
          #cloud-config
          users:
            - name: cloud
              sudo: ALL=(ALL) NOPASSWD:ALL
              ssh_authorized_keys:
                - ssh-ed25519 AAAAC3Nz... user@example
      # Devices and boot order (first = boot). VirtualDisk entries reference
      # virtualDiskTemplates by name (per-replica, resolved by the controller);
      # a VirtualImage/ClusterVirtualImage is shared read-only by every replica.
      blockDeviceRefs:
        - kind: VirtualDisk
          name: root          # boot disk
        - kind: VirtualDisk
          name: cache
        - kind: ClusterVirtualImage
          name: tools-iso      # shared CD-ROM attached to every replica
  # Per-replica disk parameters (reclaim/size/source). Each must be referenced above.
  virtualDiskTemplates:
    # Writable root disk: one per replica, cloned from an image, removed with the replica.
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
    # Reusable cache: survives scale-down and is reattached on scale-up.
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

Replicas are named `<pool>-<random>`. Disks follow the same scheme: a per-replica (`Delete`) disk is named `<replica>-<template>` (for example `runners-1b2e84-root`), a reusable (`Retain`) disk is named `<pool>-<template>-<random>`. List replicas with `d8 k get vm -l vmpool.virtualization.deckhouse.io/pool=runners`.

## Attaching a shared CD-ROM (or any shared image) to every replica

Besides the per-replica disks, `blockDeviceRefs` may reference read-only images — a `ClusterVirtualImage` or a `VirtualImage`. Such an image is not per-replica: every replica attaches the same image (for example a common ISO with tools or drivers). Images are not listed in `virtualDiskTemplates` (they have no per-replica state) and are not subject to the bijection.

Add the image to `blockDeviceRefs` at the position where you want it in the boot order — for an OS-installation ISO, place it before the disk; for a tools CD-ROM, after:

```yaml
spec:
  virtualMachineTemplate:
    spec:
      blockDeviceRefs:
        - kind: VirtualDisk           # per-replica writable root, boots first
          name: root
        - kind: ClusterVirtualImage   # shared read-only CD-ROM, attached to every replica
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

The image is attached to existing replicas the same way as any other device — a change to `blockDeviceRefs` applies to a live replica on its next recreation (rotation or scale-up).

## Scaling

The pool supports the standard `scale` subresource, which works with manual replica changes and autoscalers.

To change the number of replicas manually, run:

```bash
d8 k scale virtualmachinepool/runners -n ci --replicas=8
```

The pool publishes `status.selector`, so an HPA reads CPU/memory metrics from the replicas directly without extra wiring:

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

Beyond CPU/memory, the pool also works with custom metrics (`Pods`/`External` via `custom.metrics.k8s.io`/`external.metrics.k8s.io`) and KEDA, for example to scale on an external queue length. With `scaleDownPolicy: Explicit` an autoscaler can only scale up: anonymous scale-down through the `scale` subresource is rejected (remove replicas by name, see below).

The `spec.scaleDownPolicy` field selects which replica is removed on anonymous scale-down:

- `NewestFirst`: the youngest replicas are removed first.
- `OldestFirst`: the oldest replicas are removed first.
- `Explicit`: anonymous scale-down is rejected; replicas can be removed only by name (see below). Use it when only the caller knows which replica is safe to remove (for example, an idle one).

## Removing specific replicas

By default, when the pool shrinks the controller chooses which replica to remove.

To remove particular replicas (and shrink the pool by that count), use the `scaleDownWith` subresource:

```bash
kubectl create --raw \
  /apis/subresources.virtualization.deckhouse.io/v1alpha2/namespaces/ci/virtualmachinepools/runners/scaledownwith \
  -f - <<'EOF'
{"targets": ["runners-1b2e84", "runners-9c0d11"]}
EOF
```

A plain `kubectl delete vm` does not shrink the pool: the controller treats it as a lost replica and creates a replacement.

## Reusable disks (`reclaim`)

The `reclaim` policy defines what happens to a replica's disk when the replica is removed from the pool.

The `reclaim.onScaleDown` field of a `virtualDiskTemplates` entry controls this behavior. `reclaim` is optional; if omitted, the disk defaults to `Delete`.

- `Delete` (default): the disk belongs to the virtual machine and is removed with it; nothing survives the replica.
- `Retain`: the disk belongs to the pool, outlives the replica and is reattached to the next replica on scale-up. Use it for state that is expensive to rebuild and should survive VM recreation, so scaling back up is warm instead of cold.

`keep` and `ttl` tune the pool of free `Retain` disks (they apply only to `Retain`):

- `keep`: how many recently-freed disks to always keep warm for instant scale-up; these are immune to `ttl`.
- `ttl`: how long a free disk lives beyond the warm buffer before it is garbage-collected.

Examples:

```yaml
# Ephemeral disk: removed with the replica (Delete is the default).
- name: root
  spec:
    persistentVolumeClaim: { size: 30Gi }
    dataSource: { type: ObjectRef, objectRef: { kind: VirtualImage, name: ubuntu } }

# Reusable disk: keep 3 warm for fast scale-up, collect the rest after 1h idle.
- name: cache
  reclaim:
    onScaleDown: Retain
    keep: 3
    ttl: 1h
  spec:
    persistentVolumeClaim: { size: 100Gi }

# Reusable disk kept indefinitely: reused forever, never auto-collected (no ttl).
- name: data
  reclaim:
    onScaleDown: Retain
  spec:
    persistentVolumeClaim: { size: 20Gi }
```

Invalid combinations are rejected on create/update: `keep`/`ttl` may be set only with `Retain`, and `keep > 0` requires a `ttl` (without a `ttl` nothing is ever collected, so `keep` would do nothing). A `Retain` disk with no `ttl` keeps every freed disk indefinitely; bound it with a `ttl` unless that is what you want.

## Notes

Below are pool limitations and non-obvious behavior to keep in mind in production.

- Removing a `virtualDiskTemplates` entry deletes its disks. For `Retain` disks this destroys reusable data, so remove a template only when you no longer need it.
- The pool maintains the replica count, not health. An existing but unhealthy VM is not replaced (VM-level restart handles liveness), and a `Stopped` replica is kept, not replaced; only a fully deleted replica is recreated.
- `Retain` disks are shared across replicas. On scale-up a new replica may reuse another replica's freed disk together with its data; there is no fixed binding between a replica and a disk.
- Editing a `virtualDiskTemplates[].spec` affects only new disks, except `size`, which grows existing disks (never shrinks). `dataSource`, `storageClassName`, etc. are not re-applied to already-created disks.
- Each `virtualDiskTemplates` disk is per-replica: every replica gets its own copy. Shared read-only images (`VirtualImage`/`ClusterVirtualImage`, e.g. a common ISO/CD-ROM) can be attached to all replicas by listing them in the template's `blockDeviceRefs`; a writable disk cannot be shared between replicas.
- Editing the template's `blockDeviceRefs` (reordering, adding or removing a shared image) applies to new replicas; live replicas keep their current devices until they are recreated (rotation or scale-up), like other restart-requiring template changes.
- Template changes that require a restart take effect only after the replica restarts according to `.spec.disruptions.restartApprovalMode` in the template.
