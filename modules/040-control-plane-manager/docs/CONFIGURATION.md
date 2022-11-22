---
title: "Managing control plane: configuration"
---

The `control-plane-manager` module is responsible for managing the cluster's control plane components. The cluster parameters that impact control plane management are derived from the initial cluster configuration (`cluster-configuration.yaml` parameter from the `d8-cluster-configuration` secret in the `kube-system` namespace), which is created during the installation.

{% include module-bundle.liquid %}

## Parameters

<!-- SCHEMA -->
