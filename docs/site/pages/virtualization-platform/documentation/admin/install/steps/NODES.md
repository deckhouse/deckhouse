---
title: "Adding nodes"
permalink: en/virtualization-platform/documentation/admin/install/steps/nodes.html
---

## Adding nodes

After the initial installation, the cluster consists of only one node — the master node. To run virtual machines on the prepared worker nodes, they need to be added to the cluster.

Next, we'll cover the process of adding two worker nodes. For more detailed information about adding static nodes to the cluster, refer to the [documentation](../../platform-management/node-management/adding-node.html)).

Ensure that all preparation steps have been completed (see [preparation of worker nodes](http://deckhouse.ru/products/virtualization-platform/documentation/admin/install/steps/prepare.html)).

Create the [NodeGroup](http://deckhouse.ru/products/virtualization-platform/reference/cr/nodegroup.html) resource `worker`. To do this, execute the following command on the **master node**:

```yaml
sudo -i d8 k create -f - << EOF
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

Create an [SSHCredentials](https://deckhouse.io/products/virtualization-platform/reference/cr/sshcredentials.html) resource in the cluster. To do this, execute the following command on the **master node**:

```yaml
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
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

**On the worker node**, create the user `caps`. To do this, execute the following commands, replacing `<SSH-PUBLIC-KEY>` with the public part of the SSH key obtained in the previous step:

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
pdpl-user -i 63 caps
```

Create the [StaticInstance](http://deckhouse.io/products/virtualization-platform/reference/cr/staticinstance.html) resources.
Execute the following commands on the **master node**, specifying the IP address and unique name of each node:

```yaml
export NODE_IP=<NODE-IP-ADDRESS> # Specify the IP address of the node to be added to the cluster.
export NODE_NAME=<NODE-NAME> # Specify the unique name of the node, for example, dvp-worker-1.
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
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

Execute the following command on the **master node** to get the list of cluster nodes:

```shell
sudo -i d8 k get no
```

Example output:

```console
NAME            STATUS   ROLES                  AGE    VERSION
master-0        Ready    control-plane,master   40m    v1.29.10
dvp-worker-1    Ready    worker                 3m     v1.29.10
dvp-worker-2    Ready    worker                 3m     v1.29.10
```
