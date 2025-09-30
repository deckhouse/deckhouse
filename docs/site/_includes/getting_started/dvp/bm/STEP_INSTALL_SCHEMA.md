{% alert level="warning" %}
Here are examples of steps for installing Deckhouse Virtualization Platform with the following parameters:

- one <strong>master node</strong>,
- one <strong>worker node</strong>,
- software-defined replicated storage based on DRBD,
- one virtual machine.

If you need a different configuration, please refer to the documentation on [installing the platform](../../documentation/admin/install/steps/prepare.html).
{% endalert %}


## Installation requirements

1. **Personal computer.** The computer from which the installation will be performed.  It is only needed to run the installer and will not be part of the cluster.

   Requirements:

   - OS: Windows 10+, macOS 10.15+, Linux (e.g. Ubuntu 20.04+, Fedora 35+);
   - installed Docker to run the installer (here are the instructions for [Ubuntu](https://docs.docker.com/engine/install/ubuntu/), [macOS](https://docs.docker.com/desktop/mac/install/), [Windows](https://docs.docker.com/desktop/windows/install/));
   - HTTPS access to the `registry.deckhouse.io` container image registry;
   - SSH key access to the node, the **master node** of the future cluster;
   - SSH key access to the node, the **worker node** of the future cluster.

1. **Physical server or virtual machine for the master node.**

   Requirements:

   - at least 4 CPU cores
   - at least 8 GB of RAM
   - at least 60 GB of disk space for the cluster and etcd data on a fast disk (400+ IOPS)
   - [supported OS](/products/virtualization-platform/documentation/about/requirements.html#supported-os-for-platform-nodes)
   - Linux kernel version >= `5.8`
   - CPU with x86_64 architecture supporting Intel-VT (VMX) or AMD-V (SVM) instructions
   - **Unique hostname** within servers (virtual machines) of the cluster
   - HTTPS access to the `registry.deckhouse.io` container image registry
   - access to the default package repositories for the operating system you are using
   - SSH key access from the **personal computer** (section 1)
   - network access from the **personal computer** (section 1) via port `22/TCP`
   - container runtime packages, such as containerd or Docker, should not be installed on the node
   - `cloud-utils` and `cloud-init` packages should be installed on the node.

1. **Physical server or virtual machine for the worker node.**

   The requirements are similar to the requirements for the master node but also depend on the applications running on the nodes.
   Additional disks are also required on the worker nodes for deploying software-defined storage.
