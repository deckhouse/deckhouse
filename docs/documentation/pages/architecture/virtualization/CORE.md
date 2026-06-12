---
title: The module core
permalink: en/architecture/virtualization/core.html
search: virt-controller, virt-api, virt-handler, virt-launcher, subresources, kubevirt, virt-operator, core
description: Architecture of the virtualization module core in Deckhouse Kubernetes Platform.
---

The [`Virtualization`](/modules/virtualization/) module core is directly responsible for working with virtual machines (VMs). The core is based on the KubeVirt project. [KubeVirt](https://github.com/kubevirt/kubevirt) is an open-source project that allows you to launch, deploy, and manage VMs using Kubernetes as an orchestration platform. It enables a cooperation between traditional VMs and container workloads in the same Kubernetes cluster, providing a single control plane. A [fork](https://github.com/deckhouse/3p-kubevirt) of KubeVirt from the company "Flant" is used in the [`virtualization`](/modules/virtualization/) module.

To manage VMs, the module core uses custom resources of the following API groups:

1. `Internal.virtualization.deckhouse.io`: The main group, an analog of the `kubevirt.io` API group of the original KubeVirt. It includes the following custom resources:

   - InternalVirtualizationVirtualMachine: A resource that describes the VM configuration and status.
   - InternalVirtualizationVirtualMachineInstance: A resource that describes a running VM.
   After VM shutdown, the InternalVirtualizationVirtualMachineInstance resource is deleted, but the InternalVirtualizationVirtualMachine resource that manages the InternalVirtualizationVirtualMachineInstance lifecycle remains.

   The resources of the main group are managed by the virt-controller component. The InternalVirtualizationVirtualMachine resources of the KubeVirt `internal.virtualization.deckhouse.io` main API group are used as the backend for the VirtualMachine resources of `virtualization.deckhouse.io` API group managed by the virtualization-controller.

   {% alert level="info" %}
   For simplification, the reduced VirtualMachine and VirtualMachineInstance names will be used for the InternalVirtualizationVirtualMachine and InternalVirtualizationVirtualMachineInstance resources (of the original KubeVirt `kubevirt.io` API group) respectively.
   {% endalert %}

2. `subresources.kubevirt.io`: Subresources group. Subresources are additional operations or actions that can be performed on core resources (for example, VirtualMachineInstance) via the Kubernetes API. They provide interfaces for managing specific aspects of resources without affecting the entire object. Instead of the declarative resource familiar to Kubernetes, they are an endpoint for imperative operations. The following KubeVirt subresources are used in the [`virtualization`](/modules/virtualization/) module:

   - `virtualmachines/{name}/addvolume`;
   - `virtualmachines/{name}/removevolume`;
   - `virtualmachines/{name}/addresourceclaim`;
   - `virtualmachines/{name}/removeresourceclaim`;
   - `virtualmachineinstances/{name}/console`;
   - `virtualmachineinstances/{name}/vnc`;
   - `virtualmachineinstances/{name}/portforward`;
   - `virtualmachineinstances/{name}/freeze`;
   - `virtualmachineinstances/{name}/unfreeze`.

   The subresources are managed by the virt-api component. The KubeVirt subresources listed above are used as a backend for similar resources of `subresources.virtualization.deckhouse.io` API group managed by the virtualization-api component.

## Module core architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

- The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
- Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`virtualization`](/modules/virtualization/) module core and its interactions with other components of DKP are shown in the following diagrams:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Architecture of the virtualization module core](../../images/architecture/virtualization/c4-l2-virtualization-core.png)

## Module core components

The module core consists of the following components:

1. **Virt-api**: A [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/) that serves requests to the `subresources.kubevirt.io` API groups.
   The virt-api component performs validation and mutation of custom resources of the `internal.virtualization.deckhouse.io` API groups using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.
   Requests pass through the **proxy** sidecar container that renames metadata from the `internal.virtualization.deckhouse.io` API group to the `kubevirt.io` API group and proxies them to the virt-api component endpoint.

   It consists of the following containers:

   - **virt-api**: Main container that implements controller and webhook server.
   - **proxy** (aka **kube-api-rewriter**): Sidecar container that performs modification of API requests passing through it, namely renaming the metadata of custom resources. This is necessary because KubeVirt components use API groups like `*.kubevirt.io`, and other components of the [`virtualization`](/modules/virtualization/) module use similar resources, but with API groups like `*.virtualization.deckhouse.io`. Kube-api-rewriter is a gateway that proxies requests between controllers that manage resources from different API groups.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to the metrics of the proxy container. It is an [open-source project](https://github.com/brancz/kube-rbac-proxy).

1. **Virt-controller**: A controller that manages the `internal.virtualization.deckhouse.io` main API group custom resources and responsible for the virtualization functionality at the cluster wide level. For each VirtualMachineInstance resource it creates a separate pod in which the VM is started. Virt-controller monitors VirtualMachineInstance resources, updates their status, and manages the associated pods.

   It consists of the following containers:

   - **virt-controller**: Main container.
   - **proxy** (aka **kube-api-rewriter**): A sidecar container that performs modification of API requests passing through it (described above).
   - **kube-rbac-proxy**: A sidecar container providing authorized access to the metrics of the cdi-apiserver and proxy containers (described above).

1. **Virt-handler** (DaemonSet): A separate controller that runs on all nodes of the cluster. Virt-handler performs the following functions:

   - It extends the [kubelet](../kubernetes-and-scheduling/kubelet.html) functionality, configuring the pod environment to run a VM inside it. At the moment virt-handler is responsible for creating network interfaces, also it is used to forward `dev/kvm` and other devices from the node into the pod. Virt-handler uses [kubelet device plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/) to forward devices.

   - Like virt-controller, virt-handler watches VirtualMachineInstance resources corresponding to VMs running on the node. When changes are detected, virt-handler sends a command to the virt-launcher process running in the compute container of the VM pod. Virt-launcher changes the VM state in accordance with the command received. Virt-handler also monitors VM events that virt-launcher returns and synchronizes the corresponding VirtualMachineInstance resource status.

   - It accepts commands from the virt-api component via the `console-port` that correspond to requests for subresources and forwards them to the virt-launcher for execution. By the subresources functionality ports are forwarded to the VM, as well as to the regular and VNC consoles.

     Virt-handler interacts with virt-launcher via the gRPC protocol and Unix socket.

   It consists of the following containers:

   - **virt-launcher**: Init container that runs `node-labeller.sh` script via virt-launcher binary. This script prepares CPU parameters, their functions, and machine types which virt-handler will use to set the appropriate labels on Node resources. These labels, in turn, will be used to schedule VMs to the nodes that support the appropriate parameters.
   - **virt-handler**: Main container.
   - **virt-launcher-image-holder**: A service sidecar container for virt-launcher image pre-loading. The container is on pause and performs only the function of storing the image.
   - **pr-helper**: [QEMU persistent reservation helper](https://www.qemu.org/docs/master/tools/qemu-pr-helper.html) is a service sidecar container that creates a listener socket that accepts incoming connections to communicate with QEMU. This is necessary because the operating system restricts sending SCSI commands with permanent redundancy to unprivileged programs, which prevents multiple VMs from sharing block SCSI devices, for example in the case of clusterization. [QEMU](https://www.qemu.org/) is a free and open-source program for emulating the hardware of various platforms, which is used to run VMs in a pod.

1. **Virt-operator**: A Kubernetes operator that manages the KubeVirt components lifecycle using InternalVirtualizationKubeVirt custom resource. Virt-operator installs the virt-api, virt-controller and virt-handler in the cluster and also configures them.

   It consists of the following containers:

   - **virt-operator**: Main container.
   - **proxy** (aka **kube-api-rewriter**): Sidecar container that performs modification of API requests passing through it (described above).

1. **Virt-launcher-[VMI name]**: The pod where the VM is running (more exactly VirtualMachineInstance).

   It consists of one container:

   - **compute**: A container where the virt-launcher is run. Virt-launcher implements cmd-server (a gRPC server for the remote command execution).

     Depending on the command received from virt-handler, virt-launcher generates an XML specification of the VM being launched or updated and sends it to libvirtd. Libvirtd is a daemon of the server part of the libvirt virtualization management system. It runs on host servers and performs management tasks for virtual guest systems.

     Libvirtd, in turn, starts the VM and manages its lifecycle. The VM is started using QEMU and KVM. [QEMU](https://www.qemu.org/) is an open-source emulator that supports hardware virtualization and works in conjunction with the [KVM](https://linux-kvm.org/page/Main_Page) hypervisor.

     In fact libvirtd starts the QEMU process, which is the VM (more exactly VirtualMachineInstance).

     Virt-handler also constantly monitors the status of the running VM, returned by libvirtd via virt-launcher, and updates the VirtualMachineInstance status.

## Module core interactions

The module core (CORE) interacts with the following components:

1. **Kube-apiserver**:

   - Watches for KubeVirt custom resources and manages KubeVirt components.
   - Watches for VirtualMachineInstance custom resources, updates their status, and manages the associated pods.
   - Authorizes requests for metrics.

1. [**CDI (Containerized-Data-Importer)**](cdi.html): KubeVirt creates a DataVolume resource based on the disk specification and a link to the VM image in the `DataVolumeTemplate` section of the VirtualMachine resource. CDI imports a disk image to PVC from the source specified in the DataVolume. The created PVC is a disk of a virtual machine managed by KubeVirt.

The following external components interact with the module core:

1. **Kube-apiserver**:

   - Sends InternalVirtualizationKubeVirt custom resource validating requests.
   - Sends `internal.virtualization.deckhouse.io` API group resource validating and mutating requests.

1. **Prometheus-main**: Collects module core components metrics.
