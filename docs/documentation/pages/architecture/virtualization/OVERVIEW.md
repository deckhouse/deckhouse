---
title: Virtualization subsystem
permalink: en/architecture/virtualization/
search: virtualization, virtualization subsystem, dvp
description: Architecture of the Virtualization subsystem in Deckhouse Kubernetes Platform
---

This subsection describes the architecture of the Virtualization subsystem of Deckhouse Kubernetes Platform (DKP).

The Virtualization subsystem is represented by the [`virtualization`](/modules/virtualization/) module, which allows you to declaratively create, run, and manage virtual machines (VM) and their resources.

The [`virtualization`](/modules/virtualization/) module consists of the following components:

- Virtualization API (API): A controller that implements a user API for creating and managing VM resources.
- A module core (CORE): It is based on the KubeVirt project and uses QEMU/KVM + libvirtd to run VMs. [KubeVirt](https://github.com/kubevirt/kubevirt) is an open-source project that allows you to launch, deploy, and manage VMs using Kubernetes as an orchestration platform. It enables a cooperation between traditional VMs and container workloads in the same Kubernetes cluster, providing a single control plane.
- [Deckhouse Virtualization Container Registry (DVCR)](dvcr.html): A container registry for storing and caching VM images.
- [Containerized Data Importer (CDI)](cdi.html): It is an add-on for managing persistent storage in Kubernetes. Its main goal is to provide a declarative way to create VM disks based on PersistentVolumeClaim (PVC) resources. CDI provides the ability to import VM images and disks into PVC volumes for use in KubeVirt-managed VMs.
- Auxiliary components: Components that implement the following auxiliary functions:

  - Security events audit.
  - Forwarding USB devices to VMs.
  - Updating network routes.
  - Deleting resources before deactivating the [`virtualization`](/modules/virtualization/) module.

For more details about the module, refer to the [module documentation section](/modules/virtualization/).
