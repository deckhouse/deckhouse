---
title: "Virtualization"
menuTitle: "Virtualization"
moduleStatus: General Availability
weight: 10
---

## Description

The `virtualization` module allows you to declaratively create, start, and manage virtual machines and associated resources.

The command line utility [`d8`](https://deckhouse.ru/documentation/v1/deckhouse-cli/) is used to manage cluster resources.

## Usage scenarios

- Running virtual machines with an x86-64-compatible OS.

  ![](./images/cases-vms.png)

- Running virtual machines and containerized applications in the same environment.

  ![](./images/cases-pods-and-vms.png)

- Creation of DKP clusters.

  ![](./images/cases.dkp.png)

{% alert level="info" %}
If you plan to use `virtualization` in a production environment, it is recommended to use a cluster deployed on physical (bare-metal) servers. For testing purposes, it is allowed to use the module in a cluster deployed on virtual machines but with nested virtualization enabled on them.
{% endalert %}

## Architecture

The module includes the following components:

- A module core (CORE) that is based on the KubeVirt project and uses QEMU/KVM + libvirtd to run virtual machines.
- Deckhouse Virtualization Container Registry (DVCR), a repository for storing and caching virtual machine images.
- Virtualization API (API), a controller that implements a user API for creating and managing virtual machine resources.

![](images/arch.png)

List of controllers and operators deployed in the `d8-virtualization` namespace after the module is enabled:

| Name                          | Component | Comment                                                                                                                      |
| ----------------------------- | --------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `cdi-operator-*`              | CORE      | Virtualization core component for disk and image management.                                                                 |
| `cdi-apiserver-*`             | CORE      | Virtualization core component for disk and image management.                                                                 |
| `cdi-deployment-*`            | CORE      | Virtualization core component for disk and image management.                                                                 |
| `dvcr-*`                      | DVCR      | A registry to store images.                                                                                                  |
| `virt-api-*`                  | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virt-controller-*`           | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virt-exportproxy-*`          | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virt-handler-*`              | CORE      | Virtualization core component for disk and image management. Must be present on all cluster nodes where VMs will be started. |
| `virt-operator-*`             | CORE      | Virtualization core component for disk and image management.                                                                 |
| `virtualization-api-*`        | API       | API for creating and managing module resources (images, disks, VMs, etc.).                                                   |
| `virtualization-controller-*` | API       | API for creating and managing module resources (images, disks, VMs, etc.).                                                   |
| `virtualization-audit-*`      | Security  | Audit logs for virtualization module reources.                                                                               |
| `vm-route-forge-*`            | CORE      | Router for configuring routes to VMs. Must be present on all cluster nodes where VMs will be started.                        |

The virtual machine runs inside the pod, which allows you to manage virtual machines as regular Kubernetes resources and utilize all the platform features, including load balancers, network policies, automation tools, etc.

![](images/vm.png)

The API provides the ability to declaratively create, modify, and delete the following underlying resources:

- Virtual machine images and boot images.
- Virtual machine disks.
- Virtual machines.
