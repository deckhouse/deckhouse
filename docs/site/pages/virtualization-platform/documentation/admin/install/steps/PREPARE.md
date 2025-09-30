---
title: "Planning and preparing"
permalink: en/virtualization-platform/documentation/admin/install/steps/prepare.html
---

## Cluster Configuration Planning

Before installing the virtualization platform, you need to plan its parameters:

1. Choose the platform edition and release channel:
   - [Platform Editions](../../../about/editions.html);
   - [Release Channels](../../../about/release-channels.html).

1. Determine the IP address subnets:
   - Subnet used by nodes for internal communication;
   - Subnet for pods;
   - Subnet for services (Service);
   - Subnets for virtual machine addresses.

1. Decide on the nodes where the Ingress controller will be deployed.

1. Specify the public domain for the cluster:
   - A common practice is to use a wildcard domain that resolves to the address of the node with the Ingress controller;
   - The domain template for applications in this case will be `%s.<public wildcard domain of the cluster>`;
   - For test clusters, you can use a universal wildcard domain from the [sslip.io](https://sslip.io/) service.

     > The domain used in the template must not coincide with the domain specified in the `clusterDomain` parameter. For example, if `clusterDomain: cluster.local` (the default value) is used, then `publicDomainTemplate` cannot be `%s.cluster.local`.

1. Choose the storage to be used:
   - You can select a storage system from the [supported list](../../../about/requirements.html#supported-storage-systems);
   - [Storage configuration](../../install/steps/storage.html) will be done after the basic platform installation.

## Node Preparation

1. Check virtualization support:
   - Make sure that Intel-VT (VMX) or AMD-V (SVM) virtualization support is enabled in the BIOS/UEFI on all cluster nodes.

1. Install the operating system:
   - Install one of the [supported operating systems](../../../about/requirements.html#supported-os-for-platform-nodes) on each cluster node. Pay attention to the version and architecture of the system.

1. Check access to the container image registry:
   - Ensure that each node has access to a container image registry. By default, the installer uses the public registry `registry.deckhouse.io`. Configure network connectivity and the necessary security policies to access this repository.
   - To check access, use the command `curl https://registry.deckhouse.io/v2/`. The response should be: `401 Unauthorized`.

1. Add a technical user:

   For automated installation and configuration of cluster components on the master node, a technical user must be created. The username can be anything; in this case, the name `dvpinstall` will be used.

   - Create a user with administrator privileges:

     ```shell
     sudo useradd -m -s /bin/bash -G sudo dvpinstall
     ```

   - Set a password (make sure to save the password as it will be needed later):

     ```shell
     sudo passwd dvpinstall
     ```
  
   - (Optionally) For convenience during the installation, you can allow the `dvpinstall` user to run `sudo` without a password:

     ```shell
     visudo   
     # Add the following line:    
     dvpinstall ALL=(ALL:ALL) NOPASSWD: ALL
     ```

1. Set up SSH access:

   SSH access for the technical user must be configured on the master node.

   On the **installation machine**:

   - Generate an SSH key that will be used to access the nodes:

     ```shell
     ssh-keygen -t rsa -b 4096 -f dvp-install-key -N "" -C "dvp-node" -v
     ```

   - Using the set password, allow SSH connections using the generated key:

     ```shell
     ssh-copy-id -i dvp-install-key dvpinstall@<master-node-address>
     ```

After completing all the steps, the cluster nodes will be ready for further installation and platform configuration. Ensure each step is completed correctly to avoid issues during installation.
