---
title: "The terraform-manager module"
---
## Description

The module provide tools for working with Terraform in the Kubernetes cluster.

* The module does not have any parameters to configure.
* The module is enabled by default if the following secrets are present in the cluster:
    * kube-system/d8-provider-cluster-configuration
    * d8-system/d8-cluster-terraform-state

  To disable the module, insert the following line into the Deckhouse configuration file:
  ```yaml
  terraformManager: "false"
  ```
