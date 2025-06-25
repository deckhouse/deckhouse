---
title: "Adding and managing bare-metal nodes"
permalink: en/admin/configuration/platform-scaling/node/bare-metal-node.html
---

## Adding nodes to a bare-metal cluster

### Manual method

1. Enable the `node-manager` module.

1. Create a `NodeGroup` object with the type `Static`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

   In this resource, specify the `Static` node type. For all NodeGroup objects in the cluster, Deckhouse automatically generates a `bootstrap.sh` script used to add nodes to the group. When adding nodes manually, you need to copy this script to the server and run it.

   You can obtain the script from the Deckhouse web interface under the “Node Groups → Scripts” tab or via the following `kubectl` command:

   ```console
   kubectl -n d8-cloud-instance-manager get secrets manual-bootstrap-for-worker -ojsonpath="{.data.bootstrap\.sh}"
   ```

   The script needs to be decoded from Base64 and then executed as `root`.

1. Once the script finishes, the server will be added to the cluster as a node in the specified group.

### Automatic method

DKP supports automatic addition of physical (bare-metal) servers to the cluster without the need to manually run an installation script on each node. To enable this:

1. Prepare the server (OS, networking):
   - Install a supported operating system;
   - Configure networking and ensure the server is accessible via SSH;
   - Create a system user (e.g., `ubuntu`) for SSH access;
   - Ensure the user can execute commands using `sudo`.

1. Create an `SSHCredentials` object to define access to the server. DKP uses this object to connect to the server over SSH. It specifies:
   - A private SSH key;
   - The OS user;
   - The SSH port;
   - (Optional) a `sudo` password, if required.

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

1. Create a `StaticInstance` object for each server:

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

   A separate `StaticInstance` resource must be created for each server, but the same `SSHCredentials` can be reused to access multiple servers.

   Possible `StaticInstance` states:

   - `Pending` — the server has not yet been configured; the corresponding node is not present in the cluster.
   - `Bootstrapping` — the server is being configured and the node is being added to the cluster.
   - `Running` — the server is successfully configured and the node has joined the cluster.
   - `Cleaning` — the server is being cleaned up and the node is being removed from the cluster.

     These states reflect the current stage of node management. CAPS automatically transitions a `StaticInstance` between these states depending on whether a node needs to be added or removed from a group.

1. Create a `NodeGroup` resource describing how DKP should use these servers:

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

   This section defines parameters for using `StaticInstance` resources:  
   - `count` specifies how many nodes will be added to the group;  
   - `labelSelector` defines the rules for selecting nodes.

   When using the Cluster API Provider Static (CAPS), it is important to correctly set the `nodeType` to `Static` and provide the `staticInstances` section in the `NodeGroup` resource:

   - If the `labelSelector` is not specified, CAPS will use any available `StaticInstance` resources in the cluster.
   - The same `StaticInstance` can be used in multiple NodeGroups if it matches the filters.
   - CAPS automatically maintains the number of nodes in the group according to the `count` parameter.
   - When a node is removed, CAPS performs cleanup and disconnection, and the corresponding `StaticInstance` transitions to the `Pending` status, allowing it to be reused.

After the NodeGroup is created, a bootstrap script will become available for adding servers to this group.  
DKP will wait until the required number of `StaticInstance` objects matching the label selector appear in the cluster.  
Once such an object appears, DKP will retrieve the server’s IP address and SSH connection parameters from the previously created manifests, connect to the server via SSH, and execute the `bootstrap.sh` script on it.  
After that, the server will be added to the specified group as a node.

## Moving a node between NodeGroups

{% alert level="warning" %}
When moving a node between NodeGroups, the node will be cleaned up and bootstrapped again.  
The corresponding `Node` object will be recreated.
{% endalert %}

1. Create a new `NodeGroup` resource, for example named `front`, which will manage the static node labeled `role: front`:

   ```yaml
   kubectl create -f - <<EOF
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

1. Change the `role` label of the existing `StaticInstance` from `worker` to `front`.  
   This will allow the new `NodeGroup` named `front` to manage this node:

   ```console
   kubectl label staticinstance static-worker-1 role=front --overwrite
   ```

1. Update the `worker` NodeGroup resource by decreasing the `count` parameter from `1` to `0`:

   ```console
   kubectl patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
   ```

### Manual cleanup of a static node

To remove a node from the cluster and clean the server (virtual machine), run the `/var/lib/bashible/cleanup_static_node.sh` script, which is already present on every static node.

Example of disconnecting a node from the cluster and cleaning the server:

```console
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

{% alert level="info" %}
This instruction applies both to nodes manually configured using the bootstrap script and to nodes configured via CAPS.
{% endalert %}

## NodeGroup example

### Static nodes

For virtual machines on hypervisors or physical servers, use static nodes by setting `nodeType: Static` in the NodeGroup.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

## Settings for Static and CloudStatic NodeGroups

Node groups with types `Static` and `CloudStatic` are intended for managing manually created nodes — either physical (bare-metal) or virtual (in the cloud, but outside DKP automation). These nodes are connected manually or via `StaticInstance` and do not support automatic updates or scaling.

Configuration specifics:

- All update operations (e.g., kubelet updates, node restarts, replacements) must be performed manually or through external automation tools outside of DKP.

- It is recommended to explicitly set the desired `kubelet` version to ensure consistency across nodes, especially if they are added with different versions manually:

  ```yaml
  nodeTemplate:
     kubelet:
       version: "1.28"
  ```

- Node registration to the cluster can be performed either manually or automatically, depending on the configuration:
  - **Manual** — the user downloads the bootstrap script, configures the server, and runs the script manually.
  - **Automatic (CAPS)** — when using `StaticInstance` and `SSHCredentials`, DKP automatically connects and configures the nodes.
  - **Hybrid approach** — a manually added node can be handed over to CAPS by using the annotation `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

If the Cluster API Provider Static (CAPS) is enabled, the `NodeGroup` resource can use the `staticInstances` section. This allows DKP to automatically connect, configure, and, if necessary, clean up static nodes based on `StaticInstance` and `SSHCredentials` resources.

## How to change CRI for a NodeGroup

{% alert level="warning" %} 
CRI can only be switched between `Containerd` and `NotManaged` via the `cri.type` parameter.
{% endalert %}

To change the CRI for a NodeGroup, set the `cri.type` parameter to either `Containerd` or `NotManaged`.

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
  kubectl patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* To set `NotManaged`:

  ```shell
  kubectl patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

{% alert level="warning" %}
When changing the `cri.type` for a NodeGroup created using `dhctl`, you must also update this value in `dhctl config edit provider-cluster-configuration` and in the NodeGroup object settings.
{% endalert %}

After changing the CRI for a NodeGroup, the `node-manager` module will sequentially reboot the nodes, applying the new CRI.  
Node updates involve disruption. Depending on the `disruption` settings for the NodeGroup, the `node-manager` module will either automatically update the nodes or require manual approval.

## How to automatically assign custom labels to a node

1. On the node, create the directory `/var/lib/node_labels`.

1. Inside that directory, create one or more files containing the desired labels. You can use any number of files and any level of nested subdirectories.

1. Add the required labels to the files in `key=value` format. For example:

   ```console
   example-label=test
   ```

1. Save the files.

When the node is added to the cluster, the labels specified in these files will be automatically applied to the node.

{% alert level="warning" %}  
Note that it is not possible to assign DKP-reserved labels using this method. It only works with custom labels that do not conflict with those reserved by Deckhouse.
{% endalert %}

## How to change the NodeGroup for a static node

If a node is managed by CAPS, it is not possible to change its associated NodeGroup.  
The only option is to [delete the StaticInstance](#can-a-staticinstance-be-deleted) and create a new one.

To move an existing manually added static node from one `NodeGroup` to another, you need to update the group label on the node:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

## How to change the IP address of a StaticInstance

You cannot change the IP address of a `StaticInstance` resource.  
If an incorrect address is specified in a `StaticInstance`, you need to [delete the StaticInstance](#can-a-staticinstance-be-deleted) and create a new one.

## Can a StaticInstance be deleted

A `StaticInstance` in the `Pending` state can be safely deleted without any issues.

To delete a `StaticInstance` that is in any state other than `Pending` (`Running`, `Cleaning`, `Bootstrapping`), follow these steps:

1. Add the label `"node.deckhouse.io/allow-bootstrap": "false"` to the `StaticInstance`.
1. Wait until the `StaticInstance` transitions to the `Pending` state.
1. Delete the `StaticInstance`.
1. Decrease the `NodeGroup.spec.staticInstances.count` parameter by 1.
