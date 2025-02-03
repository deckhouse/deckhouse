---
title: "Working with etcd"
permalink: en/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html
---

## etcd backup

### Automatic backup

Deckhouse creates a CronJob `kube-system/d8-etcd-backup-*`, which is triggered at 00:00 UTC+0. The etcd data backup is saved to the archive `/var/lib/etcd/etcd-backup.tar.gz` on all master nodes.

### Manual backup using Deckhouse CLI

In Deckhouse v1.65 and higher clusters, etcd data backup can be created with a single `d8 backup etcd` command:

```bash
d8 backup etcd --kubeconfig $KUBECONFIG ./etcd.db
```

<!-- TODO what's in the etcd.db file? The etcdctl version explains what file is being created, but this one doesn't.
TODO where to run this, on each master node or not? -->

### Manual backup with etcdctl

{% alert level="warning" %}
Not recommended for use in Deckhouse 1.65 and higher.
{% endalert %}

On Deckhouse v1.64 and earlier, run the following script on any master node as `root`:

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

The `kube-backup.tar.gz` file will be created in the current directory with a snapshot of the etcd database of one of the etcd cluster nodes.
The resulting snapshot can be used to restore the state of the etcd cluster.

It is also recommended to backup the directory `/etc/kubernetes`, which contains:

- manifests and configuration of [control-plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components) components;
- [PKI of the Kubernetes cluster](https://kubernetes.io/docs/setup/best-practices/certificates/).

We recommend storing backup copies of the etcd cluster snapshots, as well as a backup of the directory `/etc/kubernetes/` in encrypted form outside the Deckhouse cluster.
For this, you can use third-party file backup tools, such as [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/), etc.

## Full cluster state recovery from etcd backup

The following are the steps for restoring a cluster to a previous state from a backup in case of complete data loss.

### Recovering a cluster with one master node

To correctly recover a cluster with one master node, follow these steps:

1. Download the [etcdctl](https://github.com/etcd-io/etcd/releases) utility to the server (it is desirable that its version is the same as the etcd version in the cluster).

    ```shell
    wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
    tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
    ```

    You can check the etcd version in your cluster by running the following command:

    ```shell
    d8 k -n kube-system exec -ti etcd-$(hostname) -- etcdctl version
    ```

1. Stop etcd.

    Etcd runs as a static pod, so it's enough to move the manifest file:

    ```shell
    mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
    ```

1. Backup the current etcd data.

    ```shell
    cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
    ```

1. Clean up the etcd directory.

    ```shell
    rm -rf /var/lib/etcd/member/
    ```

1. Place the etcd backup in `~/etcd-backup.snapshot`.

1. Restore the etcd database.

    ```shell
     ETCDCTL_API=3 etcdctl snapshot restore ~/etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ --data-dir=/var/lib/etcd
     ```

1. Start etcd.

    ```shell
    mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
    ```

### Recovering a multi-master cluster

To properly recover a multi-master cluster, follow these steps:

1. Explicitly enable High Availability (HA) mode using the global parameter [highAvailability](/products/virtualization-platform/reference/mc.html#global-parameters-highavailability). This is necessary, for example, to avoid losing one Prometheus replica and its PVC, since HA is disabled by default in single-master cluster mode.

1. Switch the cluster to single-master mode, according to the [instruction](#how-to-reduce-the-number-of-master-nodes-in-a-multi-master-cloud-cluster-to-single-master) for cloud clusters or manually remove static master nodes from the cluster.

1. On the remaining single master node, follow the steps to restore etcd from backup as described in the [guide](#restoring-a-single-master-cluster) for a single-master cluster.

1. When etcd is running will be restored, remove information about the master nodes already deleted in step 1 from the cluster using the following command (specify the node name):

    ```shell
    d8 k delete node <MASTER_NODE_I>
    ```

1. Restart all cluster nodes.

1. Wait for the tasks from the Deckhouse queue to complete:

    ```shell
    d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
    ```

1. Switch the cluster back to multi-master mode in accordance with [instruction](#how-to-add-master-nodes-in-a-cloud-cluster-single-master-to-multi-master) for cloud clusters or [instruction](/products/virtualization-platform/documentation/admin/platform-management/node-management/adding-node.html) for static or hybrid clusters.

## Restoring a Kubernetes object from an etcd backup

A short scenario for restoring individual objects from an etcd backup:

1. Get a backup of your data.

1. Start a temporary etcd instance.

1. Fill it with data from the backup.

1. Get descriptions of the required objects using the `etcdhelper` utility.

### Steps for restoring objects from an etcd backup

In the example:

- `etcd-snapshot.bin` is a file with a [backup](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html#%D1%80%D0%B5%D0%B7%D0%B5%D1%80%D0%B2%D0%BD%D0%BE%D0%B5-%D0%BA%D0%BE%D0%BF%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-etcd) of etcd data (snapshot);
- `infra-production` is the namespace in which you want to restore the objects.

1. Start a pod with a temporary etcd instance.

    It is desirable that the version of the etcd instance you are starting matches the version of etcd from which the backup was created. For simplicity, the instance is launched not locally, but in the cluster, since the cluster already has an etcd image.

    - Prepare the `etcd.pod.yaml` file with the pod manifest:

      ```shell
      cat <<EOF >etcd.pod.yaml
      apiVersion: v1
      kind: Pod
      metadata:
      name: etcdrestore
      namespace: default
      spec:
      containers:
      - command:
      - /bin/sh
      - -c
      - "sleep 96h"
      image: IMAGE
      imagePullPolicy: IfNotPresent
      name: etcd
      volumeMounts:
      - name: etcddir
      mountPath: /default.etcd
      volumes:
      - name: etcddir
      emptyDir: {}
      EOF
      ```

    - Set the current name of the etcd image:

      ```shell
      IMG=`kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[0].spec. containers[*].image}"`
      sed -i -e "s#IMAGE#$IMG#" etcd.pod.yaml
      ```

    - Create a pod:

      ```shell
      kubectl create -f etcd.pod.yaml
      ```

    - Copy `etcdhelper` and the etcd snapshot to the pod container.

      `etcdhelper` can be built from [source](https://github.com/openshift/origin/tree/master/tools/etcdhelper) or copied from a pre-built image (e.g. the `etcdhelper` image on [Docker Hub](https://hub.docker.com/r/webner/etcdhelper/tags)).

      Example:

      ```shell
      kubectl cp etcd-snapshot.bin default/etcdrestore:/tmp/etcd-snapshot.bin
      kubectl cp etcdhelper default/etcdrestore:/usr/bin/etcdhelper
      ```

    - In the container, set permissions to run `etcdhelper`, restore the data from the backup, and start etcd.

      Example:

      ```console
      ~ # kubectl -n default exec -it etcdrestore -- sh
      / # chmod +x /usr/bin/etcdhelper
      / # etcdctl snapshot restore /tmp/etcd-snapshot.bin
      / # etcd &
      ```

    - Get the descriptions of the desired cluster objects by filtering them with `grep`.

      Example:

      ```console
      ~ # kubectl -n default exec -it etcdrestore -- sh
      / # mkdir /tmp/restored_yaml
      / # cd /tmp/restored_yaml
      /tmp/restored_yaml # for o in `etcdhelper -endpoint 127.0.0.1:2379 ls /registry/ | grep infra-production` ; do etcdhelper -endpoint 127.0.0.1:2379 get $o > `echo $o | sed -e "s#/registry/##g;s#/#_#g"`.yaml ; done
      ```

      The `sed` replacement in the example allows object descriptions to be saved to files named like the etcd registry structure. For example: `/registry/deployments/infra-production/supercronic.yaml` → `deployments_infra-production_supercronic.yaml`.

1. Copy the received object descriptions from the pod to the master node using the command:

    ```shell
    d8 k cp default/etcdrestore:/tmp/restored_yaml restored_yaml
    ```

1. Remove information about the creation time, UID, status and other operational data from the received object descriptions, then restore the objects using the command:

    ```shell
    d8 k create -f restored_yaml/deployments_infra-production_supercronic.yaml
    ```

1. A pod with a temporary etcd instance can be deleted using the command:

    ```shell
    d8 k delete -f etcd.pod.yaml
    ```

## How to get a list of etcd cluster nodes (option 1)

Use the `etcdctl member list` command.

Example:

```shell
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
--cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
--endpoints https://127.0.0.1:2379/ member list -w table
```

**Warning.** The last parameter in the output table shows that the etcd cluster node is in the [learner](https://etcd.io/docs/v3.5/learning/design-learner/) state, not the leader state.

## How to get a list of cluster nodes etcd (option 2)

Use the `etcdctl endpoint status` command. For this command, after the `--endpoints` flag, you need to substitute the address of each control-plane node.

The `true` value in the fifth column of the output indicates the leader.

An example of a script that automatically transfers all addresses of control-plane nodes:

```shell
MASTER_NODE_IPS=($(d8 k get nodes -l \
node-role.kubernetes.io/control-plane="" \
-o 'custom-columns=IP:.status.addresses[?(@.type=="InternalIP")].address' \
--no-headers))
unset ENDPOINTS_STRING
for master_node_ip in ${MASTER_NODE_IPS[@]}
do ENDPOINTS_STRING+="--endpoints https://${master_node_ip}:2379 "
done
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod \
-l component=etcd,tier=control-plane -o name | head -n1)\
-- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
--key /etc/kubernetes/pki/etcd/ca.key \
$(echo -n $ENDPOINTS_STRING) endpoint status -w table
```

## Rebuilding etcd cluster

A rebuild may be required if the etcd cluster has collapsed, or when migrating from a multi-master cluster to a single-master cluster.

1. Select the node from which to start restoring the etcd cluster. In case of migrating to a single-master cluster, this is the node where etcd should remain.
1. Stop etcd on all other nodes. To do this, delete the file `/etc/kubernetes/manifests/etcd.yaml`.
1. On the remaining node, in the manifest `/etc/kubernetes/manifests/etcd.yaml`, add the `--force-new-cluster` argument to the `spec.containers.command` field.

1. After the cluster has been successfully started, remove the `--force-new-cluster` parameter.

{% alert level="danger" %}
The operation is destructive, it completely destroys the consensus and starts the etcd cluster from the state that was saved on the selected node. Any pending entries will be lost.
{% endalert %}

## Eliminating infinite restart

This option may be needed if starting with the `--force-new-cluster` argument does not restore etcd. This can happen during an unsuccessful converge of master nodes, when the new master node was created with the old etcd disk, changed its address from the local network, and there are no other master nodes. It is worth using this method if the etcd container is in an infinite restart, and its log contains the error: `panic: unexpected removal of unknown remote peer`.

1. Install the [etcdutl](https://github.com/etcd-io/etcd/releases) utility.
1. From the current local snapshot of the etcd database (`/var/lib/etcd/member/snap/db`), create a new snapshot:

    ```shell
    ./etcdutl snapshot restore /var/lib/etcd/member/snap/db --name <HOSTNAME> \
    --initial-cluster=HOSTNAME=https://<ADDRESS>:2380 --initial-advertise-peer-urls=https://ADDRESS:2380 \
    --skip-hash-check=true --data-dir /var/lib/etcdtest
    ```

    where:

    - `<HOSTNAME>` is the name of the master node;
    - `<ADDRESS>` is the address of the master node.

1. Run commands to use the new snapshot:

    ```shell
    cp -r /var/lib/etcd /tmp/etcd-backup
    rm -rf /var/lib/etcd
    mv /var/lib/etcdtest /var/lib/etcd
    ```

1. Find the `etcd` and `kube-apiserver` containers:

    ```shell
    crictl ps -a --name "^etcd|^kube-apiserver"
    ```

1. Remove the found `etcd` and `kube-apiserver` containers:

    ```shell
    crictl rm <CONTAINER-ID>
    ```

1. Restart the master node.
