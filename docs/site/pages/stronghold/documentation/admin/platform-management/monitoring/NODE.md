---
title: "Node monitoring"
permalink: en/stronghold/documentation/admin/platform-management/monitoring/node.html
---

## Monitoring

For node groups (NodeGroup resource), DKP exports availability metrics for the group.

### What information does Prometheus collect?

All node group metrics have the prefix `d8_node_group_` in their name, and a label with the node group's name `node_group_name`.

The following metrics are collected for each node group:

- `d8_node_group_ready` — the number of nodes in the group that are in `Ready` status;
- `d8_node_group_nodes` — the total number of nodes in the group (in any status);
- `d8_node_group_instances` — the total number of instances in the group (in any status);
- `d8_node_group_desired` — the desired (target) number of `Machines` objects in the group;
- `d8_node_group_min` — the minimum number of instances in the group;
- `d8_node_group_max` — the maximum number of instances in the group;
- `d8_node_group_up_to_date` — the number of nodes in the group in up-to-date state;
- `d8_node_group_standby` — the number of standby nodes (see the [`standby`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) parameter in the group);
- `d8_node_group_has_errors` — one if there are any errors in the node group.
