# Patches

#### `001-bundle-extra-images.patch`

Internal patch which adds `libguestfs`, `virt-exportserver` and `virt-exportproxy`
to images bundle target.

#### `003-macvtap-binding.patch`

This PR adds macvtap networking mode for binding podNetwork.

- https://github.com/kubevirt/community/pull/186
- https://github.com/kubevirt/kubevirt/pull/7648

#### `004-allow-live-migration.patch`

Allow live-migration for pod network in bridge mode

- https://github.com/kubevirt/kubevirt/issues/7238

#### `005-fix-cgroup-v2-device-rules-state.patch`

When a block volume is non-hotpluggable (i.e. it is specified explicitly in the VMI spec), the device cgroup permissions are managed purely by Kubernetes and CRI. For v2, that means a BPF program is assigned to the POD's cgroup. However, when we manage hotplug volumes, we overwrite the BPF program to allow access to the new block device. The problem is that we do not know what the existing BPF program does, hence we just follow some assumptions about the 'default' devices that we need to allow (e.g. /dev/kvm and some others). We need to also consider the non-hotpluggable volumes, otherwise a VM with a block PVC or DV will fail to start if a hotplug volume is attached to it.

- https://github.com/kubevirt/kubevirt/pull/8865

#### `006-fix-cgroup-v2-device-rules-state-macvtap.patch`

Same as above but also enables live-migration for macvtap interfaces

#### `007-tolerations-for-strategy-dumper-job.patch`

There is a problem when all nodes in cluster have taints, KubeVirt can't run virt-operator-strategy-dumper job.
The provided fix will always run the job in same place where virt-operator runs

- https://github.com/kubevirt/kubevirt/pull/9360

#### `008-fix-admissionreview.patch`

Fixes admission review for creating pods

- https://github.com/kubevirt/kubevirt/pull/9579
