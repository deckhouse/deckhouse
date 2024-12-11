---
title: "Cloud provider layout"
description: "Common information about the cloud provider layout."
---

A cloud provider's layout determines how resources are located in the cloud. The layout is depended on the cloud provider's architecture and the way it organizes resources.

Every cloud provider layout has a unique structure. However, there are some common concepts that are shared across cloud providers. These concepts include:

### Resources

Every layout has two kinds of resources:

* Resources created by the Deckhouse platform:
  * Statically allocated resources(bootstrap or converge phases):
    * Terraform module

  * Dynamically allocated resources:
    * Cloud Controller Manager
    * Machine Controller Manager
    * Cluster API provider

* Pre-existing resources: Resources that are already present in the cloud provider's environment.

### Incoming traffic for applications

Incoming traffic for applications can be routed through:

* Globally routable IP address(Public IP, External IP, etc.) assigned by the cloud provider
* Load balancer from `cloud-controller-manager`
* MetalLB from `metallb` or `l2-load-balancer` Deckhouse modules
* DNAT rules that are manually configured
