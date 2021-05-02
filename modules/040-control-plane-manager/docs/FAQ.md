---
title: "Managing control plane: FAQ"
---

## How do I add a master node?

All you need to do is to attach the `node-role.kubernetes.io/master: ""` label to a cluster node (all other actions are performed automatically).

## How do I delete the master node?

* You can delete the master node as follows (if the deletion does not lead to the etcd cluster losing its quorum; in the cluster operating correctly, the 2 -> 1 migration is the only situation when etcd may lose its quorum):
    1. Drain the master node;
    2. If the deletion of the master is agreed, then you can delete the VM the usual way.
       If you can't delete the master right away (for example, it is used for backups or it is involved in the deployment process), then you have to stop the Container Runtime on the node:
       
       In the case of Docker:
       ```
       systemctl stop docker
       systemctl disable docker
       ```
       In the case of Containerd:
       ```
       systemctl stop containerd
       systemctl disable containerd
       ```
* If the deletion may result in etcd losing its quorum (the 2 -> 1 mirgation):
    1. Drain the master node;
    2. Stop kubelet on the node (without stopping the etcd container):
       ```
       systemctl stop kubelet
       systemctl stop bashible.timer
       systemctl stop bashible
       systemctl disable kubelet
       systemctl disable bashible.timer
       systemctl disable bashible
       ```
    3. Delete the Node object from Kubernetes;
    4. [Wait](#how-do-i-view-the-list-of-etcd-members) until the etcd member is automatically deleted;
    5. If the deletion of the master is agreed, then you can delete the VM the usual way;
       If you can't delete the master right away (for example, it is used for backups or it is involved in the deployment process), then you have to stop the Container Runtime on the node:
       
       In the case of Docker:
       ```
       systemctl stop docker
       systemctl disable docker
       ```
       In the case of Containerd:
       ```
       systemctl stop containerd
       systemctl disable containerd
       ```

## КHow do I dismiss the master status while keeping the node?

1. Remove the `node-role.kubernetes.io/master: ""` label and wait for the etcd member to be automatically deleted.
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

1. Exec to the etcd pod:
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

1. Stop (delete the `/etc/kubernetes/manifests/etcd.yaml` file) etcd on all nodes except one. This last node will serve as a starting point for the new multi-master cluster;
2. On the last node, specify the following command line parameter: `--force-new-cluster`;
3. After the new cluster is ready, remove the `--force-new-cluster` parameter.

**Caution!** This operation is unsafe and breaks the guarantees given by the consensus protocol. Note that it brings the cluster to the state that was saved on the node. Any pending entries will be lost.

## How do I enable event auditing?

Kubernetes [Auditing](https://kubernetes.io/docs/tasks/debug-application-cluster/debug-cluster/) can help you if you need to keep track of operations or troubleshoot the cluster. You can configure it by setting the appropriate [Audit Policy](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy).

Currently, the following fixed parameters of log rotation are in use:
```bash
--audit-log-maxage=7
--audit-log-maxbackup=10
--audit-log-maxsize=100
```
There must be some `log scraper` on master nodes  *(filebeat, promtail)* that will monitor the log directory:
```bash
/var/log/kube-audit/audit.log
```
Depending on the `Policy` settings and the number of requests to the **apiserver**, the amount of logs collected may be high. Thus, in some cases, logs can only be kept for less than 30 minutes. The maximum disk space for logs is limited to `1000 MB`.  Logs older than `7 days` will also be deleted.

### Cautionary note
> ⚠️ Note that the current implementation of this feature isn't safe and may lead to a temporary failure of the **control-plane**.
>
> The **apiserver** will not be able to start if there are unsupported options or a typo in the secret.

If **apiserver** is unable to start, you have to manually disable the `--audit-log-*` parameters in the `/etc/kubernetes/manifests/kube-apiserver.yaml` manifest and restart **apiserver** using the following command:
```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
```
After the restart, you will be able to fix the `Secret` or [delete it](#useful-commands).

### Enabling and configuring
The following parameter in the `d8-system/deckhouse` `ConfigMap` enables the audit:
```yaml
  controlPlaneManager: |
    apiserver:
      auditPolicyEnabled: true
```
The parameters are configured via the `kube-system/audit-policy` `Secret`. You need to put in it a `base64`-encoded `yaml` file:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: <base64>
```
### An example
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
- [The official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy);
- [Our Habr article](https://habr.com/ru/company/flant/blog/468679/);
- [The code of the generator script used in GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

### Useful commands
Create a `Secret` from the file:
```bash
kubectl -n kube-system create secret generic audit-policy --from-file=./audit-policy.yaml
```
Delete a `Secret` from the cluster:
```bash
kubectl -n kube-system delete secret audit-policy
```

## How do I speed up the restart of pods if the connection to the node has been lost?

By default, a node is marked as unavailable if it does not report its state for 40 seconds. After another 5 minutes, its pods will be rescheduled to other nodes. Thus, the overall application unavailability lasts approximately 6 minutes.

In specific cases, if an application cannot run in multiple instances, there is a way to lower its unavailability time: 

1. Reduce the period required for the `Node` to become `Unreachable` if the connection to it is lost by setting the `nodeMonitorGracePeriodSeconds` parameter.
1. Set a lower timeout for evicting pods on a failed node using the `failedNodePodEvictionTimeoutSeconds` parameter.

### An example:
```yaml
  controlPlaneManager: |
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```
In this case, if the connection to the node is lost, the applications will be restarted in about 1 minute.

### Cautionary note
Both these parameters directly impact the CPU and memory resources consumed by the `control plane`. By lowering timeouts, we force system components to send statuses more frequently and check the resource state more often. 

When deciding on the appropriate threshold values, consider resources consumed by the control nodes (graphs can help you with this). Note that the lower parameters are, the more resources you may need to allocate to these nodes.     
