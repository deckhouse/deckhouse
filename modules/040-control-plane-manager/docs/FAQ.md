---
title: "Managing control plane: FAQ"
---

<div id='how-do-i-add-a-master-node'></div>

## How do I add a master node to a static or hybrid cluster?

> It is important to have an odd number of masters to ensure a quorum.

When installing Deckhouse Kubernetes Platform with default settings, the NodeGroup `master` lacks the section [`spec.staticInstances.labelSelector`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-labelselector) with label filter settings for `staticInstances` resources. Because of this, after changing the number of `staticInstances` nodes in the NodeGroup `master` (parameter [`spec.staticInstances.count`](../ node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)), when adding a regular node using Cluster API Provider Static (CAPS), it can be "intercepted" and added to the NodeGroup `master`, even if the corresponding `StaticInstance` (in `metadata`) specifies a label with a `role` different from `master`.
To avoid this "interception", after installing DKP, edit the NodeGroup `master` — add the section [`spec.staticInstances.labelSelector`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-labelselector) with label filter settings for `staticInstances` resources. Example of NodeGroup `master` with `spec.staticInstances.labelSelector`:

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
When adding new master nodes using CAPS and changing the number of master nodes in the NodeGroup `master` (parameter [`spec.staticInstances.count`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)), please note the following:

When bootstrapping the cluster, the configuration specifies the first master node on which the installation takes place.
If, after bootstrapping, you need to create a multi-master cluster and add master nodes using CAPS, you must specify the number of nodes in the `spec.staticInstances.count` parameter of the NodeGroup `master` as one less than the desired number.

For example, if you need to create a multi-master with three master nodes in `spec.staticInstances.count` NodeGroup `master`, specify the value `2` and create two `staticInstances` for the nodes to be added. After adding them to the cluster, the number of master nodes will be three: the master node on which the installation took place and two master nodes added using CAPS.
{% endalert %}

Otherwise, adding a master node to a static or hybrid cluster is similar to adding a regular node. To do this, use the corresponding [examples](../node-manager/examples.html#adding-a-static-node-to-a-cluster). All the necessary actions to configure a cluster control plane components on the new master nodes are performed automatically. Wait until the master nodes appear in `Ready` status.

<div id='how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master'></div>

## How do I add master nodes to a cloud cluster?

The following describes the conversion of a single-master cluster into a multi-master.

> Before adding nodes, ensure you have the required quotas in the cloud provider.
>
> It is important to have an odd number of masters to ensure a quorum.

{% alert level="warning" %}
If your cluster uses the [`stronghold`](/modules/stronghold/) module, make sure the module is fully operational before adding or removing a master node. We strongly recommend creating a [backup of the module’s data](/modules/stronghold/auto_snapshot.html) before making any changes.
{% endalert %}

1. Make a [backup of `etcd`](faq.html#etcd-backup-and-restore) and the `/etc/kubernetes` directory.
1. Transfer the archive to a server outside the cluster (e.g., on a local machine).
1. Ensure there are no [alerts](../prometheus/faq.html#how-to-get-information-about-alerts-in-a-cluster) in the cluster that can prevent the creation of new master nodes.
1. Make sure that Deckhouse queue is empty:

   ```shell
   d8 system queue list
   ```

1. Run the appropriate edition and version of the Deckhouse installer container **on the local machine** (change the container registry address if necessary):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to check the state before working:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   The command output should indicate that Terraform found no inconsistencies and no changes are required.

1. **In the installer container**, run the following command and specify the required number of replicas using the `masterNodeGroup.replicas` parameter:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

   > For **Yandex Cloud**, when using external addresses on master nodes, the number of array elements in the [masterNodeGroup.instanceClass.externalIPAddresses](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-externalipaddresses) parameter must equal the number of master nodes. If `Auto` is used (public IP addresses are provisioned automatically), the number of array elements must still equal the number of master nodes.
   >
   > To illustrate, with three master nodes (`masterNodeGroup.replicas: 3`) and automatic address reservation, the `masterNodeGroup.instanceClass.externalIPAddresses` parameter would look as follows:
   >
   > ```bash
   > externalIPAddresses:
   > - "Auto"
   > - "Auto"
   > - "Auto"
   > ```

1. **In the installer container**, run the following command to start scaling:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Wait until the required number of master nodes are `Ready` and all `control-plane-manager` instances are up and running:

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

<div id='how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster-multi-master-to-single-master'></div>

## How do I reduce the number of master nodes in a cloud cluster?

The following describes the conversion of a multi-master cluster into a single-master.

{% alert level="warning" %}
The steps described below must be performed from the first in order of the master node of the cluster (master-0). This is because the cluster is always scaled in order: for example, it is impossible to delete nodes master-0 and master-1, leaving master-2.
{% endalert %}

{% alert level="warning" %}
If your cluster uses the [`stronghold`](/modules/stronghold/) module, make sure the module is fully operational before adding or removing a master node. We strongly recommend creating a [backup of the module’s data](/modules/stronghold/auto_snapshot.html) before making any changes.
{% endalert %}

1. Create a [backup of etcd](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
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

## How do I dismiss the master role while keeping the node?

1. Make a [backup of `etcd`](faq.html#etcd-backup-and-restore) and the `/etc/kubernetes` directory.
1. Transfer the archive to a server outside the cluster (e.g., on a local machine).
1. Ensure there are no [alerts](../prometheus/faq.html#how-to-get-information-about-alerts-in-a-cluster) in the cluster that can prevent the update of the master nodes.
1. Make sure that Deckhouse queue is empty:

   ```shell
   d8 system queue list
   ```

1. Remove the following labels:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Use the following command to remove labels:

   ```bash
   d8 k label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
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

1. Exec to the node and run the following commands:

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

## How do I switch to a different OS image in a multi-master cluster?

1. Make a [backup of `etcd`](faq.html#etcd-backup-and-restore) and the `/etc/kubernetes` directory.
1. Transfer the archive to a server outside the cluster (e.g., on a local machine).
1. Ensure there are no [alerts](../prometheus/faq.html#how-to-get-information-about-alerts-in-a-cluster) in the cluster that can prevent the update of the master nodes.
1. Make sure that Deckhouse queue is empty:

   ```shell
   d8 system queue list
   ```

1. Run the appropriate edition and version of the Deckhouse installer container **on the local machine** (change the container registry address if necessary):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to check the state before working:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The command output should indicate that Terraform found no inconsistencies and no changes are required.

1. **In the installer container**, run the following command and specify the required OS image using the `masterNodeGroup.instanceClass` parameter (specify the addresses of all master nodes using the `-ssh-host` parameter):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **In the container with the installer**, run the following command to update the nodes:

   Carefully review the actions that converge plans to perform when it asks for confirmation.

   When the command is executed, the nodes will be replaced by new nodes with confirmation on each node. The replacement will be performed one by one in reverse order (2,1,0).

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Repeat the steps below (Sec. 9-12) for **each master node one by one**, starting with the node with the highest number (suffix 2) and ending with the node with the lowest number (suffix 0).

1. **On the newly created node**, check the systemd-unit log for the `bashible.service`. Wait until the node configuration is complete (you will see a message `nothing to do` in the log):

   ```bash
   journalctl -fu bashible.service
   ```

1. Make sure the node is listed as an etcd cluster member:

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

1. Make sure `control-plane-manager` is running on the node:

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Proceed to update the next node (repeat the steps above).

## How do I switch to a different OS image in a single-master cluster?

1. Convert your single-master cluster to a multi-master one, as described in [the guide on adding master nodes to a cluster](#how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master).
1. Update the master nodes following the [instructions](#how-do-i-switch-to-a-different-os-image-in-a-multi-master-cluster).
1. Convert your multi-master cluster to a single-master one according to [the guide on excluding master nodes from the cluster](#how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster).

## How to configure HA with two master nodes and an arbiter node?

The Deckhouse Kubernetes Platform allows you to configure HA with two master nodes and an arbiter node. This approach allows you to meet HA requirements in conditions of limited resources.

Only etcd is placed on the arbiter node, without the other control plane components. This node is used to ensure the etcd quorum.

Requirements for the arbiter node:

* At least 2 CPU cores;
* At least 4 GB of RAM;
* At least 8 GB of disk space for etcd.

The network latency requirements for the arbiter node are similar to those for the master nodes.

### Configuring in a cloud cluster

The example below applies to a cloud cluster with three master nodes.
To configure HA with two master nodes and an arbiter node in a cloud cluster, you need to remove one master node from the cluster and add one arbiter node.

To do this, follow these steps:

{% alert level="warning" %}
If your cluster uses the [`stronghold`](/modules/stronghold/) module, make sure the module is fully operational before adding or removing a master node. We strongly recommend creating a [backup of the module’s data](/modules/stronghold/auto_snapshot.html) before making any changes.
{% endalert %}

1. Create a [backup of etcd](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
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

1. **In the installer container**, run the following command:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   Change the cloud provider settings

   * Set `masterNodeGroup.replicas` to `2`.
   * Create a NodeGroup for the arbiter node. The arbiter node **must have** the label `node-role.deckhouse.io/etcd-only: ""` and a taint that prevents user workloads from being placed on it. Example of a NodeGroup description for the arbiter node:

     ```yaml
     nodeGroups:
       - name: arbiter
         replicas: 1
         nodeTemplate:
           labels:
             node.deckhouse.io/etcd-arbiter: ""
           taints:
             - key: node.deckhouse.io/etcd-arbiter
               effect: NoSchedule
         zones:
           - europe-west3-b
         instanceClass:
           machineType: n1-standard-4
       # ... the rest of the manifesto
     ```

   * Save your changes.

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
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST>
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

### Configuring in a static cluster

To configure HA with two master nodes and an arbiter node in a static cluster, follow these steps:

1. Create a NodeGroup for the arbiter node. The arbiter node **must have** the label `node-role.deckhouse.io/etcd-only: “”` and a taint that prevents user workloads from being placed on it. Example of a NodeGroup description for the arbiter node:

   ```yaml
   apiVersion: deckhouse.io/v1
     kind: NodeGroup
     metadata:
       name: arbiter
     spec:
       nodeType: Static
       nodeTemplate:
         labels:
           node.deckhouse.io/etcd-arbiter: ""
         taints:
           - key: node.deckhouse.io/etcd-arbiter
             effect: NoSchedule
     # ... the rest of the manifesto
     ```

1. Add a node to the cluster that will be used as an arbiter node in a [way that is convenient](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/bare-metal-node.html#adding-nodes-to-a-bare-metal-cluster) for you.
1. [Ensure](#how-do-i-view-the-list-of-etcd-members) that the added arbiter node is in the list of etcd cluster members.
1. [Remove](#how-do-i-dismiss-the-master-role-while-keeping-the-node) one master node from the cluster.

## How do I view the list of etcd members?

### Option 1

Use the `etcdctl member list` command.

Example:

```shell
for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
  d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ member list -w table
  if [ $? -eq 0 ]; then
    break
  fi
done
```

**Warning.** The last parameter in the output table shows etcd member is in [`learner`](https://etcd.io/docs/v3.5/learning/design-learner/) state, is not in `leader` state.

### Option 2

To obtain information about etcd cluster nodes in tabular form, use the `etcdctl endpoint status` command. For the leader, the `IS LEADER` column will show `true`.

Example:

```shell
for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
  d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ endpoint status --cluster -w table
  if [ $? -eq 0 ]; then
    break
  fi
done
```

## What if something went wrong?

During its operation, `control-plane-manager` automatically creates backups of configurations and data that may be useful in case of issues. These backups are stored in the `/etc/kubernetes/deckhouse/backup` directory. If errors or unforeseen situations occur during operation, you can use these backups to restore to the previous stable state.

## What if the etcd cluster fails?

If the etcd cluster is not functioning and it cannot be restored from a backup, you can attempt to rebuild it from scratch by following the steps below.

1. First, on all nodes that are part of your etcd cluster, except for one, remove the `etcd.yaml` manifest located in the `/etc/kubernetes/manifests/` directory. This last node will serve as a starting point for the new multi-master cluster.
1. On the last node, edit etcd manifest `/etc/kubernetes/manifests/etcd.yaml` and add the parameter `--force-new-cluster` to `spec.containers.command`.
1. After the new cluster is ready, remove the `--force-new-cluster` parameter.

{% alert level="warning" %}
This operation is unsafe and breaks the guarantees given by the consensus protocol. Note that it brings the cluster to the state that was saved on the node. Any pending entries will be lost.
{% endalert %}

### What if etcd restarts with an error?

This method may be necessary if the `--force-new-cluster` option doesn't restore etcd work. Such a scenario can occur during an unsuccessful converge of master nodes, where a new master node was created with an old etcd disk, changed its internal address, and other master nodes are absent. Symptoms indicating the need for this method include: the etcd container being stuck in an endless restart with the log showing the error: `panic: unexpected removal of unknown remote peer`.

1. Install the [etcdutl](https://github.com/etcd-io/etcd/releases) utility.
1. Create a new etcd database snapshot from the current local snapshot (`/var/lib/etcd/member/snap/db`):

   ```shell
   ./etcdutl snapshot restore /var/lib/etcd/member/snap/db --name <HOSTNAME> \
   --initial-cluster=HOSTNAME=https://<ADDRESS>:2380 --initial-advertise-peer-urls=https://ADDRESS:2380 \
   --skip-hash-check=true --data-dir /var/lib/etcdtest
   ```

   * `<HOSTNAME>` — the name of the master node;
   * `<ADDRESS>` — the address of the master node.

1. Execute the following commands to use the new snapshot:

   ```shell
   cp -r /var/lib/etcd /tmp/etcd-backup
   rm -rf /var/lib/etcd
   mv /var/lib/etcdtest /var/lib/etcd
   ```

1. Locate the `etcd` and `api-server` containers:

   ```shell
   crictl ps -a | egrep "etcd|apiserver"
   ```

1. Remove the `etcd` and `api-server` containers:

   ```shell
   crictl rm <CONTAINER-ID>
   ```

1. Restart the master node.

### What to do if the database volume of etcd reaches the limit set in quota-backend-bytes?

When the database volume of etcd reaches the limit set by the `quota-backend-bytes` parameter, it switches to "read-only" mode. This means that the etcd database stops accepting new entries but remains available for reading data. You can tell that you are facing a similar situation by executing the command:

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | sed -n 1p) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ endpoint status -w table --cluster
   ```

If you see a message like `alarm:NOSPACE` in the `ERRORS` field, you need to take the following steps:

1. Make change to `/etc/kubernetes/manifests/etcd.yaml` — find the line with `--quota-backend-bytes` and edit it. If there is no such line — add, for example: `- --quota-backend-bytes=8589934592` - this sets the limit to 8 GB.

1. Disarm the active alarm that occurred due to reaching the limit. To do this, execute the command:

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | sed -n 1p) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Change the [maxDbSize](configuration.html#parameters-etcd-maxdbsize) parameter in the `control-plane-manager` settings  to match the value specified in the manifest.

## How to defragment etcd

{% alert level="warning" %}
Before defragmenting, [back up etcd](#how-to-manually-backup-etcd).
{% endalert %}

To view the size of the etcd database on a specific node before and after defragmentation, use the command (where `NODE_NAME` is the name of the master node):

```bash
d8 k -n kube-system exec -it etcd-NODE_NAME -- /usr/bin/etcdctl \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  endpoint status --cluster -w table
```

Output example (the size of the etcd database on the node is specified in the `DB SIZE` column):

```console
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
|          ENDPOINT           |        ID        | VERSION | STORAGE VERSION | DB SIZE | IN USE | PERCENTAGE NOT IN USE | QUOTA  | IS LEADER  | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS | DOWNGRADE TARGET VERSION | DOWNGRADE ENABLED |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
| https://192.168.199.80:2379 | 489a8af1e7acd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |       true |      false |        56 |  258054684 |          258054684 |        |                          |             false |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
| https://192.168.199.81:2379 | 589a8ad1e7ccd7b0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
| https://192.168.199.82:2379 | 229a8cd1e7bcd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
```

### How to defragment an etcd node in a single-master cluster

{% alert level="warning" %}
Defragmenting etcd is a resource-intensive operation that temporarily blocks etcd from running on that node.
Keep this in mind when choosing a time to perform the operation in a cluster with a single master node.
{% endalert %}

To defragment etcd in a cluster with a single master node, use the following command (where `NODE_NAME` is the name of the master node):

```bash
d8 k -n kube-system exec -ti etcd-NODE_NAME -- /usr/bin/etcdctl \
  --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ defrag --command-timeout=30s
```

Example output when the operation is successful:

```console
Finished defragmenting etcd member[https://localhost:2379]. took 848.948927ms
```

> If a timeout error occurs, increase the value of the `–command-timeout` parameter from the command above until defragmentation is successful.

### How to defragment etcd in a cluster with multiple master nodes

To defragment etcd in a cluster with multiple master nodes:

1. Get a list of etcd pods. To do this, use the following command:

   ```bash
   d8 k -n kube-system get pod -l component=etcd -o wide
   ```

   Example output:

   ```console
   NAME           READY    STATUS    RESTARTS   AGE     IP              NODE        NOMINATED NODE   READINESS GATES
   etcd-master-0   1/1     Running   0          3d21h   192.168.199.80  master-0    <none>           <none>
   etcd-master-1   1/1     Running   0          3d21h   192.168.199.81  master-1    <none>           <none>
   etcd-master-2   1/1     Running   0          3d21h   192.168.199.82  master-2    <none>           <none>
   ```

1. Identify the leader master node. To do this, contact any etcd pod and get a list of nodes participating in the etcd cluster using the command (where `NODE_NAME` is the name of the master node):

   ```bash
   d8 k -n kube-system exec -it etcd-NODE_NAME -- /usr/bin/etcdctl \
     --cert=/etc/kubernetes/pki/etcd/server.crt \
     --key=/etc/kubernetes/pki/etcd/server.key \
     --cacert=/etc/kubernetes/pki/etcd/ca.crt \
     endpoint status --cluster -w table
   ```

   Output example (the leader in the `IS LEADER` column will have the value `true`):

   ```console
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   |          ENDPOINT           |        ID        | VERSION | STORAGE VERSION | DB SIZE | IN USE | PERCENTAGE NOT IN USE | QUOTA  | IS LEADER  | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS | DOWNGRADE TARGET VERSION | DOWNGRADE ENABLED |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   | https://192.168.199.80:2379 | 489a8af1e7acd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |       true |      false |        56 |  258054684 |          258054684 |        |                          |             false |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   | https://192.168.199.81:2379 | 589a8ad1e7ccd7b0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   | https://192.168.199.82:2379 | 229a8cd1e7bcd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   ```

1. Defragment the etcd nodes that are members of the etcd cluster one by one. Use the following command to defragment (where `NODE_NAME` is the name of the master node):

   > Important: Defragment the leader last.
   >
   > Restoring etcd on a node after defragmentation may take some time. It is recommended to wait at least a minute before proceeding to defragment the next etcd node.

      ```bash
   d8 k -n kube-system exec -ti etcd-NODE_NAME -- /usr/bin/etcdctl \
     --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ defrag --command-timeout=30s
   ```

   Example output when the operation is successful:

   ```console
   Finished defragmenting etcd member[https://localhost:2379]. took 848.948927ms
   ```

   > If a timeout error occurs, increase the value of the `–command-timeout` parameter from the command above until defragmentation is successful.

## How do I configure additional audit policies?

1. Enable [the auditPolicyEnabled](configuration.html#parameters-apiserver-auditpolicyenabled) flag in the module configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
     settings:
       apiserver:
         auditPolicyEnabled: true
   ```

2. Create the `kube-system/audit-policy` Secret containing a Base64 encoded YAML file:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <base64>
   ```

   The minimum viable example of the `audit-policy.yaml` file looks as follows:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   You can find detailed information on how to configure `audit-policy.yaml` file here:
   * [The official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy);
   * [The code of the generator script used in GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

### How to omit Deckhouse built-in policy rules?

Set the [apiserver.basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) module parameter to `false`.

An example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      basicAuditPolicyEnabled: false
```

### How to stream audit log to stdout instead of files?

Set the [apiserver.auditLog.output](configuration.html#parameters-apiserver-auditlog) parameter to `Stdout`.

An example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      auditLog:
        output: Stdout
```

### How to deal with the audit log?

There must be some `log scraper` on master nodes  *([log-shipper](../log-shipper/cr.html#clusterloggingconfig), promtail, filebeat)* that will monitor the log file:

```bash
/var/log/kube-audit/audit.log
```

The following fixed parameters of log rotation are in use:

* The maximum disk space is limited to `1000 Mb`.
* Logs older than `30 days` will be deleted.

Depending on the `Policy` settings and the number of requests to the `apiserver`, the amount of logs collected may be high. Thus, in some cases, logs can only be kept for less than 30 minutes.

{% alert level="warning" %}
The current implementation of this feature isn't safe and may lead to a temporary failure of the control plane.

The `apiserver` will not be able to start if there are unsupported options or a typo in the Secret.
{% endalert %}

If `apiserver` is unable to start, you have to manually disable the `--audit-log-*` parameters in the `/etc/kubernetes/manifests/kube-apiserver.yaml` manifest and restart apiserver using the following command:

```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
# Or (depending on your CRI).
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

After the restart, you will be able to fix the Secret or delete it:

```bash
d8 k -n kube-system delete secret audit-policy
```

## How do I speed up the restart of Pods if the connection to the node has been lost?

By default, a node is marked as unavailable if it does not report its state for 40 seconds. After another 5 minutes, its Pods will be rescheduled to other nodes. Thus, the overall application unavailability lasts approximately 6 minutes.

In specific cases, if an application cannot run in multiple instances, there is a way to lower its unavailability time:

1. Reduce the period required for the node to become `Unreachable` if the connection to it is lost by setting the `nodeMonitorGracePeriodSeconds` parameter.
1. Set a lower timeout for evicting Pods on a failed node using the `failedNodePodEvictionTimeoutSeconds` parameter.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

In this case, if the connection to the node is lost, the applications will be restarted in about 1 minute.

Both these parameters directly impact the CPU and memory resources consumed by the control plane. By lowering timeouts, we force system components to send statuses more frequently and check the resource state more often.

When deciding on the appropriate threshold values, consider resources consumed by the control nodes (graphs can help you with this). Note that the lower parameters are, the more resources you may need to allocate to these nodes.

## etcd backup and restore

### What is done automatically

CronJob `kube-system/d8-etcd-backup-*` is automatically started at 00:00 UTC+0. The result is saved in `/var/lib/etcd/etcd-backup.tar.gz` on all nodes with `control-plane` in the cluster (master nodes).

### How to manually backup etcd

#### Using Deckhouse CLI (Deckhouse Kubernetes Platform v1.65+)

Starting with Deckhouse Kubernetes Platform v1.65, a new `d8 backup etcd` tool is available for taking snapshots of etcd state.

```bash
d8 backup etcd ./etcd-backup.snapshot
```

#### Using bash (Deckhouse Kubernetes Platform v1.64 and older)

Login into any control-plane node with `root` user and use next script:

```bash
#!/usr/bin/env bash
set -e

pod=etcd-`hostname`
d8 k -n kube-system exec "$pod" -- /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /var/lib/etcd/${pod##*/}.snapshot && \
mv /var/lib/etcd/"${pod##*/}.snapshot" etcd-backup.snapshot && \
cp -r /etc/kubernetes/ ./ && \
tar -cvzf kube-backup.tar.gz ./etcd-backup.snapshot ./kubernetes/
rm -r ./kubernetes ./etcd-backup.snapshot
```

In the current directory etcd snapshot file `kube-backup.tar.gz` will be created from one of an etcd cluster members.
From this file, you can restore the previous etcd cluster state in the future.

Also, we recommend making a backup of the `/etc/kubernetes` directory, which contains:

* manifests and configurations of [control-plane components](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components);
* [Kubernetes cluster PKI](https://kubernetes.io/docs/setup/best-practices/certificates/).

This directory will help to quickly restore a cluster in case of complete loss of control-plane nodes without creating a new cluster
and without rejoin the remaining nodes into the new cluster.

We recommend encrypting etcd snapshot backups as well as backup of the directory `/etc/kubernetes/` and saving them outside the Deckhouse cluster.
You can use one of third-party files backup tools, for example: [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/), etc.

You can see [documentation](https://github.com/deckhouse/deckhouse/blob/main/modules/040-control-plane-manager/docs/internal/ETCD_RECOVERY.md) for learn about etcd disaster recovery procedures from snapshots.

### How do perform a full recovery of the cluster state from an etcd backup?

The following steps will be described to restore to the previous state of the cluster from a backup in case of complete data loss.

#### Restoring a single-master cluster

Follow these steps to restore a single-master cluster on master node:

1. Find `etcdutl` utility on the master-node and copy the executable to `/usr/local/bin/`:

   ```shell
   cp $(find /var/lib/containerd/ \
   -name etcdutl -print | tail -n 1) /usr/local/bin/etcdutl
   ```

   Check the version of `etcdutl` using the command:

   ```shell
   etcdutl version
   ```

   The result must be a correct output of `etcdutl version` command without errors.

   Alternatively, you can download [etcdutl](https://github.com/etcd-io/etcd/releases) executable to the node (preferably its version is the same as the etcd version in the cluster):

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.6.1/etcd-v3.6.1-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.6.1-linux-amd64.tar.gz && mv etcd-v3.6.1-linux-amd64/etcdutl /usr/local/bin/etcdutl
   ```

   You can check current `etcd` version using following command (might not work, if etcd and Kubernetes API are already unavailable):

   ```shell
   d8 k -n kube-system exec -ti etcd-$(hostname) -- etcdutl version
   ```

1. Stop the etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Save the current etcd data.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Clean the etcd directory.

   ```shell
   rm -rf /var/lib/etcd
   ```

1. Put the etcd backup to `~/etcd-backup.snapshot` file.

1. Restore the etcd database.

   ```shell
   ETCDCTL_API=3 etcdutl snapshot restore ~/etcd-backup.snapshot  --data-dir=/var/lib/etcd
   ```

1. Run etcd. The process may take some time.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

   To verify that etcd is running, use the command:

   ```shell
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

   Output example:

   ```console
   CONTAINER        IMAGE            CREATED              STATE     NAME      ATTEMPT     POD ID          POD
   4b11d6ea0338f    16d0a07aa1e26    About a minute ago   Running   etcd      0           ee3c8c7d7bba6   etcd-gs-test
   ```

1. Restart the master node.

#### Restoring a multi-master cluster

Follow these steps to restore a multi-master cluster:

1. Explicitly set the High Availability (HA) mode by specifying the [highAvailability](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-highavailability) parameter. This is necessary, for example, in order not to lose one Prometheus replica and its PVC, since HA is disabled by default in single-master mode.

1. Switch the cluster to single-master mode according to [instruction](#how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster) for cloud clusters or independently remove static master-node from the cluster.

1. On a single master-node, perform the steps to restore etcd from backup in accordance with the [instructions](#restoring-a-single-master-cluster) for a single-master cluster.

1. When etcd operation is restored, delete the information about the master nodes already deleted in step 1 from the cluster:

   ```shell
   d8 k delete node MASTER_NODE_I
   ```

1. Restart all nodes of the cluster.

1. Wait for the deckhouse queue to complete:

   ```shell
   d8 system queue main
   ```

1. Switch the cluster back to multi-master mode according to [instructions](#how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master) for cloud clusters or [instructions](#how-do-i-add-a-master-node-to-a-static-or-hybrid-cluster) for static or hybrid clusters.

### How do I restore a Kubernetes object from an etcd backup?

To get cluster objects data from an etcd backup, you need:

1. Start an temporary instance of etcd.
1. Fill it with data from the [backup](#how-to-manually-backup-etcd).
1. Get desired objects using `auger`.

#### Example of steps to restore objects from an etcd backup

In the example below, `etcd-backup.snapshot` is a [etcd shapshot](#how-to-manually-backup-etcd), `infra-production` is the namespace in which objects need to be restored.

* To decode objects from `etcd` you would need [auger](https://github.com/etcd-io/auger/tree/main). It can be built from source on any machine that has Docker installed (it cannot be done on cluster nodes).

  ```shell
  git clone -b v1.0.1 --depth 1 https://github.com/etcd-io/auger
  cd auger
  make release
  build/auger -h
  ```
  
* Resulting executable `build/auger`, and also the `snapshot` from the backup copy of etcd must be uploaded on master-node, on which following actions would be performed.

Following actions are performed on a master node, to which `etcd snapshot` file and `auger` tool were copied:

1. Set the correct access permissions for the backup file:

   ```shell
   chmod 644 etcd-backup.snapshot
   ```

1. Set full path for snapshot file and for the tool into environmental variables:

   ```shell
   SNAPSHOT=/root/etcd-restore/etcd-backup.snapshot
   AUGER_BIN=/root/auger 
   chmod +x $AUGER_BIN
   ```

1. Run a Pod with temporary instance of `etcd`.
   * Create Pod manifest. It should schedule on current master node by `$HOSTNAME` variable, and mounts snapshot file by `$SNAPSHOT` variable, which it then restores in temporary `etcd` instance:

     ```shell
     cat <<EOF >etcd.pod.yaml 
     apiVersion: v1
     kind: Pod
     metadata:
       name: etcdrestore
       namespace: default
     spec:
       nodeName: $HOSTNAME
       tolerations:
       - operator: Exists
       initContainers:
       - command:
         - etcdutl
         - snapshot
         - restore
         - "/tmp/etcd-snapshot"
         - --data-dir=/default.etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-snapshot-restore
         # Uncomment the fragment below to set limits for the container if the node does not have enough resources to run it.
         # resources:
         #   requests:
         #     ephemeral-storage: "200Mi"
         #   limits:
         #     ephemeral-storage: "500Mi"
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
         - name: etcd-snapshot
           mountPath: /tmp/etcd-snapshot
           readOnly: true
       containers:
       - command:
         - etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-temp
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
       volumes:
       - name: etcddir
         emptyDir: {}
         # Use the snippet below instead of emptyDir: {} to set limits for the container if the node's resources are insufficient to run it.
         # emptyDir:
         #  sizeLimit: 500Mi
       - name: etcd-snapshot
         hostPath:
           path: $SNAPSHOT
           type: File
     EOF
     ```

   * Create Pod from the resulting manifest:

     ```shell
     d8 k create -f etcd.pod.yaml
     ```

1. Set environment variables. In this example:

   * `infra-production` - namespace which we will search resources in.

   * `/root/etcd-restore/output` - path for outputting recovered resource manifests.

   * `/root/auger` - path to `auger` executable.

     ```shell
     FILTER=infra-production
     BACKUP_OUTPUT_DIR=/root/etcd-restore/output
     mkdir -p $BACKUP_OUTPUT_DIR && cd $BACKUP_OUTPUT_DIR
     ```

1. Commands below will filter needed resources by `$FILTER` and output them into `$BACKUP_OUTPUT_DIR` directory:

   ```shell
   files=($(d8 k -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get / --prefix --keys-only | grep "$FILTER"))
   for file in "${files[@]}"
   do
     OBJECT=$(d8 k -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get "$file" --print-value-only | $AUGER_BIN decode)
     FILENAME=$(echo $file | sed -e "s#/registry/##g;s#/#_#g")
     echo "$OBJECT" > "$BACKUP_OUTPUT_DIR/$FILENAME.yaml"
     echo $BACKUP_OUTPUT_DIR/$FILENAME.yaml
   done
   ```

1. From resulting `yaml` files, delete `creationTimestamp`, `UID`, `status` and other operational fields, and then restore the objects:

   ```bash
   d8 k create -f deployments_infra-production_supercronic.yaml
   ```

1. Delete the Pod with a temporary instance of etcd:

   ```bash
   d8 k -n default delete pod etcdrestore
   ```

## How the node to run the Pod on is selected

The Kubernetes scheduler component selects the node to run the Pod on.

The selection process involves two phases, namely `Filtering` and `Scoring`. They are supposed to efficiently distribute the Pods between the nodes.

Although there are some additional phases, such as `pre-filtering`, `post-filtering`, and so on, you can safely narrow them down to the global phases mentioned above, as they merely increase flexibility and help to optimize things.

### The structure of the Kubernetes scheduler

The Scheduler comprises plugins that function in either or both phases.

Example of plugins:

* **ImageLocality** — favors nodes that already have the container images that the Pod runs. Phase: `Scoring`.
* **TaintToleration** — implements [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Phases: `Filtering`, `Scoring`.
* **NodePorts** - checks whether the ports required for the Pod to run are available on the node. Phase: `Filtering`.

The full list of plugins is available in the [Kubernetes documentation](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

### Working logic

#### Scheduler profiles

There are two predefined scheduler profiles:

* `default-scheduler`: The default profile that distributes pods to nodes with the lowest load;
* `high-node-utilization`: A profile that places pods on nodes with the highest load.

To specify a scheduler profile, use the `spec.schedulerName` parameter in the pod manifest.

Example of using a profile:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: scheduler-example
  labels:
    name: scheduler-example
spec:
  schedulerName: high-node-utilization
  containers:
  - name: example-pod
    image: registry.k8s.io/pause:2.0  
```

#### Pod scheduling stages

The selection process starts with the `Filtering` phase. During it, `filter` plugins select nodes that satisfy filter conditions such as `taints`, `nodePorts`, `nodeName`, `unschedulable`, etc.
If the nodes are in different zones, the scheduler alternates zones when selecting to ensure that all Pods will not end up in the same zone.

Suppose there are two zones with the following nodes:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

In this case, the nodes will be selected in the following order:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4.
```

Note that Kubernetes limits the number of nodes to calculate their scores during scheduling. This optimizes the selection process and prevents unnecessary scoring.
By default, the threshold is linear. For clusters with less than or equal to 50 nodes, 100% of nodes are considered for scheduling; for clusters with 100 nodes, a 50%-threshold is used; and for clusters with 5000 nodes, a 10%-threshold is used. The minimum threshold value is 5% for clusters with more than 5000 nodes. Therefore, even if all the conditions are met, a node may not be included in the list of candidates for scheduling if the default settings are used.

This logic can be changed (read more about the parameter `percentage Of Nodes To Score` in the [Kubernetes documentation](https://kubernetes.io/docs/reference/config-api/kube-scheduler-config.v1/)), but Deckhouse does not provide such an option.

The `Scoring` phase follows once the nodes that meet the conditions are selected. Each plugin evaluates the filtered node list and assigns a score to each node based on available resources: `pod capacity`, `affinity`, `volume provisioning`, and other factors. The scores from the different plugins are then summed up and the node with the highest score is selected. If several nodes have the same score, the node is selected at random.

Finally, the scheduler assigns the Pod to the node with the highest ranking.

#### Documentation

* [General description of the scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).
* [Plugin system](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).
* [Node Filtering Details](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/).
* [Scheduler source code](https://github.com/kubernetes/kubernetes/tree/master/cmd/kube-scheduler).

### How to change or extend the scheduler logic

To change the logic of the scheduler it is possible to use the extension mechanism [Extenders](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/624-scheduling-framework/README.md).

Each plugin is a webhook that must satisfy the following requirements:

* Use of TLS.
* Accessibility through a service within the cluster.
* Support for standard `Verbs` (`filterVerb = filter`, `prioritizeVerb = prioritize`).
* It is also assumed that all plugins can cache node information (`nodeCacheCapable: true`).

You can connect an `extender` using [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration) resource.

{% alert level="danger" %}
When using the `failurePolicy: Fail` option, in case of an error in the webhook's operation, the scheduler will stop working and new pods will not be able to start.
{% endalert %}

## How does kubelet certificate rotation work?

In Deckhouse Kubernetes Platform, kubelet certificate rotation is automatic.

The kubelet uses a client TLS certificate (`/var/lib/kubelet/pki/kubelet-client-current.pem`) with which it can request a new client certificate or a new server certificate (`/var/lib/kubelet/pki/kubelet-server-current.pem`) from kube-apiserver.

When there is 5-10% (random value from the range) of time left before the certificate expires, kubelet requests a new certificate from kube-apiserver. For a description of the algorithm, see the official [Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#bootstrap-initialization) documentation.

### Certificates lifetime

By default, lifetime of certificates is 1 year (8760 hours). If necessary, this value can be changed using `--cluster-signing-duration` argument in `/etc/kubernetes/manifests/kube-controller-manager.yaml` manifest. But to ensure that kubelet has time to install the certificate before it expires, we recommend setting the certificate lifetime to more than 1 hour.

{% alert level="warning" %}
If the client certificate lifetime has expired, kubelet will not be able to make requests to kube-apiserver and will not be able to renew certificates. In this case, the node will be marked as `NotReady` and recreated.
{% endalert %}

### Specifics of working with kubelet server certificates in Deckhouse Kubernetes Platform

Deckhouse Kubernetes Platform uses IP addresses for kubelet API requests. The kubelet configuration does not use the `tlsCertFile` and `tlsPrivateKeyFile` fields, but uses a dynamic certificate that kubelet generates itself. Also, the CIS benchmark `AVD-KCV-0088` and `AVD-KCV-0089` checks, which track whether the `--tls-cert-file` and `--tls-private-key-file` arguments were passed to kubelet, are disabled in the Deckhouse Kubernetes Platform (in the `operator-trivy` module).

{% offtopic title="Information about the logic of working with server certificates in Kubernetes" %}

Kubelet handles server certificates using the following logic:

* If `tlsCertFile` and `tlsPrivateKeyFile` are not empty, kubelet will use them as the default certificate and key.
  * When a client requests the kubelet API by specifying an IP address (e.g., `https://10.1.1.2:10250/`), the default private key (`tlsPrivateKeyFile`) will be used to establish a TLS connection. In this case, certificate rotation will not work.
  * When a client requests the kubelet API by specifying a host name (e.g., `https://k8s-node:10250/`), a dynamically generated private key from the `/var/lib/kubelet/pki/` directory will be used to establish a TLS connection. In this case, certificate rotation will work.

* If `tlsCertFile` and `tlsPrivateKeyFile` are empty, a dynamically generated private key from the `/var/lib/kubelet/pki/` directory will be used to establish the TLS connection. In this case, certificate rotation will work.
{% endofftopic %}

## How to manually update control plane component certificates?

There may be a situation when the cluster's master nodes are powered off for an extended period. During this time, the control plane component certificates may expire. After the nodes are powered back on, the certificates will not update automatically and must be renewed manually.

Control plane component certificates are updated using the `kubeadm` utility.
To update the certificates, do the following on each master node:

1. Find the `kubeadm` utility on the master node and create a symbolic link using the following command:

   ```shell
   ln -s $(find /var/lib/containerd -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

2. Update the certificates:

   ```shell
   kubeadm certs renew all
   ```
