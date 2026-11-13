---
title: "Updating Kubernetes and versioning"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/updating-and-versioning.html
---

## Updating and version management

The control plane update process in DVP is fully automated.

- DVP supports the latest five Kubernetes versions.
- You can roll back the control plane one minor version and upgrade forward several minor versions — one at a time.
- Patch versions (e.g., `1.35.3` → `1.35.5`) are updated automatically with Deckhouse and cannot be managed manually.
- Minor versions are set manually using the [`kubernetesVersion`](/modules/control-plane-manager/configuration.html#parameters-kubernetesversion) parameter in the `control-plane-manager` ModuleConfig.

### Changing the Kubernetes version

1. Open the `control-plane-manager` ModuleConfig editor:

   ```shell
   d8 k edit mc control-plane-manager
   ```

1. Set the target Kubernetes version using the `settings.kubernetesVersion` parameter:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 3
     enabled: true
     settings:
       kubernetesVersion: "1.35"
   ```

   {% alert level="info" %}
   The kubernetesVersion field in [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration), previously used to select the Kubernetes version, has been deprecated. Until the `settings.kubernetesVersion` parameter of the `control-plane-manager` module is set, this field determines the cluster's Kubernetes version. Once the parameter is set, its value is used instead.
   {% endalert %}

1. Save the changes.
