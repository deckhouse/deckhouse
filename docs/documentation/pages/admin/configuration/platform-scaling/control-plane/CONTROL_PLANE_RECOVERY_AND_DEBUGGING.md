---
title: "Control plane recovery and debugging"
permalink: en/admin/configuration/platform-scaling/control-plane/control-plane-recovery-and-debugging.html
---

## Recovery from failures

During its operation DKP automatically creates backups of configuration and data that may be useful in case of problems. These backups are saved in the `/etc/kubernetes/deckhouse/backup` directory. If any issues or unexpected situations occur during operation, you can use these backups to restore the system to a previously healthy state.

## Restoring etcd cluster functionality

If the etcd cluster is not functioning and cannot be restored from a backup, you can attempt to recover it from scratch by following the steps below.

1. On all nodes that are part of your etcd cluster, **except one**, delete the `etcd.yaml` manifest located in `/etc/kubernetes/manifests/`. This will leave only one active node, from which the multi-master cluster state will be restored.
1. On the remaining node, open the `etcd.yaml` manifest and add the `--force-new-cluster` flag under `spec.containers.command`.
1. After the cluster is successfully restored, remove the `--force-new-cluster` flag.

{% alert level="danger" %}
This operation is destructive: it completely wipes the existing data and initializes a new cluster based on the state preserved on the remaining node. All pending records will be lost.
{% endalert %}

## Restoring a master node when kubelet fails to load control plane components

Such a situation may occur if images of the control plane components on the master were deleted in a cluster that has a single master node (e.g., the directory `/var/lib/containerd` was deleted). In this case, kubelet cannot pull images of the control plane components when restarted since the master node lacks authorization parameters required for accessing `registry.deckhouse.io`.

Below is an instruction on how you can restore the master node.

### containerd

1. Execute the following command to restore the master node in any cluster running under DKP:

   ```shell
   d8 k -n d8-system get secrets deckhouse-registry -o json |
   jq -r '.data.".dockerconfigjson"' | base64 -d |
   jq -r '.auths."registry.deckhouse.io".auth'
   ```

1. Copy the command's output and use it for setting the `AUTH` variable on the corrupted master.

1. Next, pull images of control plane components to the corrupted master:

   ```shell
   for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
     crictl pull --auth $AUTH $image
   done
   ```

1. Restart kubelet after pulling the images.

## etcd restore

### Viewing etcd cluster members

Below are the steps to view the list of nodes that are part of the etcd cluster:

1. Find the etcd pod:

   ```shell
   d8 k -n kube-system get pods -l component=etcd,tier=control-plane
   ```

   Typically, pod name has the `etcd-` prefix.

1. Run the following command on any available etcd Pod (assuming it is running in the `kube-system` namespace):

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
     etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
   ```

   This command uses substitution: `$(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1)`.
   It automatically inserts the name of the first Pod matching the specified labels.  

### Restoring the etcd cluster in case of complete unavailability

1. Stop all etcd nodes except one by deleting the `etcd.yaml` manifest on the others.
1. On the remaining node, add the `--force-new-cluster` option to the etcd startup command.
1. After the cluster is restored, remove this option.

{% alert level="danger" %}
Be careful: these actions completely erase the previous data and form a new etcd cluster.
{% endalert %}

### Recovering etcd after panic: unexpected removal of unknown remote peer error

In some cases, manual restoration via `etcdutl snapshot restore` can help:

1. Save a local snapshot from `/var/lib/etcd/member/snap/db`.
1. Use `etcdutl` with the `--force-new-cluster` option to restore.
1. Completely wipe the `/var/lib/etcd` directory and place the restored snapshot there.
1. Remove any "stuck" etcd/kube-apiserver containers and restart the node.

### Actions to take when etcd database exceeds quota-backend-bytes limit

When the database volume of etcd reaches the limit set by the `quota-backend-bytes` parameter, it switches to "read-only" mode. This means that the etcd database stops accepting new entries but remains available for reading data. You can tell that you are facing a similar situation by executing the command:

```shell
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
--cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
--endpoints https://127.0.0.1:2379/ endpoint status -w table --cluster
```

If you see a message like `alarm:NOSPACE` in the `ERRORS` field, you need to take the following steps:

1. Make change to `/etc/kubernetes/manifests/etcd.yaml` — find the line with `--quota-backend-bytes` and increase the value by multiplying the specified number by two. If there is no such line — add, for example: `- --quota-backend-bytes=8589934592` — this sets the limit to 8 GB.
1. Disarm the active alarm that occurred due to reaching the limit. To do this, execute the command:

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Change the [`maxDbSize`](/modules/control-plane-manager/configuration.html#parameters-etcd-maxdbsize) parameter in the `control-plane-manager` settings to match the value specified in the manifest.

## etcd defragmentation

{% alert level="warning" %}
Before defragmenting, [back up etcd](../../backup/backup-and-restore.html#creating-backups-with-deckhouse-cli).
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

## High availability

If any component of the control plane becomes unavailable, the cluster temporarily maintains its current state but cannot process new events. For example:

- If `kube-controller-manager` fails, Deployment scaling will stop working.
- If `kube-apiserver` is unavailable, no requests can be made to the Kubernetes API, although existing applications will continue to function.

However, prolonged unavailability of control plane components disrupts the processing of new objects, handling of node failures, and other operations. Over time, this can lead to cluster degradation and impact user applications.

To mitigate these risks, the control plane should be scaled to a high-availability configuration — a minimum of three nodes. This is especially critical for etcd, which requires a quorum to elect a leader. The quorum works on a majority basis (N/2 + 1) of the total number of nodes.

Example:

| Cluster size | Quorum (majority) | Max fault tolerance |
|--------------|-------------------|----------------------|
| 1            | 1                 | 0                    |
| 3            | 2                 | 1                    |
| 5            | 3                 | 2                    |
| 7            | 4                 | 3                    |
| 9            | 5                 | 4                    |

{% alert level="info" %}
An even number of nodes does not improve fault tolerance but increases replication overhead.
{% endalert %}

In most cases, three etcd nodes are sufficient. Use five if high availability is critical. More than seven is rarely necessary and not recommended due to high resource consumption.

After new control plane nodes are added:

- The label `node-role.kubernetes.io/control-plane=""` is applied.
- A DaemonSet launches control plane pods on the new nodes.
- DKP creates or updates files in `/etc/kubernetes`: manifests, configuration files, certificates, etc.
- All DKP modules that support high availability will enable it automatically, unless the global setting [`highAvailability`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-highavailability) is manually overridden.

Control plane node removal is performed in reverse:

- Labels `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, and `node.deckhouse.io/group` are removed.
- DKP removes its pods from these nodes.
- etcd members on the nodes are automatically deleted.
- If the number of nodes drops from two to one, etcd may enter `readonly` mode. In this case, you must start etcd with the `--force-new-cluster` flag, which should be removed after a successful startup.
