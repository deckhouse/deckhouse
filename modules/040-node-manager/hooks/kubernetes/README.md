# kubernetes

Hooks whose subject is containerd or kubelet, components of the kubernetes team. node-manager hosts them because they look at nodes and NodeGroups.

- `containerd_v1_nodes_present` records whether any node still runs containerd v1 into the release requirements store, so an upgrade that drops containerd v1 waits until those nodes are migrated.
- `cntrd_v2_support` exports metrics for the nodes that cannot run containerd v2 or lack cgroup v2, from the labels bashible puts on nodes. The alerts about those nodes are built on these metrics.
- `kubelet_csr_approver` approves kubelet serving certificate requests after checking the requester, the names and the usages, so kubelet certificates rotate without a human.
