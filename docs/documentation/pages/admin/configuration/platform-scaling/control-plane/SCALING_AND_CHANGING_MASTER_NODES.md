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
       - Run `kubectl delete node <node-name>`;
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
   kubectl label node <node-name> node-role.kubernetes.io/control-plane-
   kubectl label node <node-name> node-role.kubernetes.io/master-
   kubectl label node <node-name> node.deckhouse.io/group-
   ```

1. Delete the static manifests of the control plane components so they no longer start on the node, and remove unnecessary PKI files:

   ```bash
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Check the node's status in the etcd cluster using `etcdctl member list`.

   Example:

   ```bash
   kubectl -n kube-system exec -ti \
   $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

After completing these steps, the node will no longer be considered a master node, but it will remain part of the cluster and can be used for other tasks.

### Changing the OS image of master nodes in a multi-master cluster

1. Create a [backup of etcd](../../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Make sure there are no alerts in the cluster that could interfere with updating master nodes.
1. Ensure the DKP queue is empty.
1. **On your local machine**, run the Deckhouse installer container for the corresponding edition and version (adjust the container registry address if necessary):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
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
   kubectl -n kube-system exec -ti \
   $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Make sure that [`control-plane-manager`](/modules/control-plane-manager/) is running on the node:

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Proceed to updating the next master node.

### Changing the OS image in a single-master cluster

1. Convert the single-master cluster into a multi-master one according to the [instructions](#adding-master-nodes-in-a-cloud-cluster).
1. Update the master nodes as described in the [instructions](#changing-the-os-image-of-master-nodes-in-a-multi-master-cluster).
1. Convert the multi-master cluster back to a single-master one following the [instructions](#reducing-the-number-of-master-nodes-in-a-cloud-cluster).

## Adding master nodes in a cloud cluster

This section describes how to convert a single-master cluster into a multi-master cluster.

{% alert level="warning" %}
Before adding nodes, make sure the required quotas are available.
It's important to have an odd number of master nodes to maintain etcd quorum.
{% endalert %}

1. Create a [backup of etcd](../../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Ensure there are no active alerts in the cluster that may interfere with adding new master nodes.
1. Make sure the Deckhouse queue is empty:

   ```shell
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

1. On the **local machine**, run the Deckhouse installer container for the appropriate edition and version (adjust the container registry address if necessary):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
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
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

## Reducing the number of master nodes in a cloud cluster

This section describes the process of converting a multi-master cluster into a single-master cluster.

{% alert level="warning" %}
The following steps must be performed starting from the first master node (`master-0`) in the cluster. This is because the cluster scales in order — for example, it is not possible to remove `master-0` and `master-1` while leaving `master-2`.
{% endalert %}

1. Create a [backup of etcd](../../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Ensure there are no alerts in the cluster that may interfere with the master node update process.
1. Make sure the DKP queue is empty:

   ```shell
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

1. On the **local machine**, run the DKP installer container for the corresponding edition and version (change the container registry address if needed):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
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

1. Remove the following labels from the master nodes you plan to delete:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Command to remove the labels:

   ```bash
   kubectl label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Make sure the nodes to be removed are no longer part of the etcd cluster:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Drain the nodes to be removed:

   ```bash
   kubectl drain <MASTER-NODE-N-NAME> --ignore-daemonsets --delete-emptydir-data
   ```

1. Power off the corresponding VMs, delete their instances from the cloud, and detach any associated disks (e.g., `kubernetes-data-master-<N>`).

1. Delete any remaining pods on the removed nodes:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=<MASTER-NODE-N-NAME> --force
   ```

1. Delete the `Node` objects for the removed nodes:

   ```bash
   kubectl delete node <MASTER-NODE-N-NAME>
   ```

1. **In the installer container**, run the following command to trigger the scaling operation:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```
