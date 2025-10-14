---
title: "Adding and managing bare-metal nodes"
permalink: en/admin/configuration/platform-scaling/node/bare-metal-node.html
description: "Manage bare-metal nodes in Deckhouse Kubernetes Platform. Node addition, configuration, and lifecycle management."
---

## Adding nodes to a bare-metal cluster

### Manual method

1. Enable the [`node-manager`](/modules/node-manager/cr.html) module.

1. Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) object with the type `Static`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

   In this resource, specify the `Static` node type. For all NodeGroup objects in the cluster, Deckhouse automatically generates a `bootstrap.sh` script used to add nodes to the group. When adding nodes manually, you need to copy this script to the server and run it.

   You can obtain the script from the Deckhouse web interface under the “Node Groups → Scripts” tab or via the following `d8 k` command:

   ```shell
   d8 k -n d8-cloud-instance-manager get secrets manual-bootstrap-for-worker -ojsonpath="{.data.bootstrap\.sh}"
   ```

   The script needs to be decoded from Base64 and then executed as `root`.

1. Once the script finishes, the server will be added to the cluster as a node in the specified group.

### Automatic method

{% alert level="warning" %}
If you have previously increased the number of master nodes in the cluster in the NodeGroup `master` (parameter [`spec.staticInstances.count`](/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)), before adding nodes using automatic method, [make sure](../control-plane/scaling-and-changing-master-nodes.html#adding-master-nodes-to-a-static-or-hybrid-cluster) that they will not be "captured".
{% endalert %}

DKP supports automatic addition of physical (bare-metal) servers to the cluster without the need to manually run an installation script on each node. To enable this:

1. Prepare the server (OS, networking):
   - Install a supported operating system.
   - Configure networking and ensure the server is accessible via SSH.
   - Create a system user (e.g., `ubuntu`) for SSH access.
   - Ensure the user can execute commands using `sudo`.

1. Create an [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) object to define access to the server. DKP uses this object to connect to the server over SSH. It specifies:
   - A private SSH key.
   - The OS user.
   - The SSH port.
   - (Optional) A `sudo` password, if required.

     Example:

     ```yaml
     apiVersion: deckhouse.io/v1alpha1
     kind: SSHCredentials
     metadata:
       name: static-nodes
     spec:
       privateSSHKey: |
         -----BEGIN OPENSSH PRIVATE KEY-----
         LS0tLS1CRUdJlhrdG...................VZLS0tLS0K
         -----END OPENSSH PRIVATE KEY-----
       sshPort: 22
       sudoPassword: password
       user: ubuntu
     ```

     > **Important**. The private key must match the corresponding public key added to the `~/.ssh/authorized_keys` file on the server.

1. Create a [StaticInstance](/modules/node-manager/cr.html#staticinstance)` object for each server:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
     labels:
       static-node: auto
   spec:
     address: 192.168.1.10
     credentialsRef:
       apiVersion: deckhouse.io/v1alpha1
       kind: SSHCredentials
       name: static-nodes
   ```

   A separate [StaticInstance](/modules/node-manager/cr.html#staticinstance) resource must be created for each server, but the same SSHCredentials can be reused to access multiple servers.

   Possible StaticInstance states:

   - `Pending`: The server has not yet been configured; the corresponding node is not present in the cluster.
   - `Bootstrapping`: The server is being configured and the node is being added to the cluster.
   - `Running`: The server is successfully configured and the node has joined the cluster.
   - `Cleaning`: The server is being cleaned up and the node is being removed from the cluster.

   These states reflect the current stage of node management. CAPS automatically transitions a StaticInstance between these states depending on whether a node needs to be added or removed from a group.

1. Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource describing how DKP should use these servers:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 3
       labelSelector:
         matchLabels:
           static-node: auto
     nodeTemplate:
       labels:
         node-role.deckhouse.io/worker: ""
   ```

   This section defines parameters for using StaticInstance resources:

   - `count` specifies how many nodes will be added to the group.  
   - `labelSelector` defines the rules for selecting nodes.

   When using the Cluster API Provider Static (CAPS), it is important to correctly set the `nodeType` to `Static` and provide the `staticInstances` section in the NodeGroup resource:

   - If the `labelSelector` is not specified, CAPS will use any available StaticInstance resources in the cluster.
   - The same StaticInstance can be used in multiple NodeGroups if it matches the filters.
   - CAPS automatically maintains the number of nodes in the group according to the `count` parameter.
   - When a node is removed, CAPS performs cleanup and disconnection, and the corresponding StaticInstance transitions to the `Pending` status, allowing it to be reused.

After the node group is created, a script for adding servers to the group will become available. DKP will wait for the required number of StaticInstance objects that match the specified labels. As soon as such an object appears, DKP will use the provided IP address and SSH connection parameters to run the `bootstrap.sh` script and add the server to the group.

## Modifying a static cluster configuration

The static cluster settings are stored in the [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) structure.

To modify the static cluster parameters, run the following command:

```shell
d8 platform edit static-cluster-configuration
```

## Moving a static node between NodeGroups

{% alert level="warning" %}
During the migration of static nodes between [NodeGroups](/modules/node-manager/cr.html#nodegroup), the node is cleaned up and bootstrapped again, and the `Node` object is recreated.
{% endalert %}

1. Create a new NodeGroup resource, for example named `front`, which will manage the static node labeled `role: front`:

   ```yaml
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: front
   ```

1. Change the `role` label of the existing [StaticInstance](/modules/node-manager/cr.html#staticinstance) from `worker` to `front`.  
   This will allow the new NodeGroup named `front` to manage this node:

   ```shell
   d8 k label staticinstance static-worker-1 role=front --overwrite
   ```

1. Update the `worker` NodeGroup resource by decreasing the `count` parameter from `1` to `0`:

   ```shell
   d8 k patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
   ```

### Manual cleanup of a static node

To remove a node from the cluster and clean the server, run the `/var/lib/bashible/cleanup_static_node.sh` script, which is already present on every static node.

Example of disconnecting a node from the cluster and cleaning the server:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

{% alert level="info" %}
This instruction applies both to nodes manually configured using the bootstrap script and to nodes configured via CAPS.
{% endalert %}

## NodeGroup example

### Example NodeGroup definition for static nodes

For virtual machines on hypervisors or physical servers, use static nodes by setting `nodeType: Static` in the [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Nodes are added to such a group manually using preconfigured scripts or automatically via CAPS.

## Changing the CRI for a NodeGroup

CRI (Container Runtime Interface) is a standard interface between the kubelet and the container runtime.

{% alert level="warning" %}
CRI can only be switched between `Containerd` and `NotManaged` via the `cri.type` parameter.
{% endalert %}

To change the CRI for a [NodeGroup](/modules/node-manager/cr.html#nodegroup), set the `cri.type` parameter to either `Containerd` or `NotManaged`.

Example YAML manifest:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: Containerd
```

You can also perform this operation using a patch:

* To set `Containerd`:

  ```shell
  d8 k patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* To set `NotManaged`:

  ```shell
  d8 k patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

{% alert level="warning" %}
When changing the `cri.type` for a NodeGroup created using `dhctl`, you must also update this value in `dhctl config edit provider-cluster-configuration` and in the NodeGroup object settings.
{% endalert %}

After changing the CRI for a NodeGroup, the [`node-manager`](/modules/node-manager/) module will sequentially reboot the nodes, applying the new CRI.  
Node updates involve disruption. Depending on the `disruption` settings for the NodeGroup, the `node-manager` module will either automatically update the nodes or require manual approval.

## Changing the NodeGroup of a static node

If a node is managed by [CAPS](#automatic-method), you can't change its associated NodeGroup.  
The only option is to [delete the StaticInstance](#deleting-a-staticinstance) and create a new one.

To move an existing manually added static node from one NodeGroup to another, you need to update the group label on the node:

```shell
d8 k label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
d8 k label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

It will take some time for the changes to take effect.

## Changing the IP address in a StaticInstance

You cannot change the IP address of a [StaticInstance](/modules/node-manager/cr.html#staticinstance) resource.  
If an incorrect address is specified in a StaticInstance, you need to [delete the StaticInstance](#deleting-a-staticinstance) and create a new one.

## Deleting a StaticInstance

A [StaticInstance](/modules/node-manager/cr.html#staticinstance) in the `Pending` state can be safely deleted without any issues.

To delete a StaticInstance that is in any state other than `Pending` (`Running`, `Cleaning`, `Bootstrapping`), follow these steps:

1. Add the label `"node.deckhouse.io/allow-bootstrap": "false"` to the StaticInstance.

   Example command for adding a label:

   ```shell
   d8 k label staticinstance d8cluster-worker node.deckhouse.io/allow-bootstrap=false
   ```

1. Wait until the StaticInstance transitions to the `Pending` state.

   To check the status of StaticInstance, use the command:

   ```shell
   d8 k get staticinstances
   ```

1. Delete the StaticInstance.

   Example command for deleting StaticInstance:

   ```shell
   d8 k delete staticinstance d8cluster-worker
   ```

1. Decrease the `NodeGroup.spec.staticInstances.count` parameter by 1.
