---
title: "Initial access configuration"
permalink: en/virtualization-platform/documentation/admin/install/steps/access.html
---

After the installation is complete, you can connect to the platform in the following ways:

- From the master node, by connecting to it via SSH.
- Remotely, by configuring the connection on any personal computer.

## Connecting to the Platform from the Master Node

Connect to the master node via SSH (the master node's IP address is provided by the installer at the end of the installation):

```bash
ssh <USER_NAME>@<MASTER_IP>
```

Verify that platform resources are accessible by listing cluster nodes:

```bash
sudo -i d8 k get nodes
```

## Remote Connection to the Platform

To configure remote access to the cluster, follow the [steps in the guide](../../platform-management/access-control/user-management.html) and install the [d8 utility](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI).
