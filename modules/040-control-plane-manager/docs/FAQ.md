---
title: "Managing control plane: FAQ"
---

## How do I add a master node?
### Static or hybrid cluster
Adding a master node to a static or hybrid cluster has no difference from adding a regular node to a cluster. To do this, use the corresponding [instruction](../040-node-manager/faq.html#how-do-i-add-a-static-node-to-a-cluster). All the necessary actions to configure a cluster control plane components on the new master nodes are performed automatically. Wait until the master nodes appear in `Ready` status.

### Cloud cluster

> Make sure you have all the necessary quota limits, before adding nodes.

To add one or more master nodes to a cloud cluster, follow these steps:
1. Determine the Deckhouse version and edition used in the cluster by running the following command on the master node or a host with configured kubectl access to the cluster:
   ```shell
   kubectl -n d8-system get deployment deckhouse \
   -o jsonpath='version-{.metadata.annotations.core\.deckhouse\.io\/version}, edition-{.metadata.annotations.core\.deckhouse\.io\/edition}' \
   | tr '[:upper:]' '[:lower:]'
   ```
1. Run the corresponding version and edition of the Deckhouse installer:
   ```shell
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
   registry.deckhouse.io/deckhouse/<DECKHOUSE_EDITION>/install:<DECKHOUSE_VERSION> bash
   ```
   For example, if the Deckhouse version in the cluster is `v1.28.0` and the Deckhouse edition is `ee`, the command to run the installer will be:
   ```shell
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ee/install:v1.28.0 bash
   ```
   > Change the container registry address if necessary (e.g, if you use an internal container registry).

1. Run the following command inside the installer container (use the `--ssh-bastion-*` parameters if using a bastion host):
   ```shell
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
   --ssh-host <SSH_HOST>
   ```
1. Specify the required number of master node replicas in the `masterNodeGroup.replicas` field and save changes.
1. Start scaling process by running the following command (specify the appropriate cluster access parameters, as in the previous step):
   ```shell
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <SSH_HOST>
   ```
1. Answer `Yes` to the question `Do you want to CHANGE objects state in the cloud?`.

All the other actions are performed automatically. Wait until the master nodes appears in Ready status.

## How do I delete the master node?

1. Check if the deletion lead to the etcd cluster losing its quorum:
   * If the deletion does not lead to the etcd cluster losing its quorum:
     * If a virtual machine with a master node can be deleted (there are no other necessary services on it), then you can delete the virtual machine in the usual way.
     * If you can't delete the master right away (for example, it is used for backups or it is involved in the deployment process), then you have to stop the Container Runtime on the node:
       In the case of Docker:
       ```shell
       systemctl stop docker
       systemctl disable docker
       ```
       In the case of Containerd:
       ```shell
       systemctl stop containerd
       systemctl disable containerd
       kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
       ```

   * If the deletion may result in etcd losing its quorum (the 2 -> 1 mirgation), stop kubelet on the node (without stopping the etcd container):

     ```shell
     systemctl stop kubelet
     systemctl stop bashible.timer
     systemctl stop bashible
     systemctl disable kubelet
     systemctl disable bashible.timer
     systemctl disable bashible
     ```

2. Delete the Node object from Kubernetes.
3. [Wait](#how-do-i-view-the-list-of-etcd-members) until the etcd member is automatically deleted.

## How do I dismiss the master role while keeping the node?

1. Remove the `node.deckhouse.io/group: master` and `node-role.kubernetes.io/master: ""` labels, then wait for the etcd member to be automatically deleted.
2. Exec to the node and run the following commands:
   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd
   ```

## How do I view the list of etcd members?

1. Exec to the etcd Pod:
   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) sh
   ```
2. Execute the command:
   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list
   ```

## What if something went wrong?

The control-plane-manager saves backups to `/etc/kubernetes/deckhouse/backup`. They can be useful in diagnosing the issue.

## What if the etcd cluster fails?

1. Stop (delete the `/etc/kubernetes/manifests/etcd.yaml` file) etcd on all nodes except one. This last node will serve as a starting point for the new multi-master cluster.
2. On the last node, edit etcd manifest `/etc/kubernetes/manifests/etcd.yaml` and add the parameter `--force-new-cluster` to `spec.containers.command`.
3. After the new cluster is ready, remove the `--force-new-cluster` parameter.

> **Caution!** This operation is unsafe and breaks the guarantees given by the consensus protocol. Note that it brings the cluster to the state that was saved on the node. Any pending entries will be lost.

## How do I configure additional audit policies?

1. Enable [the following flag](configuration.html#parameters-apiserver-auditpolicyenabled) in the `d8-system/deckhouse` `ConfigMap`:
   ```yaml
   controlPlaneManager: |
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
   - [The official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy).
   - [The code of the generator script used in GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

   Create a Secret from the file:
   ```bash
   kubectl -n kube-system create secret generic audit-policy --from-file=./audit-policy.yaml
   ```

### How to omit Deckhouse built-in policy rules?

Set `apiserver.basicAuditPolicyEnabled` to `false`.

An example:

```yaml
controlPlaneManager: |
  apiserver:
    auditPolicyEnabled: true
    basicAuditPolicyEnabled: false
```

### How stream audit log to stdout instead of files?

Set `apiserver.auditLog.output` to `stdout`.

An example:

```yaml
controlPlaneManager: |
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
> **Note** that the current implementation of this feature isn't safe and may lead to a temporary failure of the control plane.
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

### An example:
```yaml
controlPlaneManager: |
  nodeMonitorGracePeriodSeconds: 10
  failedNodePodEvictionTimeoutSeconds: 50
```
In this case, if the connection to the node is lost, the applications will be restarted in about 1 minute.

### Cautionary note
Both these parameters directly impact the CPU and memory resources consumed by the control plane. By lowering timeouts, we force system components to send statuses more frequently and check the resource state more often.

When deciding on the appropriate threshold values, consider resources consumed by the control nodes (graphs can help you with this). Note that the lower parameters are, the more resources you may need to allocate to these nodes.
