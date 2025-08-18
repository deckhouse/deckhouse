---
title: "Adding and removing a node"
permalink: en/virtualization-platform/documentation/admin/platform-management/node-management/adding-node.html
lang: en
---

## Adding a static node to a cluster

<span id='adding-node-to-cluster'></span>

You can add a static node manually or using the Cluster API Provider Static (CAPS).

### Adding a static node manually

To add a bare-metal server to a cluster as a static node, follow these steps:

1. Use the existing [NodeGroup](../../../../reference/cr/nodegroup.html) custom resource or create a new one,
   setting the `Static` or `CloudStatic` value for the [`nodeType`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-nodetype) parameter.

   Example of a NodeGroup resource named `worker`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

1. Get the Base64-encoded script code to add and configure the node.

   Example command to get the Base64-encoded script code to add a node to the `worker` NodeGroup:

   ```shell
   NODE_GROUP=worker
   kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
   ```

1. Pre-configure the new node according to your environment specifics:
   - Add all the necessary mount points to the `/etc/fstab` file: NFS, Ceph, etc.
   - Install the necessary packages.
   - Set up network connectivity between the new node and the other nodes of the cluster.
1. Connect to the new node over SSH and run the following command, inserting the Base64 string you got in step 2:

   ```shell
   echo <Base64-CODE> | base64 -d | bash
   ```

### Adding a static node using CAPS

To learn more about Cluster API Provider Static (CAPS), refer to [Configuring a node via CAPS](node-group.html#configuring-a-node-via-caps).

Example of adding a static node to a cluster using CAPS:

To add a static node to a cluster (bare metal server or virtual machine), follow these steps:

1. **Allocate a server with an installed operating system (OS)** and set up network connectivity.
   If necessary, install additional OS-specific packages and add mount points to use on the node.

   - Create a user (named `caps` in the following example) capable of running `sudo` by running the following command on the server:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   - Allow the user to run `sudo` commands without having to enter a password.
     To do that, add the following line to the `sudo` configuration on the server
     by either editing the `/etc/sudoers` file, running the `sudo visudo` command, or via any other method:

     ```text
     caps ALL=(ALL) NOPASSWD: ALL
     ```

   - Generate a pair of SSH keys with an empty passphrase on the server using the following command:

     ```shell
     ssh-keygen -t rsa -f caps-id -C "" -N ""
     ```

     The public and private keys of the `caps` user will be stored in the `caps-id.pub` and `caps-id` files
     in the current directory on the server.

   - Add the generated public key to the `/home/caps/.ssh/authorized_keys` file of the `caps` user
     by running the following commands in the directory storing the keys on the server:

     ```shell
     mkdir -p /home/caps/.ssh 
     cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
     chmod 700 /home/caps/.ssh 
     chmod 600 /home/caps/.ssh/authorized_keys
     chown -R caps:caps /home/caps/
     ```

1. **Create a SSHCredentials resource in the cluster**.

   - To access the added server, CAPS requires the private key of the service user `caps`.
     The Base64-encoded key is added to the SSHCredentials resource.

     To encode the private key to Base64, run the following command in the user key directory on the server:

     ```shell
     base64 -w0 caps-id
     ```

   - On any computer configured to manage the cluster,
     create an environment variable with the value of the Base64-encoded private key you generated earlier.
     To prevent the key from saving in the shell history, add a whitespace character at the beginning of the command:

     ```shell
     CAPS_PRIVATE_KEY_BASE64=<PRIVATE_KEY_IN_BASE64>
     ```

   - Create a SSHCredentials resource with the service user name and associated private key:

     ```shell
     d8 k create -f - <<EOF
     apiVersion: deckhouse.io/v1alpha1
     kind: SSHCredentials
     metadata:
       name: static-0-access
     spec:
       user: caps
       privateSSHKey: "${CAPS_PRIVATE_KEY_BASE64}"
     EOF
     ```

1. **Create a StaticInstance resource in the cluster**.

   The StaticInstance resource defines the IP address of the static node server and the data required to access the server:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
   spec:
     # Specify the static node server's IP address.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: static-0-access
   EOF
   ```

1. **Create a NodeGroup resource in the cluster**:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
   EOF
   ```

1. **Wait until the NodeGroup resource is in the `Ready` state**.
   To check the resource state, run the following command:

   ```shell
   d8 k get ng worker
   ```

   In the NodeGroup state, 1 node should appear in the `READY` column:

   ```console
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   worker   Static   1       1       1                                                                 15m   True
   ```

### Adding a static node using CAPS and label selector filters

<span id="caps-with-label-selector"></span>

To connect different StaticInstance resources to different NodeGroup resources,
you can use the label selector specified in the NodeGroup and StaticInstance metadata.

In the following example, you can see how three static nodes are distributed between two NodeGroup resources:
one into the `worker` group, and two others into the `front` group.

1. Prepare the required resources (three servers) and create the SSHCredentials resources for them,
   following steps 1 and 2 from the [previous scenario](#adding-a-static-node-using-caps).
1. Create two NodeGroup in the cluster.

   Specify `labelSelector`, so that only the corresponding servers could connect to the NodeGroup resources:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 2
       labelSelector:
         matchLabels:
           role: front
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

1. Create the StaticInstance resources in the cluster.

   Specify the actual IP addresses of the servers and set the `role` label in metadata:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-1
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP1>"
     credentialsRef:
       kind: SSHCredentials
       name: front-1-credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-2
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP2>"
     credentialsRef:
       kind: SSHCredentials
       name: front-2-credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     address: "<SERVER-WORKER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: worker-1-credentials
   EOF
   ```

1. To check the result, run the following command:

   ```shell
   d8 k get ng
   ```

   In the output, you should see a list of created NodeGroup resources, with static nodes distributed between them:

   ```console
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   master   Static   1       1       1                                                               1h     True
   front    Static   2       2       2                                                               1h     True
   ```

## How do I know if something went wrong?

If a node in a NodeGroup isn't updated
(the`UPTODATE` value is less than the `NODES` value when executing the `kubectl get nodegroup` command)
or you assume there are other problems that may be related to the `node-manager` module,
check the logs of the `bashible` service. The `bashible` service runs on each node managed by the `node-manager` module.

To view the logs of the `bashible` service, run the following command on the node:

```shell
journalctl -fu bashible
```

Example of output when the `bashible` service has performed all necessary actions:

```console
May 25 04:39:16 kube-master-0 systemd[1]: Started Bashible service.
May 25 04:39:16 kube-master-0 bashible.sh[1976339]: Configuration is in sync, nothing to do.
May 25 04:39:16 kube-master-0 systemd[1]: bashible.service: Succeeded.
```

## Removing a node from a cluster

<span id='remove-node-from-node-manager-management'></span>

{% alert level="info" %}
The procedure is valid for both a manually configured node (using the bootstrap script) and a node configured using CAPS.
{% endalert %}

To disconnect a node from a cluster and clean up the server (VM), run the following command on the node:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

### How do I clean up a node for adding to another cluster?

{% alert level="warning" %}
This is only necessary if you need to move a static node from one cluster to another.
Note that these operations result in removing data from the local storage.
If you only need to change a NodeGroup, follow the [NodeGroup changing procedure](#how-do-i-change-the-nodegroup-of-a-static-node) instead.

If the node you are cleaning up has the LINSTOR/DRBD storage pools,
to evict resources from the node and remove the LINSTOR/DRBD node,
use the [corresponding procedure](https://deckhouse.io/products/kubernetes-platform/modules/sds-replicated-volume/stable/faq.html#how-do-i-evict-drbd-resources-from-a-node) for the `sds-replicated-volume` module.
{% endalert %}

To clean up a node for adding to another cluster, follow these steps:

1. Remove the node from the Kubernetes cluster:

   ```shell
   kubectl drain <node> --ignore-daemonsets --delete-local-data
   kubectl delete node <node>
   ```

1. Run the clean-up script on the node:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. After the node is restarted, it can be added to another cluster.

## FAQ

### Can I delete a StaticInstance?

A StaticInstance in the `Pending` state can be deleted safely.

To delete a StaticInstance in any state other than `Pending`, such as `Running`, `Cleaning`, or `Bootstrapping`,
do the following:

1. Add the label `"node.deckhouse.io/allow-bootstrap": "false"` to the StaticInstance.

   Example command for adding a label:

   ```shell
   d8 k label staticinstance d8cluster-worker node.deckhouse.io/allow-bootstrap=false
   ```

1. Wait until the StaticInstance status is changed to `Pending`.

   To check the status of StaticInstance, use the command:

   ```shell
   d8 k get staticinstances
   ```

1. Delete the StaticInstance.

   Example command for deleting StaticInstance:

   ```shell
   d8 k delete staticinstance d8cluster-worker
   ```

1. Decrease the `NodeGroup.spec.staticInstances.count` parameter's value by 1.
1. Wait until the NodeGroup is in the `Ready` state.

### How do I change the IP address of a StaticInstance?

You cannot change the IP address in the StaticInstance resource.
If an incorrect address is specified in the StaticInstance,
you have to [delete the StaticInstance](#can-i-delete-a-staticinstance) and create a new one.

### How do I migrate a manually configured static node under CAPS control?

You need to [clean up the node](#removing-a-node-from-a-cluster)
and then [hand it over](#adding-a-static-node-using-caps) under CAPS control.

### How do I change the NodeGroup of a static node?

If a node is under CAPS control, you **can't** change the NodeGroup membership of such a node.
The only way is to [delete a StaticInstance](#can-i-delete-a-staticinstance) and create a new one.

To switch an existing manually added static node to another NodeGroup,
change its group label and delete its role label using the following commands:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Applying the changes will take some time.

### How do I know what is running on a node while it is being created?

To find out what's happening on a node (for example, when it's taking too long to create or stuck in the `Pending` state),
you can check the `cloud-init` logs.
To do that, follow these steps:

1. Find the node that is currently bootstrapping:

   ```shell
   kubectl get instances | grep Pending
   ```

   An output example:

   ```console
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Get information about connection parameters for viewing logs:

   ```shell
   kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   An output example:

   ```console
   bootstrapStatus:
     description: Use 'nc 192.168.199.178 8000' to get bootstrap logs.
     logsEndpoint: 192.168.199.178:8000
   ```

1. To view the `cloud-init` logs for diagnostics,
   run the command you got (`nc 192.168.199.178 8000` according to the example above).

   The logs of the initial node configuration are located at `/var/log/cloud-init-output.log`.
