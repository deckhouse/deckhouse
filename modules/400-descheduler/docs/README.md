---
title: "The descheduler module"
---

The module runs a [descheduler](https://github.com/kubernetes-sigs/descheduler) with [strategies](#strategies) defined in a `Descheduler` custom resource.

descheduler every 15 minutes evicts Pods that satisfy strategies enabled in the `Descheduler` custom resource. This leads to forced run the scheduling process for evicted Pods.

## Nuances of descheduler operation

* descheduler do not take into account pods in `d8-*` and `kube-system` namespaces;
* Pods with local storage enabled are never evicts;
* Pods that are associated with a DaemonSet are never evicts;
* Pods with [priorityClassName](../001-priority-class/) set to `system-cluster-critical` or `system-node-critical` (*critical* Pods) are never evicts;
* descheduler takes into account the priority class when evicting Pods from a high-loaded node (check out the [priority-class](../001-priority-class/) module);
* The Best effort Pods are evicted before Burstable and Guaranteed ones;
* descheduler takes into account the [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/): the Pod will not be evicted if descheduling violates the PDB;
* descheduler takes into account node fitting. If no nodes available to start evicted pod, pod is not evicted.

To limit pods set, `labelSelector` param is used.
To set up node fit list, `nodeSelector` param is used. `nodeSelector` has the same syntax as `labelSelector`. Node fit list always includes nodes with labels `node-role.kubernetes.io/control-plane: ""` and `node-group.de  `

## Strategies

You can enable, disable, and configure a strategy in the [`Descheduler` custom resource](cr.html).

### HighNodeUtilization

This strategy finds nodes that are under utilized and evicts Pods in the hope that these Pods will be scheduled compactly into fewer nodes. This strategy must be used with the scheduler strategy `MostRequestedPriority`.

### LowNodeUtilization

This strategy finds underutilized or overutilized nodes using cpu/memory/Pods (in %) thresholds and evicts Pods from overutilized nodes hoping that these Pods will be rescheduled on underutilized nodes. Note that this strategy takes into account Pod requests instead of actual resources consumed.
