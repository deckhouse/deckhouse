---
title: "Initial access configuration"
permalink: en/stronghold/documentation/admin/install/steps/access.html
---

After the installation is complete, you can connect to the platform in the following ways:

- Directly from the master node
- Remotely from a pre-configured personal computer

## Connecting to the Platform from the Master Node

Connect to the master node via SSH (the master node's IP address is provided by the installer at the end of the installation):

```bash
ssh <USER_NAME>@<MASTER_IP>
```

Verify that platform resources are accessible by listing the cluster nodes:

```bash
d8 k get nodes
```

## Remote Connection to the Platform

You can set up a remote connection to the cluster. To do this, follow the steps on your personal computer as described in [the instructions](../../platform-management/access-control/user-management.html#create-a-configuration-file-for-remote-access).
