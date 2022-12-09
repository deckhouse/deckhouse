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

## How do I dismiss the master role while keeping the node?

1. Remove the `node.deckhouse.io/group: master` and `node-role.kubernetes.io/control-plane: ""` labels, then wait for the etcd member to be automatically deleted.
2. Exec to the node and run the following commands:

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

## How do I view the list of etcd members?

### Option 1

1. Exec to the etcd Pod:

   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) sh
   ```

2. Execute the command:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
   --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list
   ```

### Option 2

Use the `etcdctl endpoint status` command. The fith parameter in the output table will be `true` for the leader.

Example:

```shell
$ ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \  
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status
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
   - [The official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy).
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

### An example

```yaml
controlPlaneManager: |
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

## Как увеличить число master-узлов

1. Запустите контейнер установщика Deckhouse соответствующей редакции и версии (на локальной машине).

    ```bash
    DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
    DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}') \
    docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \                                                                                        
    registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
    ```

2. Измените число реплик для masterNodeGroup на требуемое.

   Пример:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
   --ssh-host <MASTER-NODE-0>
   ```

Фрагмент конфигурации:

   ```yaml
   masterNodeGroup:
     instanceClass:
       replicas: <требуемое число реплик>
   ```

3. Выполните converge.

   Пример:

    ```bash
    dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
    --ssh-user=<USERNAME> \
    --ssh-host <MASTER-NODE-0>
    ```

4. Убедитесь, что `control-plane-manager` функционирует.

    ```bash
    kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
    ```

## Как уменьшить число master-узлов

1. Запустите контейнер установщика Deckhouse соответствующей редакции и версии (на локальной машине).

    ```bash
    DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
    DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}') \
    docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \                                                                                        
    registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
    ```

2. Измените число реплик для masterNodeGroup на требуемое.

   Пример:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
   --ssh-host <MASTER-NODE-0>
   ```

   Фрагмент конфигурации:

   ```yaml
   masterNodeGroup:
     instanceClass:
       replicas: <требуемое число реплик>
   ```

3. Снимите следующие лейблы с удаляемых узлов:
  * node-role.kubernetes.io/control-plane
  * node-role.kubernetes.io/master
  * node.deckhouse.io/group

   Пример:

    ```bash
    kubectl label node <имя удаляемого узла> \
    node-role.kubernetes.io/control-plane- \
    node-role.kubernetes.io/master- \
    node.deckhouse.io/group-
    ```

4. Убедитесь, что узлы пропали из членов кластера etcd.

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

5. Выполните drain для удаляемых узлов.

   Пример:

   ```bash
   kubectl drain <имя удаляемого узла> --ignore-daemonsets --delete-emptydir-data
   ```

6. Выключите (poweroff) удаляемый узел и удалите инстансы соответствующих узлов из облака и подключенные к ним диски (kubernetes-data).

7. Удалите оставшиеся ресурсы с узлов.

   Пример:

    ```bash
    kubectl delete pods --all-namespaces --field-selector spec.nodeName=<имя удаляемого узла> --force
    ```

8. Удалите ресурсы узлов.

   Пример:

   ```bash
   kubectl delete node <имя удаляемого узла>
   ```

9. Выполните converge.

   Пример:

    ```bash
    dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
    --ssh-user=<USERNAME> \
    --ssh-host <MASTER-NODE-0>
    ```

   > При выполнении команды убедитесь, что планируется удалить именно требуемые ноды!

10. Убедитесь, что `control-plane-manager` функционирует.

    ```bash
    kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
    ```

## Как изменить образ ОС в multi-master кластере

1. Запустите контейнер установщика Deckhouse соответствующей редакции и версии (на локальной машине).

    ```bash
    DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
    DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}') \
    docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \                                                                                        
    registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
    ```

2. Измените образ ОС для masterNodeGroup.

   Пример:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
   --ssh-host <MASTER-NODE-0> --ssh-host <MASTER-NODE-1> --ssh-host <MASTER-NODE-2>
   ```

   Фрагмент конфигурации:

   ```yaml
   masterNodeGroup:
     instanceClass:
       template: <new image version>
   ```

> Следующие действия выполняйте поочередно на всех master-узлах, начиная с последнего (с суффиксом 2) и заканчивая первым (с суффиксом 0):

1. Выберите master-узел для обновления.

   ```bash
   NODE="<MASTER-NODE-X>"
   ```

2. Снимите следующие лейблы с удаляемых узлов:
* node-role.kubernetes.io/control-plane
* node-role.kubernetes.io/master
* node.deckhouse.io/group

  Пример:
    ```bash
    kubectl label node <имя удаляемого узла> \
    node-role.kubernetes.io/control-plane- \
    node-role.kubernetes.io/master- \
    node.deckhouse.io/group-
```

3. Убедитесь что узел пропал из членов etcd кластера.

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

4. Выполните `drain` для узла `<MASTER-NODE-X>`.

   ```bash
   kubectl drain ${NODE} --ignore-daemonsets --delete-emptydir-data
   ```

5. Выключите (poweroff) `<MASTER-NODE-X>`, удалите инстанс узла из облака и подключенные к нему диски (kubernetes-data).

6. Удалите Pod'ы, оставшиеся на удаляемом узле `<MASTER-NODE-X>`.

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=${NODE} --force
   ```

7. Удалите объект Node `<MASTER-NODE-X>`.

   ```bash
   kubectl delete node ${NODE}
   ```

8. Выполните converge.

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
   --ssh-user=<USERNAME> \
   --ssh-host <MASTER-NODE-0> \
   --ssh-host <MASTER-NODE-1> \
   --ssh-host <MASTER-NODE-2>
   ```

   > При выполнении команды убедитесь, что планируется удалить именно требуемые узлы!

9. В логах `bashible.service` на вновь созданном узле `<MASTER-NODE-X>` должно быть сообщение “nothing to do”.

   ```bash
   journalctl -fu bashible.service
   ```

10. Убедитесь, что узел появился в членах кластера etcd.

    ```bash
    kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
    "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table"
    ```

11. Убедитесь, что `control-plane-manager` функционирует на узле `<MASTER-NODE-X>`.

    ```bash
    kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager --field-selector spec.nodeName=${NODE}
    ```

## Как изменить образ ОС в single-master кластере

1. Преобразуйте single-master в multi-master кластер в соответствии с [инструкцией](#как-увеличить-число-master-узлов)

> Внимание! Помимо увеличения числа реплик, также рекомендуется сразу установить требуемую версию ОС в masterNode.instanceClass.template: <new image version>

2. Обновите master-узлы в соответствии с [инструкцией](#как-изменить-образ-ОС-в-multi-master-кластере)
3. Преобразуйте multi-master в single-master кластер в соответствии с [инструкцией](#как-уменьшить-число-master-узлов)
