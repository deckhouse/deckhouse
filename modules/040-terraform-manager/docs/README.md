---
title: "The terraform-manager module"
---
## Description

The module provide tools for working with Terraform in the Kubernetes cluster.

* The module consists of 2 parts:
  * `terraform-auto-converger` - checks the terraform state and applies non-destructive changes
  * `terraform-state-exporter` - checks the terraform state and exports cluster metrics

* The module has the following settings:

  * `autoConvergerEnabled: true / false` - disables auto-applying of the terraform state. 
    * By default: `true` - enabled
  * `autoConvergerPeriod: interval (for example: 5s, 10m5s 1h30m30s)` - after what period of time check the terraform state and apply it.
    * By default: `1h` - 1 hour
    
* The module is enabled by default if the following secrets are present in the cluster:
    * kube-system/d8-provider-cluster-configuration
    * d8-system/d8-cluster-terraform-state

  To disable the module, insert the following line into the Deckhouse configuration file:
  ```yaml
  terraformManagerEnabled: "false"
  ```
