---
title: "Managing control plane: configuration"
---

Some cluster parameters that affect control plane management historically came from the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) resource. Prefer the corresponding settings in this ModuleConfig instead — for example, [`kubernetesVersion`](configuration.html#parameters-kubernetesversion). The ClusterConfiguration fields remain only as a deprecated fallback during migration.

<!-- SCHEMA -->
