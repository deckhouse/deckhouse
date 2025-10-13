---
title: "About the platform"
permalink: en/virtualization-platform/documentation/about.html
---

Deckhouse Virtualization Platform (DVP) enables declarative creation, execution, and management of virtual machines and their resources.
DVP is powered by [Deckhouse Kubernetes Platform](/products/kubernetes-platform/). The [d8](/products/kubernetes-platform/documentation/v1/deckhouse-cli/) command line utility is used to manage DKP/DVP resources.

## Usage scenarios

- Running virtual machines with an x86-64-compatible OS.

  ![Launching VM](/../../images/virtualization-platform/cases-vms.png)

- Running virtual machines and containerized applications in the same environment.

  ![Launching VMs and applications](/../../images/virtualization-platform/cases-pods-and-vms.png)

- Creation of DKP clusters.

  ![Creation of DKP clusters](/../../images/virtualization-platform/cases.dkp.png)

> **Warning.** If you intend to use Deckhouse Virtualization Platform in a production environment, we recommend deploying it on physical (bare-metal) servers. Deploying Deckhouse Virtualization Platform on virtual machines is also possible, but in this case you must enable nested virtualization.

## Architecture

The platform includes the following components:

- The core of the platform (CORE) is built on the KubeVirt project and uses QEMU/KVM + libvirtd to run virtual machines.
- Deckhouse Virtualization Container Registry (DVCR) - repository for storing and caching virtual machine images.
- Virtualization API (API) - a controller that implements a user API for creating and managing virtual machine resources.

![Architecture](/../../images/virtualization-platform/arch.png)

The following controllers and operators are deployed in the d8-virtualization namespace once the module is enabled:

| Name                          | Component | Comment                                                                                                                      |
| ----------------------------- | --------- |------------------------------------------------------------------------------------------------------------------------------|
| `cdi-operator-*`              | CORE      | Virtualization core component for disk and image management.                                                                 |
| `cdi-apiserver-*`             | CORE      | Virtualization core component for disk and image management.                                                                 |
| `cdi-deployment-*`            | CORE      | Virtualization core component for disk and image management.                                                                 |
| `dvcr-*`                      | DVCR      | A registry to store images.                                                                                                  |
| `virt-api-*`                  | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virt-controller-*`           | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virt-exportproxy-*`          | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virt-handler-*`              | CORE      | Virtualization core component for disk and image management. Must be present on all cluster nodes where VMs will be started. |
| `virt-operator-*`             | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virtualization-api-*`        | API       | API for creating and managing module resources (images, disks, VMs, etc.)                                                     |
| `virtualization-controller-*` | API       | API for creating and managing module resources (images, disks, VMs, etc.)                                                     |
| `vm-route-forge-*`            | CORE      | Router for configuring routes to VMs. Must be present on all cluster nodes where VMs will be started.                        |

The virtual machine runs inside the pod, which allows you to manage virtual machines as regular Kubernetes resources and utilize all the platform features, including load balancers, network policies, automation tools, etc.

![Launching VM](/../../images/virtualization-platform/vm.png)

The API provides the ability to declaratively create, modify, and delete the following underlying resources:

- virtual machine images and boot images
- virtual machine disks
- virtual machines.
