title: "Overview"
permalink: en/virtualization-platform/documentation/
---

The module allows you to declaratively create, run, and manage virtual machines and their resources on the [Deckhouse platform](https://deckhouse.io).

Scenarios of using the module:

- Running virtual machines with x86_64 compatible OS.
- Running virtual machines and containerized applications in the same environment.

![](/images/virtualization-platform/cases-vms.png)

![](/images/virtualization-platform/cases-pods-and-vms.png)

## Requirements

The virtualization module requires a Deckhouse Kubernetes Platform cluster for its operation.

- The processor requirements for the cluster nodes on which the virtual machines are to run include x86_64 architecture and support for Intel-VT or AMD-V instructions.
- Other cluster node requirements are described in the document: [Going to Production](https://deckhouse.io/guides/production.html)
- Any [compatible](https://deckhouse.io/documentation/v1/supported_versions.html#linux) Linux-based operating system is supported on the cluster nodes.
- The Linux kernel on cluster nodes must be version 5.7 or newer.

{% alert level="warning" %}
If you plan to use the virtualization module in a production environment, it is recommended to deploy it on physical servers. Deploying the module on virtual machines is also possible, but in this case you need to enable nested virtualization.
{% endalert %}

The [d8](https://github.com/deckhouse/deckhouse-cli) command line utility is used to connect to virtual machines using serial port, VNC, or ssh protocol. For EE-version users, the ability to manage resources via UI is available.

You can view the resource documentation locally from the console using the standard functionality of the command utility: `d8 kubectl explain <resource name>`

## How to enable the module

To enable the module, you need a Deckhouse Kubernetes Platform cluster deployed according to [requirements](#Requirements). To deploy Deckhouse Kubernetes Platform, follow [instructions](https://deckhouse.io/gs/#other-options).

1. Enable the [CNI Cilium](/documentation/v1/modules/021-cni-cilium/) module to provide network connectivity for cluster resources.
2. To store virtual machine data, you must enable one of the following modules according to their installation instructions:

- [SDS-Replicated-volume](https://deckhouse.io/modules/sds-replicated-volume/stable/)
- [CEPH-CSI](/documentation/v1/modules/031-ceph-csi/)

It is also possible to use other storage options that support block device creation with `RWX` (`ReadWriteMany`) access mode.

4. Enable [module](./CONFIGURATION.md)
5. Install d8 command line utility:

```bash
curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/d8-install.sh | sudo bash -s
```

## Updating the module

The virtualization module uses five update channels designed for use in different environments, to which different requirements apply in terms of reliability:

| Update Channel | Description                                                                                                                                                                                                                                                                                        |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Alpha          | The least stable update channel with the most frequent appearance of new versions. It is focused on development clusters with a small number of developers.                                                                                                                                        |
| Beta           | is focused on development clusters, as is the Alpha update channel. Receives versions previously tested on the Alpha update channel.                                                                                                                                                               |
| Early Access   | Recommended update channel if you are not sure about the choice. It is suitable for clusters in which active work is underway (new applications are being launched, finalized, etc.). Functional updates reach this update channel no earlier than one week after their appearance in the release. |
| Stable         | Stable update channel for clusters in which active work has been completed and operation is mainly carried out. Functional updates reach this update channel no earlier than two weeks after they appear in the release.                                                                           |
| Rock Solid     | The most stable update channel. It is suitable for clusters that need to provide an increased level of stability. Functional updates to this channel do not reach earlier than a month after their appearance in the release.                                                                      |

Module components can be updated automatically, or with manual confirmation as updates are released in the update channels.

Information on the versions available on the update channels can be obtained on this website https://releases.deckhouse.io/

## Architecture

The module includes the following components:

- The module core, based on the KubeVirt project and uses QEMU/KVM + libvirtd to run virtual machines.
- Deckhouse Virtualization Container Registry (DVCR) - repository for storing and caching virtual machine images.
- Virtualization-API - controller that implements a user API for creating and managing virtual machine resources.
- Routing Controller (ROUTER) - A controller that manages routes to provide network connectivity for virtual machines.

The API provides capabilities for creating and managing the following resources:

- Virtual Images
- Virtual Disks
- Virtuam machine Classes
- Virtual machines
- Virtual Machine Operations

## Description of functional features

### Virtual Images

Images are immutable resources that allow you to create new virtual machines based on preconfigured and configured images. Depending on the type, images can be in `raw`, `qcow2`, `vmdk` and other formats for virtual machine disk images, and in `iso` format for installation images that can be attached as `cdrom devices`.

You can use external sources such as `http server`, `container registry`, and locally via the command line (`cli`) to download images. It is also possible to create images from virtual machine disks, for example when you need to create a base image for replication (`golden-image`).

It is important to note that images can be attached to a virtual machine in read-only mode.

Images are of two types: clustered `ClusterVirtualImage`, which are available to all users of the platform, and namespaced `VirtualImage`, which are available only to users within a specific `namespace`.

For `ClusterVirtualImage`, images are stored only in `DVCR`, while for `VirtualImage` you can use both `DVCR` and platform-provided storage (`PVC`).

### Virtual Disks

Creating disks for virtual machines is provided by the `VirtualDisk` resource. Disks are used in the virtual machine as the primary storage medium. Disks can be created from external sources, previously created images (`VirtualImage` or `ClusterVirtualImage`) or can be created `empty`.

One of the key features of disks is the ability to resize them without having to stop the virtual machine. It is important to note that only the ability to increase disk size is supported, while decreasing is not available.

Furthermore, disks can be attached to virtual machines while they are running, providing flexibility in storage management. The `VirtualMachineBlockDeviceAttachment` resource is used for this task.

Platform-provided storage (`PVC`) is used to store disks.

### Virtual Machine Classes

A virtual machine class is designed for:
- configuring the type of virtual machine vCPU
- control the placement of virtual machines on the platform nodes
- configuring virtual machine resources (vCPU, memory) for more optimal planning and placement of virtual machines on the platform nodes.

The virtual machine class is configured using the `VirtualMachineClass` resource.

### Virtual Machines

The `VirtualMachine` resource is responsible for creating and managing the lifecycle of virtual machines. Through the `VirtualMachine` configuration you can define virtual machine parameters such as number of processors, amount of RAM, attached images and disks, as well as placement rules on platform nodes, similar to the way it is done for Pods.

A virtual machine's startup policy defines its state. It can be enabled, disabled, or the state can be managed manually. When a node on which a virtual machine is running is rebooted, it will be temporarily evicted from that node using a "live migration" mechanism to another free node that satisfies the placement rules.

The virtual machine runs inside the Pod, which allows you to manage virtual machines as normal Kubernetes resources and use all the features of the platform, including load balancers, network policies, automation tools, etc.

![](images/vm.png)

### Virtual Machine Operations

The `VirtualMachineOperations` resource is intended for declarative control of virtual machine state changes. The resource allows you to perform the following actions on virtual machines: Start, Stop, Restart.

## Role Model

The following user roles are provided for managing module resources:

- User
- PrivilegedUser
- Editor
- Admin
- ClusterEditor
- ClusterAdmin.

The following table shows the access matrix for these roles

| Abbreviation | Verb   | Kubernetes verbs         |
| ------------ | ------ | ------------------------ |
| C            | create | create                   |
| R            | read   | get,list,watch           |
| U            | update | patch, update            |
| D            | delete | delete, deletecollection |

| Resource                             | User | PrivilegedUser | Editor | Admin | ClusterEditor | ClusterAdmin |
|--------------------------------------|------|----------------|--------|-------|---------------|--------------|
| virtualmachines                      | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualdisks                         | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualimages                        | R    | R              | R      | CRUD  | CRUD          | CRUD         |
| clustervirtualimages                 | R    | R              | R      | R     | CRUD          | CRUD         |
| virtualmachineblockdeviceattachments | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualmachineoperations             | R    | CR             | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualmachineipaddresses            | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualmachineipaddressleases        | -    | -              | -      | R     | R             | CRUD         |
| virtualmachineclasses                | R    | R              | R      | R     | CRUD          | CRUD         |

| d8 cli                        | User | PrivilegedUser | Editor | Admin | ClusterEditor | ClusterAdmin |
| ----------------------------- | ---- | -------------- | ------ | ----- | ------------- | ------------ |
| d8 v console                  | N    | Y              | Y      | Y     | Y             | Y            |
| d8 v ssh / scp / port-forward | N    | Y              | Y      | Y     | Y             | Y            |
| d8 v vnc                      | N    | Y              | Y      | Y     | Y             | Y            |
