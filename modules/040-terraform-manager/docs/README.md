---
title: "The terraform-manager module"
description: Description of the Deckhouse terraform-manager module. Ensures that the objects in the cluster correspond to the Terraform state.
---

The module provide tools for working with Terraform in the Kubernetes cluster.

* The module consists of 2 parts:
  * `terraform-auto-converger` — checks the Terraform state and applies non-destructive changes;
  * `terraform-state-exporter` — checks the Terraform state and exports cluster metrics.

* The module is enabled by default if the following secrets are present in the cluster:
  * `kube-system/d8-provider-cluster-configuration`;
  * `d8-system/d8-cluster-terraform-state`.
