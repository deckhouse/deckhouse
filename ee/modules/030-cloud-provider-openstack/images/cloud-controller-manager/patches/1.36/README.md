## 001-ignore-static-nodes.patch

This patch is for our case when we want to have a static Nodes in the cluster, managed by openstack cloud provider.

## 002-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 003-skip-node-deletion.patch

When `SKIP_NODE_DELETION` is set, `InstanceExists` returns an error instead of `(false, nil)` if the OpenStack instance is not found.
This prevents `cloud-node-lifecycle` from deleting the Kubernetes Node while keeping shutdown taint handling enabled.

Planned Node deletion is not affected — those components remove Nodes directly through Kubernetes API, bypassing CCM:

- Nodes of type **CloudPermanent** — `dhctl` removes Nodes during converge.
- Nodes of type **CloudEphemeral (MCM engine)** — MCM removes the Node on Machine deletion.
- Nodes of type **CloudEphemeral (CAPI engine)** — CAPI controller removes the Node on Machine deletion.

Nodes that remain in the `NotReady` state for an extended period can be tracked using the `K8SNodeNotReady` alert — it fires when a node is `NotReady` for more than 10 minutes.

## 004-support-load-balancer-annotations.patch

Allows using `loadbalancer.openstack.deckhouse.io/load-balancer-id` and `loadbalancer.openstack.deckhouse.io/load-balancer-address` as input annotations for a predefined load balancer and floating IP.
Deckhouse annotations take precedence over deprecated or CCM-owned fields, while upstream `loadbalancer.openstack.org/load-balancer-id` and `loadbalancer.openstack.org/load-balancer-address` annotations remain CCM-owned output.
