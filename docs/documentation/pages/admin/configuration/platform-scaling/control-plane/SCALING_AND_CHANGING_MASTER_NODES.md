---
title: "Scaling and changing master nodes"
permalink: en/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html
---

## Scaling and single/multi-master transition

### Control plane operation modes

Deckhouse Kubernetes Platform (DKP) supports two operation modes for the control plane:

1. **Single-master**:
   - `kube-apiserver` uses only the local `etcd` instance.
   - A proxy server runs on the node to handle requests on `localhost`.
   - The `kube-apiserver` listens only on the master node's IP address.

2. **Multi-master**:
   - `kube-apiserver` interacts with all `etcd` instances in the cluster.
   - A proxy is configured on all nodes:
     - If the local `kube-apiserver` is unavailable, requests are redirected to other nodes.
   - This ensures high availability and supports scaling.

### Automatic scaling of master nodes

DKP allows automatic addition and removal of master nodes using the label `node-role.kubernetes.io/control-plane=""`.

Automatic control of master nodes includes:

- **Adding the label** `node-role.kubernetes.io/control-plane=""` to a node:
  - All control plane components are deployed.
  - The node is added to the etcd cluster.
  - Certificates and configuration files are regenerated automatically.

- **Removing the label**:
  - Control plane components are removed.
  - The node is properly removed from the etcd cluster.
  - Related configuration files are updated.

{% alert level="info" %}
Transitioning from 2 to 1 master node requires manual etcd adjustment. All other changes in master node count are handled automatically.
{% endalert %}

### Common scaling scenarios

DKP supports both automatic and manual scaling of master nodes in cloud and bare-metal clusters:

1. **Single-master → Multi-master**:

   - Add one or more master nodes.
   - Apply the label `node-role.kubernetes.io/control-plane=""` to them.
   - DKP will automatically:
     - Deploy all control plane components.
     - Configure the nodes to work with the `etcd` cluster.
     - Synchronize certificates and configuration files.

1. **Multi-master → Single-master**:

   - Remove the labels `node-role.kubernetes.io/control-plane=""` and `node-role.kubernetes.io/master=""` from the extra master nodes.
   - For **bare-metal clusters**:
     - To correctly remove the nodes from `etcd`:
       - Run `d8 k delete node <node-name>`;
       - Power off the corresponding VMs or servers.

{% alert level="warning" %}
In cloud clusters, all necessary actions are automatically handled by the `dhctl converge` command.
{% endalert %}

1. **Changing the number of master nodes in a cloud cluster**:

   - Similar to node addition/removal, typically done using the `dhctl converge` command or cloud tools.

{% alert level="warning" %}
An odd number of master nodes is required to maintain etcd quorum stability.
{% endalert %}

### Removing the master role from a node without deleting the node itself

If you need to remove a node from the set of master nodes but keep it in the cluster for other purposes, follow these steps:

1. Remove the labels so the node is no longer treated as a master:

   ```bash
   d8 k label node <node-name> node-role.kubernetes.io/control-plane-
   d8 k label node <node-name> node-role.kubernetes.io/master-
   d8 k label node <node-name> node.deckhouse.io/group-
   ```

1. Make sure that the master node to be deleted is no longer listed as a member of the etcd cluster:

   ```bash
   for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
     d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
     if [ $? -eq 0 ]; then
       break
     fi
   done
   ```

1. Delete the static manifests of the control plane components so they no longer start on the node, and remove unnecessary PKI files. Exec to the node and run the following commands:

   ```bash
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

After completing these steps, the node will no longer be considered a master node, but it will remain part of the cluster and can be used for other tasks.

### Changing the OS image of master nodes in a multi-master cluster

1. Create a [backup of etcd](../../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Make sure there are no alerts in the cluster that could interfere with updating master nodes.
1. Ensure the DKP queue is empty:

   ```shell
   d8 system queue list
   ```

1. **On your local machine**, run the Deckhouse installer container for the corresponding edition and version (adjust the container registry address if necessary):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to check the state before starting the operation:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The output should indicate that Terraform has found no discrepancies and no changes are required.

1. **In the installer container**, run the following command and specify the desired OS image in the `masterNodeGroup.instanceClass` parameter  
   (provide all master node addresses using the `--ssh-host` parameter):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **In the installer container**, run the following command to update the nodes:

   Carefully review the actions that `converge` plans to perform when it prompts for confirmation.

   During execution, nodes will be replaced with new ones, one by one, starting from the highest numbered node (2) down to the lowest (0):

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The following steps (9–12) should be performed **sequentially on each** master node, starting with the highest numbered node (with suffix 2) and ending with the lowest (with suffix 0).

1. **On the newly created node**, open the systemd journal for the `bashible.service`.  
   Wait until the setup process is complete — the log should contain the message `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Verify that the etcd node appears in the cluster node list:

   ```bash
   for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
     d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
     if [ $? -eq 0 ]; then
       break
     fi
   done
   ```

1. Make sure that [`control-plane-manager`](/modules/control-plane-manager/) is running on the node:

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Proceed to updating the next master node.

### Changing the OS image in a single-master cluster

1. Convert the single-master cluster into a multi-master one according to the [instructions](#adding-master-nodes-in-a-cloud-cluster).
1. Update the master nodes as described in the [instructions](#changing-the-os-image-of-master-nodes-in-a-multi-master-cluster).
1. Convert the multi-master cluster back to a single-master one following the [instructions](#reducing-the-number-of-master-nodes-in-a-cloud-cluster).

## Adding master nodes to a static or hybrid cluster

> It is important to have an odd number of masters to ensure a quorum.

When installing Deckhouse Kubernetes Platform with default settings, the NodeGroup `master` lacks the section [`spec.staticInstances.labelSelector`](/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-labelselector) with label filter settings for `staticInstances` resources. Because of this, after changing the number of `staticInstances` nodes in the NodeGroup `master` (parameter [`spec.staticInstances.count`](/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)), when adding a regular node using Cluster API Provider Static (CAPS), it can be "intercepted" and added to the NodeGroup `master`, even if the corresponding `StaticInstance` (in `metadata`) specifies a label with a `role` different from `master`.
To avoid this "interception", after installing DKP, edit the NodeGroup `master` — add the section [`spec.staticInstances.labelSelector`](/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-labelselector) with label filter settings for `staticInstances` resources. Example of NodeGroup `master` with `spec.staticInstances.labelSelector`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  staticInstances:
    count: 2
    labelSelector:
      matchLabels:
        role: master
```

Next, when adding master nodes to the cluster using CAPS, specify the label specified in `spec.staticInstances.labelSelector` NodeGroup `master` in the corresponding `StaticInstance`. Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: static-master-1
  labels:
    # The label specified in spec.staticInstances.labelSelector NodeGroup master.
    role: master
spec:
  # Specify the IP address of the static node server.
  address: "<SERVER-IP>"
  credentialsRef:
    kind: SSHCredentials
    name: credentials
```

{% alert level="info" %}
When adding new master nodes using CAPS and changing the number of master nodes in the NodeGroup `master` (parameter [`spec.staticInstances.count`](/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)), please note the following:

When bootstrapping the cluster, the configuration specifies the first master node on which the installation takes place.
If, after bootstrapping, you need to create a multi-master cluster and add master nodes using CAPS, you must specify the number of nodes in the `spec.staticInstances.count` parameter of the NodeGroup `master` as one less than the desired number.

For example, if you need to create a multi-master with three master nodes in `spec.staticInstances.count` NodeGroup `master`, specify the value `2` and create two `staticInstances` for the nodes to be added. After adding them to the cluster, the number of master nodes will be three: the master node on which the installation took place and two master nodes added using CAPS.
{% endalert %}

Otherwise, adding a master node to a static or hybrid cluster is similar to adding a regular node. To do this, use the corresponding [examples](../node/bare-metal-node.html#adding-nodes-to-a-bare-metal-cluster). All the necessary actions to configure a cluster control plane components on the new master nodes are performed automatically. Wait until the master nodes appear in `Ready` status.

## Adding master nodes in a cloud cluster

This section describes how to convert a single-master cluster into a multi-master cluster.

{% alert level="warning" %}
Before adding nodes, make sure the required quotas are available.
It's important to have an odd number of master nodes to maintain etcd quorum.
{% endalert %}

{% alert level="warning" %}
If your cluster uses the [`stronghold`](/modules/stronghold/) module, make sure the module is fully operational before adding or removing a master node. We strongly recommend creating a [backup of the module’s data](/modules/stronghold/auto_snapshot.html) before making any changes.
{% endalert %}

1. Create a [backup of etcd](../../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Ensure there are no active alerts in the cluster that may interfere with adding new master nodes.
1. Make sure the Deckhouse queue is empty:

   ```shell
   d8 system queue list
   ```

1. On the **local machine**, run the Deckhouse installer container for the appropriate edition and version (adjust the container registry address if necessary):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to verify the state before proceeding:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   The output should confirm that Terraform found no differences and no changes are needed.

1. **In the installer container**, run the following command and set the target number of master nodes in the `masterNodeGroup.replicas` parameter:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

   > For **Yandex Cloud**, if public IPs are assigned to master nodes, the number of elements in the `masterNodeGroup.instanceClass.externalIPAddresses` array must match the number of master nodes. Even when using the `Auto` value (automatic public IP assignment), the number of items in the array must still match.
   >
   > For example, with three master nodes (`masterNodeGroup.replicas: 3`) and automatic IP assignment, the `masterNodeGroup.instanceClass.externalIPAddresses` section would look like:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > - "Auto"
   > - "Auto"
   > ```

1. **In the installer container**, run the following command to trigger scaling:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Wait until the required number of master nodes reaches the `Ready` status and all [`control-plane-manager`](/modules/control-plane-manager/) pods become ready:

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

## Reducing the number of master nodes in a cloud cluster

This section describes the process of converting a multi-master cluster into a single-master cluster.

{% alert level="warning" %}
The following steps must be performed starting from the first master node (`master-0`) in the cluster. This is because the cluster scales in order — for example, it is not possible to remove `master-0` and `master-1` while leaving `master-2`.
{% endalert %}

{% alert level="warning" %}
If your cluster uses the [`stronghold`](/modules/stronghold/) module, make sure the module is fully operational before adding or removing a master node. We strongly recommend creating a [backup of the module’s data](/modules/stronghold/auto_snapshot.html) before making any changes.
{% endalert %}

1. Create a [backup of etcd](../../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Ensure there are no alerts in the cluster that may interfere with the master node update process.
1. Make sure the DKP queue is empty:

   ```shell
   d8 system queue list
   ```

1. On the **local machine**, run the DKP installer container for the corresponding edition and version (change the container registry address if needed):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command and set `masterNodeGroup.replicas` to `1`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   > For **Yandex Cloud**, if external IPs are used for master nodes, the number of items in the `masterNodeGroup.instanceClass.externalIPAddresses` array must match the number of master nodes. Even when using `Auto` (automatic public IP allocation), the number of entries must still match.
   >
   > For example, for a single master node (`masterNodeGroup.replicas: 1`) and automatic IP assignment, the `masterNodeGroup.instanceClass.externalIPAddresses` section would look like:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > ```

1. **In the installer container**, run the following command to trigger the scaling operation:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   > For **OpenStack** and **VKCloud(OpenStack)**, after confirming the node deletion, it is extremely important to check the disk deletion `<prefix>kubernetes-data-N` in Openstack itself.
   >
   > For example, when deleting the `cloud-demo-master-2` node in the Openstack web interface or in the OpenStack CLI, it is necessary to check the absence of the `cloud-demo-kubernetes-data-2` disk.
   >
   > If the kubernetes-data disk remains, there may be problems with ETCD operation as the number of master nodes increases.

1. Check the Deckhouse queue and make sure that there are no errors with the command:

   ```shell
   d8 system queue list
   ```

### Accessing the DKP controller in a multi-master cluster

In clusters with multiple master nodes, DKP runs in high-availability mode (with multiple replicas). To access the active DKP controller, you can use the following command (example shown for the `deckhouse-controller queue list` command):

```console
d8 system queue list
```
