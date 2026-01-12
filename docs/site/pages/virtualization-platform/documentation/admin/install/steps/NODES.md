---
title: "Adding nodes"
permalink: en/virtualization-platform/documentation/admin/install/steps/nodes.html
---

After the initial installation, the cluster consists of only one node — the master node. To run virtual machines on the prepared worker nodes, they need to be added to the cluster.

Next, we'll cover the process of adding two worker nodes. For more detailed information about adding static nodes to the cluster, refer to the [documentation](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/bare-metal-node.html).

{% alert level=“info” %}
To run the commands below, you need to have the [d8 utility](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI) installed and a configured kubectl context for accessing the cluster.
Alternatively, you can connect to the master node via SSH and run the command as the `root` user using `sudo -i`.
{% endalert %}

## Node preparation

1. Make sure that Intel-VT (VMX) or AMD-V (SVM) virtualization support is enabled in the BIOS/UEFI on all cluster nodes.

1. Install one of the [supported operating systems](../../../about/requirements.html#supported-os-for-platform-nodes) on each cluster node. Pay attention to the version and architecture of the system.

1. Check access to the container image registry:
   - Ensure that each node has access to a container image registry. By default, the installer uses the public registry `registry.deckhouse.io`. Configure network connectivity and the necessary security policies to access this repository.
   - To check access, use the following command:

     ```shell
     curl https://registry.deckhouse.io/v2/
     ```

     Expected output:

     ```console
     401 Unauthorized
     ```

## Adding prepared nodes

Create the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource `worker`. To do this, run the following command:

```yaml
d8 k create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
 name: worker
spec:
 nodeType: Static
 staticInstances:
   count: 2
   labelSelector:
     matchLabels:
       role: worker
EOF
```

Generate an SSH key with an empty passphrase. To do this, execute the following command on the **master node**:

```shell
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```

Create an [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource in the cluster. To do this, execute the following command on the **master node**:

```yaml
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: caps
spec:
  user: caps
  privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
EOF
```

Retrieve the public part of the previously generated SSH key (it will be needed in the next step). To do this, execute the following command on the **master node**:

```shell
cat /dev/shm/caps-id.pub
```

**On the worker node**, create the user `caps`. To do this, run the following commands, replacing `<SSH-PUBLIC-KEY>` with the public part of the SSH key obtained in the previous step:

```shell
export KEY='<SSH-PUBLIC-KEY>' # Specify the public part of the SSH key.
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```

**In Astra Linux operating systems**, when using the mandatory integrity control module Parsec, configure the maximum integrity level for the user `caps`:

```shell
sudo -i pdpl-user -i 63 caps
```

Create the [StaticInstance](/modules/node-manager/cr.html#staticinstance) resources. Run the following commands on the **master node**, specifying the IP address and unique name of each node:

```yaml
export NODE_IP=<NODE-IP-ADDRESS> # Specify the IP address of the node to be added to the cluster.
export NODE_NAME=<NODE-NAME> # Specify the unique name of the node, for example, dvp-worker-1.
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: "$NODE_NAME"
  labels:
    role: worker
spec:
  address: "$NODE_IP"
  credentialsRef:
    kind: SSHCredentials
    name: caps
EOF
```

Ensure that all nodes in the cluster are in the `Ready` status.

Run the following command to get the list of cluster nodes:

```shell
d8 k get no
```

{% offtopic title="Example output..." %}

```console
NAME            STATUS   ROLES                  AGE    VERSION
master-0        Ready    control-plane,master   40m    v1.29.10
dvp-worker-1    Ready    worker                 3m     v1.29.10
dvp-worker-2    Ready    worker                 3m     v1.29.10
```

{% endofftopic %}
