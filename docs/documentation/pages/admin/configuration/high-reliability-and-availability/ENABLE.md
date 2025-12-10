---
title: Managing HA mode
permalink: en/admin/configuration/high-reliability-and-availability/enable.html
description: Managing HA mode
---

{% alert level="info" %}
Note that if the cluster has **more than one master node**, HA mode is **enabled automatically**.
This applies both when deploying a cluster with multiple master nodes from the start
and when increasing the number of master nodes from one to three.
{% endalert %}

## Enabling HA mode globally

You can enable HA mode globally for DKP in one of the following ways.

### Using ModuleConfig/global custom resource

1. Set the [`settings.highAvailability`](../../../reference/api/global.html#parameters-highavailability) parameter to `true` in `ModuleConfig/global`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: global
   spec:
     version: 2
     settings: 
       highAvailability: true
   ```

1. To ensure HA mode is enabled,
   you can, for example, check the number of `deckhouse` Pods in the `d8-system` namespace.
   To do that, run the following command:

   ```shell
   d8 k -n d8-system get po | grep deckhouse
   ```

   The number of `deckhouse` Pods in the output must be more than one:

   ```text
   deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
   deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
   deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
   ```

### Using Deckhouse web UI

If the [`console`](/modules/console/) module is enabled in the cluster,
open the Deckhouse web UI, navigate to **Deckhouse** — **Global settings** — **Global module settings**,
and switch the **HA mode** toggle to **Yes**.

## HA configuration with two master nodes and an arbiter node

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

1. Create a [backup of etcd](../backup/backup-and-restore.html#backing-up-etcd) and the `/etc/kubernetes` directory.
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

1. Add a node to the cluster that will be used as an arbiter node in a [way that is convenient](../platform-scaling/node/bare-metal-node.html#adding-nodes-to-a-bare-metal-cluster) for you.
1. [Ensure](/modules/control-plane-manager/faq.html#how-do-i-view-the-list-of-etcd-members) that the added arbiter node is in the list of etcd cluster members.
1. [Remove](../platform-scaling/control-plane/scaling-and-changing-master-nodes.html#removing-the-master-role-from-a-node-without-deleting-the-node-itself) one master node from the cluster.

## Enabling HA mode for individual components

Some DKP modules may have their own HA mode settings.
To enable HA mode in a specific module, set the `settings.highAvailability` parameter in its configuration.
The HA mode operation in individual modules is independent of the global HA mode.

List of modules supporting individual HA mode:

- [`deckhouse`](/modules/deckhouse/)
- [`openvpn`](/modules/openvpn/)
- [`istio`](/modules/istio/)
- [`dashboard`](/modules/dashboard/)
- [`multitenancy-manager`](/modules/multitenancy-manager/)
- [`user-authn`](/modules/user-authn/)
- [`ingress-nginx`](/modules/ingress-nginx/)
- [`prometheus-monitoring`](/modules/prometheus/)
- [`monitoring-kubernetes`](/modules/monitoring-kubernetes/)
- [`snapshot-controller`](/modules/snapshot-controller/)

To enable HA mode manually for a specific module,
add the `settings.highAvailability` parameter to its configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    highAvailability: true
```

To ensure HA mode is enabled, check the number of Pods for the target module.
For example, to verify the mode operation for the `deckhouse` module,
check the number of corresponding Pods in the `d8-system` namespace by running the following command:

```shell
d8 k -n d8-system get po | grep deckhouse
```

The number of `deckhouse` Pods in the output must be more than one:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```
