---
title: "Managing nodes: FAQ"
search: add a node to the cluster, set up a GPU-enabled node, ephemeral nodes
---

## How do I automatically add a static node to a cluster?

To add a new node to a static cluster, you need to:

- Create a `NodeGroup` with the necessary parameters (`nodeType` can be `Static` or `CloudStatic`) or use an existing one. Let's, for example, create a [`NodeGroup` called `worker`](usage.html#an-example-of-the-static-nodegroup-configuration).
- Get the script for installing and configuring the node: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."bootstrap.sh"' -r`
- Before configuring Kubernetes on the node, make sure that you have performed all the necessary actions for the node to work correctly in the cluster:
  - Added all the necessary mount points (nfs, ceph,...) to `/etc/fstab`;
  - Installed the suitable `ceph-common` version on the node as well as other packages;
  - Configured the network in the cluster;
- Connect to the new node over SSH and run the following command using the data from the secret: `echo <base64> | base64 -d | bash`

## How to put a node under the node-manager's control?

To make a node controllable by `node-manager`, perform the following steps:

- Create a `NodeGroup` with the necessary parameters (`nodeType` can be `Static` or `CloudStatic`) or use an existing one. Let's, for example, create a [`NodeGroup` called `worker`](usage.html#an-example-of-the-static-nodegroup-configuration).
- Get the script for installing and configuring the node: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."adopt.sh"' -r`
- Connect to the new node over SSH and run the following command using the data from the secret: `echo <base64> | base64 -d | bash`

## How do I change the node-group of a static node?

To switch an existing static node to another node-group, you need to change its group label:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<group_name>
```

The changes will not be applied instantly. One of the deckhouse hooks is responsible for updating the state of NodeGroup objects. It subscribes to node changes.

## How do I take a node out of the node-manager's control?

- Stop the bashible service and timer: `systemctl stop bashible.timer bashible.service`
- Delete bashible scripts: `rm -rf /var/lib/bashible`
- Remove annotations and labels from the node:

```shell
kubectl annotate node <node_name> node.deckhouse.io/configuration-checksum- update.node.deckhouse.io/waiting-for-approval- update.node.deckhouse.io/disruption-approved- update.node.deckhouse.io/disruption-required- update.node.deckhouse.io/approved- update.node.deckhouse.io/draining- update.node.deckhouse.io/drained-
kubectl label node <node_name> node.deckhouse.io/group-
```

## How to clean up a node for adding to the cluster?

This is only needed if you have to move a static node from one cluster to another. Be aware that these operations remove local storage data. If you just need to change NodeGroup you have to follow [this instruction](#how-do-i-change-the-node-group-of-a-static-node).

1. Stop all the services:
   ```shell
   systemctl stop kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl stop bashible.service bashible.timer
   systemctl stop kubelet.service
   systemctl stop docker
   ```
2. Unmount all mounted partitions:
   ```shell
   for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done
   ```
3. Delete all directories and files:
   ```shell
   rm -rf /var/lib/bashible
   rm -rf /etc/kubernetes
   rm -rf /var/lib/kubelet
   rm -rf /var/lib/docker
   rm -rf /etc/cni
   rm -rf /var/lib/cni
   rm -rf /var/lib/etcd
   rm -rf /etc/systemd/system/kubernetes-api-proxy*
   rm -rf /etc/systemd/system/bashible*
   rm -rf /etc/systemd/system/sysctl-tuner*
   rm -rf /etc/systemd/system/kubelet*
   ```
4. Delete all interfaces:
   ```shell
   ifconfig cni0 down
   ifconfig flannel.1 down
   ifconfig docker0 down
   ip link delete cni0
   ip link delete flannel.1
   ```
5. Cleanup systemd:
   ```shell
   systemctl daemon-reload
   systemctl reset-failed
   ```
6. Start Docker:
   ```shell
   systemctl start docker
   ```
7. [Run](#how-do-i-automatically-add-a-static-node-to-a-cluster) the `bootstrap.sh` script.
8. Turn on all the services:
   ```shell
   systemctl start kubelet.service
   systemctl start kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl start bashible.service bashible.timer
   ```

## How do I know if something went wrong?

The `node-manager` module creates the `bashible` service on each node. You can browse its logs using the following command: `journalctl -fu bashible`.

## How do I know what is running on a node while it is being created?

You can analyze `cloud-init` to find out what's happening on a node during the bootstrapping process:

- Find the node that is currently bootstrapping: `kubectl -n d8-cloud-instance-manager get machine | grep Pending`
- To show details about a specific `machine`, enter: `kubectl -n d8-cloud-instance-manager describe machine kube-2-worker-01f438cf-757f758c4b-r2nx2`
  You will see the following information:
  ```shell
  Status:
    Bootstrap Status:
      Description:   Use 'nc 192.168.199.115 8000' to get bootstrap logs.
      Tcp Endpoint:  192.168.199.115
  ```

- Run the `nc 192.168.199.115 8000`command to see `cloud-init` logs and determine the cause of the problem on the node.

The logs of the initial node configuration are located at `/var/log/cloud-init-output.log`.

## How do I configure a GPU-enabled node?

If you have a GPU-enabled node and want to configure Docker to work with the `node-manager`, you must configure this node according to the [documentation](https://github.com/NVIDIA/k8s-device-plugin#quick-start).

Create a `NodeGroup` with the following parameters:

```shell
docker:
  manage: false
operatingSystem:
  manageKernel: false
```

Then put the node under the control of `node-manager`.

## NodeGroup parameters and their result

| The NodeGroup parameter       | Disruption update    | Node provisioning | Kubelet restart |
| ----------------------------- | -------------------- | ----------------- | --------------- |
| operatingSystem.manageKernel  | + (true) / - (false) | -                 | -               |
| kubelet.maxPods               | -                    | -                 | +               |
| kubelet.rootDir               | -                    | -                 | +               |
| docker.maxConcurrentDownloads | +                    | -                 | +               |
| docker.manage                 | + (true) / - (false) | -                 | -               |
| nodeTemplate                  | -                    | -                 | -               |
| chaos                         | -                    | -                 | -               |
| kubernetesVersion             | -                    | -                 | +               |
| static                        | -                    | -                 | +               |
| disruptions                   | -                    | -                 | -               |
| cloudInstances.classReference | -                    | +                 | -               |

Refer to the description of the [NodeGroup](cr.html#nodegroup) custom resource for more information about the parameters.

Changing the `instancePrefix` parameter in the Deckhouse configuration won't result in a `RollingUpdate`. Deckhouse will create new `MachineDeployment`s and delete the old ones.

## How do I redeploy ephemeral machines in the cloud with a new configuration?

If the Deckhouse configuration is changed (both in the node-manager module and in any of the cloud providers), the VMs will not be redeployed. The redeployment is performed only in response to changing `InstanceClass` or `NodeGroup` objects.

To force the redeployment of all Machines, you need to add/modify the `manual-rollout-id` annotation to the `NodeGroup`: `kubectl annotate NodeGroup name_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## How do I allocate nodes to specific loads?

> ⛔ Note that you cannot use the `deckhouse.io` domain in `labels` and `taints` keys of the `NodeGroup`. It is reserved for **Deckhouse** components. Please, use the `dedicated` or `dedicated.client.com` keys.

There are two ways to solve this problem:

- You can set labels to `NodeGroup`'s `spec.nodeTemplate.labels`, to use them in the `Pod`'s [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters. In this case, you select nodes that the scheduler will use for running the target application.
- You cat set taints to `NodeGroup`'s `spec.nodeTemplate.taints` and then remove them via the `Pod`'s [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter. In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

> ℹ Deckhouse tolerates the `dedicated` by default, so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.️
> To use custom keys for `taints` (e.g., `dedicated.client.com`), you must add the key's value to the `global.modules.placement.customTolerationKeys` field of the `d8-system/deckhouse` ConfigMap. This way, deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.

## How to allocate nodes to system components?

### Frontend

For **Ingress** controllers, use the `NodeGroup` with the following configuration:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/frontend: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

### System components

`NodeGroup` for components of Deckhouse subsystems will look as follows:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/system: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
```

## How do I speed up node provisioning on the cloud when scaling applications horizontally?

The most efficient way is to have some extra nodes "ready". In this case, you can run new application replicas on them almost instantaneously. The obvious disadvantage of this approach is the additional maintenance costs related to these nodes.

Here is how you should configure the target `NodeGroup`:

1. Specify the number of "ready" nodes (or a percentage of the maximum number of nodes in the group) using the `cloudInstances.standby` paramter.
1. If there are additional service components (not maintained by Deckhouse, such as `filebeat` DaemonSet) for these nodes, you need to specify their combined resource consumption via the `standbyHolder.notHeldResources` parameter.
1. This feature requires that at least one group node is already running in the cluster. In other words, there must be either a single replica of the application, or the `cloudInstances.minPerZone` parameter must be set to `1`.

An example:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    notHeldResources:
      cpu: 300m
      memory: 2Gi
```

## How do I disable machine-controller-manager in the case of potentially cluster-damaging changes?

> ⛔ **_Caution!!!_** Use this switch only if you know what you are doing and clearly understand the consequences!

Set the `mcmEmergencyBrake` parameter to true::

```yaml
mcmEmergencyBrake: true
```

## How do I restore the master node if kubelet cannot load the control plane components?

Such a situation may occur if images of the `control plane` components on the master were deleted in a cluster that has a single master node (e.g., the directory `/var/lib/docker` (`/var/lib/containerd`) was deleted if docker (container) is used). In this case, `kubelet` cannot pull images of the `control plane` components when restarted since the master node lacks authorization parameters required for accessing `registry.deckhouse.io`.

Here is how you can restore the master node:

### Docker

Execute the following command to restore the master node in any cluster running under Deckhouse:

```
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r 'del(.auths."registry.deckhouse.io".username, .auths."registry.deckhouse.io".password)'
```

Copy the output of the command and add it to the `/root/.docker/config.json` file on the corrupted master.
Next, you need to pull images of `control plane` components to the corrupted master:

```
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  docker pull $image
done
```

You need to restart `kubelet` after pulling the images.
Please, pay attention that you must delete the changes made to the `/root/.docker/config.json` file after restoring the master node!

### Containerd

Execute the following command to restore the master node in any cluster running under Deckhouse:

```
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Copy the command's output and use it for setting the AUTH variable on the corrupted master.
Next, you need to pull images of `control plane` components to the corrupted master:

```
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

You need to restart `kubelet` after pulling the images.
