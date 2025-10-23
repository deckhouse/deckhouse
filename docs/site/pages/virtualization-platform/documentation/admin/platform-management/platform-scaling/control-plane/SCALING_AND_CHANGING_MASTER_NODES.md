---
title: "Scaling and changing master nodes"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/scaling-and-changing-master-nodes.html
---

## Scaling and single/multi-master transition

### Control plane operation modes

Deckhouse Virtualization Platform (DVP) supports two operation modes for the control plane:

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

DVP allows automatic addition and removal of master nodes using the label `node-role.kubernetes.io/control-plane=""`.

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

DVP supports both automatic and manual scaling of master nodes in bare-metal clusters:

1. **Single-master → Multi-master**:

   - Add one or more master nodes.
   - Apply the label `node-role.kubernetes.io/control-plane=""` to them.
   - DVP will automatically:
     - Deploy all control plane components.
     - Configure the nodes to work with the `etcd` cluster.
     - Synchronize certificates and configuration files.

1. **Multi-master → Single-master**:

   - Remove the labels `node-role.kubernetes.io/control-plane=""` and `node-role.kubernetes.io/master=""` from the extra master nodes.
   - For **bare-metal clusters**:
     - To correctly remove the nodes from `etcd`:
       - Run `d8 k delete node <node-name>`;
       - Power off the corresponding VMs or servers.

### Removing the master role from a node without deleting the node itself

If you need to remove a node from the set of master nodes but keep it in the cluster for other purposes, follow these steps:

1. Remove the labels so the node is no longer treated as a master:

   ```bash
   d8 k label node <node-name> node-role.kubernetes.io/control-plane-
   d8 k label node <node-name> node-role.kubernetes.io/master-
   d8 k label node <node-name> node.deckhouse.io/group-
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
   for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
     d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
     if [ $? -eq 0 ]; then
       break
     fi
   done
   ```

After completing these steps, the node will no longer be considered a master node, but it will remain part of the cluster and can be used for other tasks.

### Changing the OS image of master nodes in a multi-master cluster

1. Create a [backup of etcd](/products/virtualization-platform/documentation/admin/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Make sure there are no alerts in the cluster that could interfere with updating master nodes.
1. Ensure the DVP queue is empty:

   ```shell
   d8 platform queue list
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
