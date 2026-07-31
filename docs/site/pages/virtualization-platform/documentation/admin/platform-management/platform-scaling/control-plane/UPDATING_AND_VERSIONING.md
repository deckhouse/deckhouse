---
title: "Updating Kubernetes and versioning"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/updating-and-versioning.html
---

## Updating and version management

The control plane update process in DVP is fully automated.

- DVP supports the latest five Kubernetes versions.
- You can roll back the control plane one minor version and upgrade forward several minor versions — one at a time.
- Patch versions (e.g., `1.27.3` → `1.27.5`) are updated automatically with Deckhouse and cannot be managed manually.
- Minor versions are set using the [`kubernetesVersion`](/modules/control-plane-manager/configuration.html#parameters-kubernetesversion) parameter of the [`control-plane-manager`](/modules/control-plane-manager/) ModuleConfig.

### Changing the Kubernetes version

1. Edit the ModuleConfig of the `control-plane-manager` module:

   ```shell
   d8 k edit mc control-plane-manager
   ```

1. Set the target Kubernetes version in `spec.settings.kubernetesVersion`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 3
     enabled: true
     settings:
       kubernetesVersion: "1.30"
   ```

   Use `kubernetesVersion: "Automatic"` to track the Kubernetes version considered stable for the current Deckhouse release. If the parameter is omitted, Deckhouse falls back to the deprecated `ClusterConfiguration.kubernetesVersion` field (if present), otherwise to the release default.

1. Save the changes.

{% alert level="warning" %}
Do not set `kubernetesVersion` in [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — the field is deprecated. Keeping an explicit pin there without migrating it to ModuleConfig `control-plane-manager` triggers the [D8ObsoleteKubernetesVersionInClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/alerts.html#control-plane-manager-d8obsoletekubernetesversioninclusterconfiguration) alert.
{% endalert %}
