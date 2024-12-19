## Patches

## 002-network-name-discovery-logic.patch

### Kubernetes versions starting from 1.22
This patch allows setting multiple values separated with a comma in InternalNetworkName and ExternalNetworkName parameters in Nodes configuration section.
Both of them we use to explicitly define what networks are external/internal to properly provide IP Addresses in the status of Node objects.

Based on [PR#524](https://github.com/kubernetes/cloud-provider-vsphere/pull/524).

> Consider sending PR with our changes to the upstream.

## 003-folder-path-option.patch

This patch adds vmFolderPath parameter to VirtualCenter configuration section.
This option acts like a filter when CCM searches VM in vSphere.

> Abandoned starting from Kubernetes version 1.22

## 004-dont-initialize-node-without-internal-ip.patch

This patch adds a check, restricting Node registration in the cluster while it has no internal IP.

> Consider implementing a flag in CCM config and sending as a PR to the upstream.
> Currently that patch makes tests fail.

## 005-ignore-static-nodes.patch

This patch is for our case when we want to have a static Nodes in the cluster, managed by vSphere cloud provider.

> Consider implementing a flag in CCM config and sending as a PR to the upstream.
