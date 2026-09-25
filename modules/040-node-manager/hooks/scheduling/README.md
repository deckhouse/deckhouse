# scheduling

Hooks that serve cluster-autoscaler, standby nodes, NodeGroup priorities and fencing. Their owners are the owners of `templates/cluster-autoscaler` and `templates/fencing-agent`. The hooks live in node-manager because their input is NodeGroup.

- `cluster_autoscaler_deployment_requirements` decides whether cluster-autoscaler has to run and builds the list of groups it may scale, separately for CAPI and for MCM. Without a group with a min and max range the autoscaler is not deployed.
- `set_ng_priorities` turns `cloudInstances.priority` of the NodeGroups into the priority-expander configuration of cluster-autoscaler, so groups with a higher priority scale up first.
- `discover_standby_ng` computes, for every NodeGroup with standby, how many placeholder pods to run and how much CPU and memory to reserve, and reports the current number of standby pods into the NodeGroup status.
- `fencing_controller` watches the Lease of every node with fencing enabled. When a node stops renewing its Lease, the hook deletes the node's pods and the Node object, so the workloads move to healthy nodes.
- `nodegroup_status.go` holds the NodeGroup status patch helper used by `discover_standby_ng`.
