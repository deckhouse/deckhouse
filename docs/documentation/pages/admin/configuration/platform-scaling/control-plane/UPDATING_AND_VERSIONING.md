---
title: "Updating Kubernetes and versioning"
permalink: en/admin/configuration/platform-scaling/control-plane/updating-and-versioning.html
---

## Updating and version management

The control plane update process in DKP is fully automated.

- DKP supports the latest five Kubernetes versions.
- You can roll back the control plane one minor version and upgrade forward several minor versions — one at a time.
- Patch versions (e.g., `1.27.3` → `1.27.5`) are updated automatically with Deckhouse and cannot be managed manually.
- Minor versions are set manually using the [`kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter in the ClusterConfiguration resource.

### Changing the Kubernetes version

1. Open the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) editor:

   ```shell
   d8 system edit cluster-configuration
   ```

1. Set the target Kubernetes version using the `kubernetesVersion` field:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   cloud:
     prefix: demo-stand
     provider: Yandex
   clusterDomain: cloud.education
   clusterType: Cloud
   defaultCRI: Containerd
   kubernetesVersion: "1.30"
   podSubnetCIDR: 10.111.0.0/16
   podSubnetNodeCIDRPrefix: "24"
   serviceSubnetCIDR: 10.222.0.0/16
   ```

1. Save the changes.
1. Wait for the update to complete. You can track the update progress with the `d8 k get no` command. The update can be considered complete when the updated version appears in the `VERSION` column of each cluster node in the command output.

## Monitoring Kubernetes update progress

The [`control-plane-manager`](/modules/control-plane-manager/) module includes the `update-observer` component, which provides up-to-date information about the Kubernetes version update process in the cluster.

`update-observer` component:

- reads cluster configuration from the `d8-cluster-configuration` Secret
- tracks kubelet versions on all nodes via `nodeInfo.kubeletVersion`
- collects versions from all control plane instances via the `control-plane-manager.deckhouse.io/kubernetes-version` annotation
- creates and maintains the **`d8-cluster-kubernetes`** ConfigMap in the `kube-system` namespace with detailed update status.

The `d8-cluster-kubernetes` ConfigMap displays:

- **Component status**: Versions of control plane components (kube-apiserver, kube-scheduler, kube-controller-manager) on each master node.
- **Node progress**: How many nodes have been updated and the total count.
- **Target and current version**: The desired version from configuration and the actual state during the update.
- **Version mismatch**: If any components are running a version different from the target (including newer than desired).
- **Version lists**: `supportedVersions` lists Kubernetes minor versions supported in the current Deckhouse release; `availableVersions` lists versions that can be selected for upgrade or downgrade in *this* cluster (the set is limited by the highest minor version ever installed on the cluster and by the rule that downgrade proceeds one minor at a time); `automaticVersion` is the minor version that will be used when the update mode is Automatic.

During `ControlPlaneUpdating`, `status.progress` reflects overall upgrade progress across intermediate minor versions. For a multi-hop upgrade (for example, 1.33 → 1.35), the percentage increases as each hop completes, not only when every control plane component reaches the final target.

Minor versions in the ConfigMap (`spec`, `status`, and metadata labels such as `k8s-version` and `max-k8s-version`) use the same string format as in ClusterConfiguration—without a `v` prefix (e.g. `"1.33"`).

You can see in real time which components are being updated, at what stage the process is, and whether the update has "stuck" on any node or component.

To view the update status, run the command:

```shell
d8 k get configmap d8-cluster-kubernetes -n kube-system -o yaml
```

### ConfigMap content examples

The `data.spec` and `data.status` fields store YAML with the `spec` field (target version and update mode) and the `status` field (current state). Below are examples of the content for various situations.

#### Cluster up to date (3 master nodes, 3 worker nodes)

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.32"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: UpToDate
    controlPlane:
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    - name: mazin-master-2
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    - name: mazin-master-0
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
  annotations:
    cause: idle
    lastReconciliationTime: "2026-02-02T01:13:05Z"
    lastUpToDateTime: "2026-01-30T16:26:36Z"
  creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: "1.32"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "20837731"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Start of update (e.g., Kubernetes version downgrade)

The target version is already set; the control plane or nodes are still transitioning to it.

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.33"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 0%
    controlPlane:
    - name: mazin-master-0
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-1
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-2
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    nodes:
      desiredCount: 6
      upToDateCount: 0
kind: ConfigMap
metadata:
  annotations:
    cause: downgradeK8s
    lastReconciliationTime: "2026-02-02T11:34:42Z"
    lastUpToDateTime: "2026-02-02T11:09:59Z"
  creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21379847"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Control plane update in progress

Some master nodes are already on the new version, others are still updating; progress is shown as a percentage:

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.33"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 60%
    controlPlane:
    - name: mazin-master-0
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-1
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-2
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
  annotations:
    cause: downgradeK8s
    lastReconciliationTime: "2026-02-02T11:41:55Z"
    lastUpToDateTime: "2026-02-02T11:09:59Z"
  creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21388343"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Intermediate step of a multi-hop upgrade (e.g. 1.33 → 1.35)

The target version in configuration may be several minors ahead of the cluster’s current minor. `status.currentVersion` follows the active minor step, while individual components can temporarily run different minors during a hop. `progress` reflects how much of the overall path (including intermediate minors) is done—so it can be well above 0% before the final target is reached.

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.35"
    updateMode: Manual
  status: |
    currentVersion: "1.34"
    supportedVersions:
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    - "1.35"
    availableVersions:
    - "1.33"
    - "1.34"
    - "1.35"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 60%
    controlPlane:
    - name: cluster-master-0
      phase: Updating
      components:
        kube-apiserver: "1.35"
        kube-controller-manager: "1.34"
        kube-scheduler: "1.34"
    nodes:
      desiredCount: 6
      upToDateCount: 0
kind: ConfigMap
metadata:
  annotations:
    cause: upgradeK8s
  labels:
    heritage: deckhouse
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
```

#### Cluster up to date (2 master nodes, 1 arbitr node and 3 worker nodes)

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.33"
    updateMode: Manual
  status: |
    currentVersion: "1.33"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: UpToDate
    controlPlane:
    - name: mazin-master-0
      phase: UpToDate
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
  annotations:
    cause: upgradeK8s
    lastReconciliationTime: "2026-02-02T11:09:59Z"
    lastUpToDateTime: "2026-02-02T11:09:59Z"
  creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21357074"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Failure of one or more control plane components

The master node has `phase: Failed`; the `description` field contains the reason (e.g., pod or container not in `Running` state):

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.32"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 73%
    controlPlane:
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    - name: mazin-master-2
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-0
      phase: Failed
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
```
