---
title: Updating
permalink: en/architecture/updating.html
---

## Releases

A release is any published version of Deckhouse Kubernetes Platform (DKP).
Each release is distributed through release channels with defined delays.
DKP publishes two types of releases:

- **Patch version** (for example, from `0.0.1` to `0.0.2`): Includes bug fixes and is published when needed.
- **Minor version** (for example, from `0.0.1` to `0.1.0`): Includes new features and is published every 3–4 weeks.

## Release channels

{% alert level="info" %}
Up-to-date information about DKP versions available on different release channels is available at [releases.deckhouse.io](https://releases.deckhouse.io).
{% endalert %}

DKP uses **five release channels** to gradually roll out new versions.
Each new DKP version is first published to the **Alpha** channel and then gradually moves to **Rock Solid**.
Updates in less stable channels are made available to a limited number of users,
which allows to detect and resolve potential issues before they affect production environments.

- **Rock Solid**: The most stable release channel.
  Ideal for clusters requiring maximum reliability.
  Updates are published at least 30 days after the release.
- **Stable**: A stable channel suitable for production clusters.
  Updates are published at least 14 days after the release.
- **Early Access**: The recommended release channel if you’re unsure which one to choose.
  Suitable for actively evolving clusters (for example, those launching or refining new applications).
  Updates are published at least 7 days after the release.
- **Beta**: Intended for development clusters, similar to the Alpha channel.
  Receives versions that have already been tested in Alpha.
- **Alpha**: The least stable channel with the most frequent updates.
  Targeted at development clusters with a small number of developers.
  Versions appear immediately after release.

### Switching to a more stable channel

When switching to a more stable channel (for example, from `Alpha` to `EarlyAccess`):

1. DKP fetches release data from the `EarlyAccess` channel.
1. It compares this data with existing DeckhouseRelease custom resources in the cluster.
   - If the cluster contains newer releases with `Pending` status (not yet applied),
     they will be **removed**, since they haven’t been published to the new channel.
   - If newer releases have already been marked as `Deployed`(installed successfully),
     the switch won’t take effect immediately.
     DKP will remain on the current release until a newer version becomes available in the `EarlyAccess` channel.

### Switching to a less stable channel

When switching to a less stable channel (for example, from `EarlyAccess` to `Alpha`):

1. DKP fetches release data from the `Alpha` channel.
1. It compares this data with existing DeckhouseRelease custom resources.
1. It applies the update according to the [configured update parameters](../admin/configuration/update/configuration.html).

## Control plane updates

In DKP, the control plane update process is highly automated and safe for both single-master and multi-master clusters.
While brief interruptions in API server availability may occur,
they do not affect the operation of applications running in the cluster.
In most cases, no additional maintenance window is required.

DKP supports the latest five minor Kubernetes versions.
A full list of supported versions is available in the [corresponding table](../supported_versions.html#kubernetes).

### Patch version updates

Patch updates to control plane components
(within the same minor version, for example, from `1.27.3` to `1.27.5`)
are applied automatically together with DKP updates.
Users cannot manage patch updates manually.
The process is fully automated by the platform.

### Automatic minor version updates

To automatically update the control plane to a new minor version (for example, from `1.28.*` to `1.30.*`),
specify [`kubernetesVersion: Automatic`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) in the ClusterConfiguration resource.
DKP will select the default Kubernetes version at the time of the update.

### Manual minor version updates

To manually update the control plane to a new minor version (for example, from `1.28.*` to `1.30.*`),
specify the target version in the [`kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter of the ClusterConfiguration resource.
For example, `kubernetesVersion: 1.30`.

```shell
d8 platform edit cluster-configuration
```

This command initiates an upgrade to the default minor Kubernetes version used by DKP at the time.
To track the upgrade progress, check the Kubernetes version in the output of the node description command:

```shell
d8 k get nodes
```

### Process of control plane component updates

1. The [control-plane-manager](/modules/control-plane-manager/) module modifies the manifests of core components
   (`apiserver`, `controller-manager`, `scheduler`, etc.).
1. The updated components are deployed on all master nodes.

When performing a minor version upgrade:

- If bumping for more than one minor version (for example, from `1.28` to `1.30`),
  the update happens in stages: `1.28` → `1.29` → `1.30`.
- At each step, the control plane is updated first, followed by the kubelet upgrade on the nodes.

When performing a downgrade:

- Only a single minor version downgrade is supported from the highest version ever used in the cluster.
- The downgrade occurs in reverse order.
  Kubelet components on the nodes are downgraded first, followed by the control plane components.

After the control plane has been updated, node updates begin:

1. The Bashible API Server updates the kubelet version in the scripts for all NodeGroups.
1. Nodes in different NodeGroups are updated in parallel,
   but within each NodeGroup, they are updated sequentially (one by one).
1. Each NodeGroup's update settings are taken into account:
   - Manual or automatic update mode.
   - Update windows.
   - Whether Pods should be evicted before updating, etc.
1. One or more candidate nodes that meet all update conditions are selected and updated.
1. The process repeats until all nodes in all groups are updated.
1. When all nodes are updated, the update process is complete.

## Retrieving the changelog

Each new version of DKP includes a *changelog*, which is a detailed list of changes,
including new features, bug fixes, component updates, and important compatibility notes.

You can find the changelog for a specific DKP version in the [Deckhouse release list on GitHub](https://github.com/deckhouse/deckhouse/releases).

A summary of key changes, component version updates, and which cluster components will be restarted
is included in the description of the zero patch release: [example for DKP v1.68](https://github.com/deckhouse/deckhouse/releases/tag/v1.68.0).

### Changelog contents

The changelog includes four sections:

- **Know before update**: Critical information to consider before updating.
  Includes compatibility requirements, pre-update actions, and possible cluster impacts.
- **Features**: Highlights of key improvements and new features introduced in the release.
- **Fixes**: Minor changes, security updates, and performance improvements.
- **Chore**: Technical updates, such as dependency updates, refactoring, build and test pipeline changes,
  and vulnerability fixes.

### Minor versions and zero patch releases

All major changes are listed in the **zero patch release** (for example, `v1.68.0` for the DKP `v1.68`).
Before updating to a new minor version (for example, `v1.68`):

1. Review the changelog for the corresponding version.
1. Check if the changes affect your infrastructure.
1. Adjust the cluster configuration if needed.

## Checking dependencies before update

Before applying a new release, DKP checks the cluster for potential issues.
If any of the following incompatibilities are detected, the update is aborted:

- Unsupported Kubernetes version.
- When `kubernetesVersion: Automatic` is enabled, the update is aborted if:
  - The release introduces a new default Kubernetes version;
  - Monitoring is enabled;
  - The cluster contains resources that are [deprecated](https://kubernetes.io/docs/reference/using-api/deprecation-guide/) in the new version.
- The installed Ingress controller version is incompatible with the new release.
- Nodes are running outdated or unsupported operating systems.
- The cluster has an enabled module that is `deprecated` or has been removed in the new release.
