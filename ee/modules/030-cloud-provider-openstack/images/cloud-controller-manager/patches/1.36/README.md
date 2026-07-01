## 001-ignore-static-nodes.patch

This patch is for our case when we want to have a static Nodes in the cluster, managed by openstack cloud provider.

## 002-skip-node-deletion.patch

When `SKIP_NODE_DELETION` is set, `InstanceExists` returns an error instead of `(false, nil)` if the OpenStack instance is not found. This prevents `cloud-node-lifecycle` from deleting the Kubernetes Node while keeping shutdown taint handling enabled.
