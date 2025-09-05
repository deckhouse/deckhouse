---
title: "Master nodes"
permalink: en/virtualization-platform/documentation/admin/platform-management/control-plane-settings/masters.html
---

## Adding a master node

> It is important to have an odd number of master nodes to ensure quorum.

Adding a master node to a cluster is no different from adding a regular node. Check for the existence
of a NodeGroup with the control-plane role (usually this is a NodeGroup named master) and follow the [adding a node](../node-management/adding-node.html#adding-a-node-to-a-cluster) instructions.
All necessary actions to configure the cluster control plane components on the new node will be performed automatically.

Before adding the next node, wait for the `Ready` status for all master nodes:

```shell
d8 k get no -l node-role.kubernetes.io/control-plane=
NAME STATUS ROLES AGE VERSION
master-0 Ready control-plane,master 276d v1.28.15
master-1 Ready control-plane,master 247d v1.28.15
master-2 Ready control-plane,master 247d v1.28.15
```

## Removing the master node role while keeping the node in the cluster

1. Make a [backup of etcd](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html#%D1%80%D0%B5%D0%B7%D0%B5%D1%80%D0%B2%D0%BD%D0%BE%D0%B5-%D0%BA%D0%BE%D0%BF%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-etcd) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (for example, to a local machine).
1. Make sure there are no alerts in the cluster that could prevent the master nodes from updating.
   A list of all alerts can be viewed using the command:

   ```shell
   d8 k get clusteralerts
   ```

1. Make sure the Deckhouse queue is empty.
   To view the status of all Deckhouse job queues, run the following command:

   ```shell
   d8 p queue list
   ```

1. Unlabel the node `node.deckhouse.io/group: master` and `node-role.kubernetes.io/control-plane: ""`.
1. Make sure the node is gone from the etcd cluster node list:

   ```bash
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Remove control plane component settings on the node:

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Make sure the number of nodes in NodeGroup `master` has decreased

   If there were 3 nodes, there should be 2:

   ```shell
   d8 k get ng master
   NAME TYPE READY NODES UPTODATE INSTANCES DESIRED MIN MAX STANDBY STATUS AGE SYNCED
   master Static 2 2 2 280d True
   ```
