---
title: "Updating Kubernetes and versioning"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/updating-and-versioning.html
---

## Updating and version management

The control plane update process in DVP is fully automated.

- DVP supports the latest five Kubernetes versions.
- You can roll back the control plane one minor version and upgrade forward several minor versions — one at a time.
- Patch versions (e.g., `1.27.3` → `1.27.5`) are updated automatically with Deckhouse and cannot be managed manually.
- Minor versions are set manually using the `kubernetesVersion` parameter in the ClusterConfiguration resource.

### Changing the Kubernetes version

1. Open the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) editor:

   ```shell
   d8 platform edit cluster-configuration
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
