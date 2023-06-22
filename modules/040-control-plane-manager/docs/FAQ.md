---
title: "Managing control plane: FAQ"
---

<div id="how-do-i-add-a-master-node"></div>

## How do I add a master node to a static or hybrid cluster?

> It is important to have an odd number of masters to ensure a quorum.

Adding a master node to a static or hybrid cluster has no difference from adding a regular node to a cluster. To do this, use the corresponding [instruction](../040-node-manager/faq.html#how-do-i-add-a-static-node-to-a-cluster). All the necessary actions to configure a cluster control plane components on the new master nodes are performed automatically. Wait until the master nodes appear in `Ready` status.

## How do I add a master nodes to a cloud cluster (single-master to a multi-master)?

> Before adding nodes, ensure you have the required quotas in the cloud provider.
>
> It is important to have an odd number of masters to ensure a quorum.

1. Run the appropriate edition and version of the Deckhouse installer container **on the local machine** (change the container registry address if necessary):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command and specify the required number of replicas using the `masterNodeGroup.replicas` parameter:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

1. **In the installer container**, run the following command to start scaling:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Wait until the required number of master nodes are `Ready` and all `control-plane-manager` instances are up and running:

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

<div id="how-do-i-delete-the-master-node"></div>

## How do I reduce the number of master nodes in a cloud cluster (multi-master to single-master)?

1. Run the appropriate edition and version of the Deckhouse installer container **on the local machine** (change the container registry address if necessary):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. Run the following command **in the installer container** and set `masterNodeGroup.replicas` to `1`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Remove the following labels from the master nodes to be deleted:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Use the following command to remove labels:

   ```bash
   kubectl label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Make sure that the master nodes to be deleted are no longer listed as etcd cluster members:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

1. Drain the nodes being deleted:

   ```bash
   kubectl drain <MASTER-NODE-N-NAME> --ignore-daemonsets --delete-emptydir-data
   ```

1. Shut down the virtual machines corresponding to the nodes to be deleted, remove the instances of those nodes from the cloud and the disks connected to them (`kubernetes-data-master-<N>`).

1. In the cluster, delete the Pods running on the nodes being deleted:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=<MASTER-NODE-N-NAME> --force
   ```

1. In the cluster, delete the Nore objects associated with the nodes being deleted:

   ```bash
   kubectl delete node <MASTER-NODE-N-NAME>
   ```

1. **In the installer container**, run the following command to start scaling:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

## How do I dismiss the master role while keeping the node?

1. Remove the `node.deckhouse.io/group: master` and `node-role.kubernetes.io/control-plane: ""` labels.
1. Make sure that the master node to be deleted is no longer listed as a member of the etcd cluster:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
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

1. Run the appropriate edition and version of the Deckhouse installer container **on the local machine** (change the container registry address if necessary):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to check the state before working:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The response should tell you that Terraform does not want to change anything.

1. **In the installer container**, run the following command and specify the required OS image using the `masterNodeGroup.instanceClass` parameter (specify the addresses of all master nodes using the `-ssh-host` parameter):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

Repeat the steps below for **each master node one by one**, starting with the node with the highest number (suffix 2) and ending with the node with the lowest number (suffix 0).

1. Select the master node to update (enter its name):

   ```bash
   NODE="<MASTER-NODE-N-NAME>"
   ```

1. Run the following command to remove the `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, and `node.deckhouse.io/group` labels from the node:

   ```bash
   kubectl label node ${NODE} \
     node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Make sure that the node is no longer listed as an etcd cluster member:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name') -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

1. Drain the node:

   ```bash
   kubectl drain ${NODE} --ignore-daemonsets --delete-emptydir-data
   ```

1. Shut down the virtual machine associated with the node, remove the node instance from the cloud and the disks connected to it (`kubernetes-data`).

1. In the cluster, delete the Pods remaining on the node being deleted:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=${NODE} --force
   ```

1. In the cluster, delete the Node object for the node being deleted:

   ```bash
   kubectl delete node ${NODE}
   ```

1. **In the installer container**, run the following command to create the updated node:

    You should read carefully what converge is going to do when it asks for approval.

    If converge requests approval for another master node, it should be skipped by saying "no".

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **On the newly created node**, check the systemd-unit log for the `bashible.service`. Wait until the node configuration is complete (you will see a message `nothing to do` in the log):

   ```bash
   journalctl -fu bashible.service
   ```

1. Make sure the node is listed as an etcd cluster member:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name') -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

1. Make sure `control-plane-manager` is running on the node:

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=${NODE}
   ```

1. Proceed to update the next node (repeat the steps above).

## How do I switch to a different OS image in a single-master cluster?

1. Convert your single-master cluster to a multi-master one, as described in [the guide on adding master nodes to a cluster](#how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master).

   > In addition to increasing the number of replicas, you can also specify the image with the required OS version using the `masterNode.instanceClass` parameter.

1. Update the master nodes following the [instructions](#how-do-i-switch-to-a-different-os-image-in-a-multi-master-cluster).
1. Convert your multi-master cluster to a single-master one according to [the guide on excluding master nodes from the cluster](#how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster-multi-master-to-single-master).

## How do I view the list of etcd members?

### Option 1

1. Exec to the etcd Pod:

   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) sh
   ```

2. Execute the command:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
   --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

Warning! The last parameter in the output table shows etcd member is in [**learner**](https://etcd.io/docs/v3.5/learning/design-learner/) state, is not in *leader* state.

### Option 2

Use the `etcdctl endpoint status` command. The fith parameter in the output table will be `true` for the leader.

Example:

```shell
$ ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \  
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table

https://10.2.1.101:2379, ade526d28b1f92f7, 3.5.3, 177 MB, false, false, 42007, 406566258, 406566258,
https://10.2.1.102:2379, d282ac2ce600c1ce, 3.5.3, 182 MB, true, false, 42007, 406566258, 406566258,
```

## What if something went wrong?

The control-plane-manager saves backups to `/etc/kubernetes/deckhouse/backup`. They can be useful in diagnosing the issue.

## What if the etcd cluster fails?

1. Stop (delete the `/etc/kubernetes/manifests/etcd.yaml` file) etcd on all nodes except one. This last node will serve as a starting point for the new multi-master cluster.
2. On the last node, edit etcd manifest `/etc/kubernetes/manifests/etcd.yaml` and add the parameter `--force-new-cluster` to `spec.containers.command`.
3. After the new cluster is ready, remove the `--force-new-cluster` parameter.

> **Caution!** This operation is unsafe and breaks the guarantees given by the consensus protocol. Note that it brings the cluster to the state that was saved on the node. Any pending entries will be lost.

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

2. Create the `kube-system/audit-policy` Secret containing a `base64`-encoded `yaml` file:

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

   You can find the detailed information about configuring the `audit-policy.yaml` file at the following links:
   - [The official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy).
   - [The code of the generator script used in GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

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

### How stream audit log to stdout instead of files?

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

There must be some `log scraper` on master nodes  *([log-shipper](../460-log-shipper/cr.html#clusterloggingconfig), promtail, filebeat)* that will monitor the log file:

```bash
/var/log/kube-audit/audit.log
```

The following fixed parameters of log rotation are in use:
- The maximum disk space is limited to `1000 Mb`.
- Logs older than `7 days` will be deleted.

Depending on the `Policy` settings and the number of requests to the **apiserver**, the amount of logs collected may be high. Thus, in some cases, logs can only be kept for less than 30 minutes.

### Cautionary note

> ***Note!** The current implementation of this feature isn't safe and may lead to a temporary failure of the control plane.
>
> The apiserver will not be able to start if there are unsupported options or a typo in the Secret.

If apiserver is unable to start, you have to manually disable the `--audit-log-*` parameters in the `/etc/kubernetes/manifests/kube-apiserver.yaml` manifest and restart apiserver using the following command:

```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
# Or (depending on your CRI).
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

After the restart, you will be able to fix the Secret or delete it:

```bash
kubectl -n kube-system delete secret audit-policy
```

## How do I speed up the restart of Pods if the connection to the node has been lost?

By default, a node is marked as unavailable if it does not report its state for 40 seconds. After another 5 minutes, its Pods will be rescheduled to other nodes. Thus, the overall application unavailability lasts approximately 6 minutes.

In specific cases, if an application cannot run in multiple instances, there is a way to lower its unavailability time:

1. Reduce the period required for the node to become `Unreachable` if the connection to it is lost by setting the `nodeMonitorGracePeriodSeconds` parameter.
1. Set a lower timeout for evicting Pods on a failed node using the `failedNodePodEvictionTimeoutSeconds` parameter.

### An example

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

### Cautionary note

Both these parameters directly impact the CPU and memory resources consumed by the control plane. By lowering timeouts, we force system components to send statuses more frequently and check the resource state more often.

When deciding on the appropriate threshold values, consider resources consumed by the control nodes (graphs can help you with this). Note that the lower parameters are, the more resources you may need to allocate to these nodes.

## How do make etcd backup?

Login into any control-plane node with `root` user and use next script:

```bash
#!/usr/bin/env bash

for pod in $(kubectl get pod -n kube-system -l component=etcd,tier=control-plane -o name); do
  if kubectl -n kube-system exec "$pod" -- sh -c "ETCDCTL_API=3 /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /tmp/${pod##*/}.snapshot" && \
  kubectl -n kube-system exec "$pod" -- gzip -c /tmp/${pod##*/}.snapshot | zcat > "${pod##*/}.snapshot" && \
  kubectl -n kube-system exec "$pod" -- sh -c "cd /tmp && sha256sum ${pod##*/}.snapshot" | sha256sum -c && \
  kubectl -n kube-system exec "$pod" -- rm "/tmp/${pod##*/}.snapshot"; then
    mv "${pod##*/}.snapshot" etcd-backup.snapshot
    break
  fi
done
```

In the current directory etcd snapshot file `etcd-backup.snapshot` will be created from one of an etcd cluster members.
From this file, you can restore the previous etcd cluster state in the future.

Also, we recommend making a backup of the `/etc/kubernetes` directory, which contains:
- manifests and configurations of [control-plane components](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components);
- [Kubernetes cluster PKI](https://kubernetes.io/docs/setup/best-practices/certificates/).
This directory will help to quickly restore a cluster in case of complete loss of control-plane nodes without creating a new cluster
and without rejoin the remaining nodes into the new cluster.

We recommend encrypting etcd snapshot backups as well as backup of the directory `/etc/kubernetes/` and saving them outside the Deckhouse cluster.
You can use one of third-party files backup tools, for example: [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/), etc.

You can see [here](https://github.com/deckhouse/deckhouse/blob/main/modules/040-control-plane-manager/docs/internal/ETCD_RECOVERY.md) for learn about etcd disaster recovery procedures from snapshots.

## How is the Node selected on which the Pod will run?

The selection of the node on which the Pod will run is determined by the Kubernetes scheduler component.
The scheduler performs two phases, namely Filtering and Scoring, which are designed to distribute the Pods to the Nodes effectively.
While there are additional phases, such as pre-filtering, post-filtering, and so on, they enhance flexibility and optimization and can be reduced to these two phases.

### General structure of the Kubernetes scheduler

The Scheduler comprises plugins that function in either or both phases.

Example of plugins:
- **ImageLocality** — favors nodes that already have the container images that the Pod runs. Phase: **Scoring**.
- **TaintToleration** — implements [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Phases: **Filtering, Scoring**.
- **NodePorts** - checks if the node has free ports for the requested Pod ports. Phase: **Filtering**.

You can see the full list of plugins at the [Kubernetes documentation](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

### Working logic

The filtering phase (**Filtering**) is executed first. At this stage, `filter` plugins select nodes that satisfy filtering conditions such as `taints`, `nodePorts`, `nodeName`, `unschedulable`, among others.
If the nodes belong to different zones, they are selected alternately to avoid placing all Pods in a single zone.

For instance, if there are two zones with nodes as follows:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

The nodes will be selected in the following order:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4.
```

Additionally, only a portion of prerequisite nodes are selected to optimize the process and avoid checking all selected nodes later.
By default, the percentage of nodes selected is linear. For clusters with less than or equal to 50 nodes, 100% of nodes are selected; for clusters with 100 nodes, 50% are selected; and for clusters with 5000 nodes, 10% are selected. The minimum value is 5% for clusters with more than 5000 nodes. Therefore, even if all conditions are met, a node may not be included in the list of possible nodes when using the default settings. However, this setting can be overridden by setting the `percentageOfNodesToScore` flag to the scheduler.

The Scoring phase follows once the nodes that meet the conditions are selected. Each plugin evaluates the filtered node list and assigns a score to each node based on available resources, Pod capacity, affinity, volume provisioning, and other factors. The scores from different plugins are summed up, and the node with the highest score is chosen at the end of this phase. If there is more than one node with the same score, it is selected randomly.

Finally, the selected node is bound to the Pod.

#### Documentation

- [General description of the scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/)
- [Plugin system](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins)
- [Node Filtering Details](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/)
- [Scheduler source code](https://github.com/kubernetes/kubernetes/tree/master/cmd/kube-scheduler)
