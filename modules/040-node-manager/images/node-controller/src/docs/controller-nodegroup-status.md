# nodegroup-status

**Name:** `nodegroup-status`
**Primary resource:** `NodeGroup`
**Replaces hook:** `update_node_group_status.go`

## Purpose

Computes and updates the `status` subresource of each NodeGroup.
Aggregates information from Nodes, Machines, MachineDeployments, and Secrets
to produce node counts, conditions, cloud status, and machine failures.

## Watched Resources

| Resource | Trigger | MapFunc |
|----------|---------|---------|
| `NodeGroup` | Any change (primary) | — |
| `Node` | Label/status change | `node.deckhouse.io/group` → NodeGroup name |
| `Machine` (MCM) | Any change | OwnerRef → MachineDeployment → NodeGroup name |
| `MachineDeployment` (MCM) | Any change | Label → NodeGroup name |
| `Machine` (CAPI) | Any change | Same mapping |
| `MachineDeployment` (CAPI) | Any change | Same mapping |
| `Secret` (configuration-checksums) | Change | Enqueue all NodeGroups |
| `Secret` (d8-cloud-provider-discovery-data) | Change | Enqueue all NodeGroups |

## Reconciliation Logic

```
NodeGroup changed (or secondary resource triggers re-enqueue)
  │
  ├─ NodeGroup not found? → done
  │
  ├─ Compute node status (nodestatus.Service):
  │   ├─ List nodes with label node.deckhouse.io/group=<ng.Name>
  │   ├─ Count: total, ready, upToDate
  │   └─ Build per-node data for condition calculation
  │
  ├─ Compute cloud status (cloudstatus.Service) — for CloudEphemeral:
  │   ├─ Get MachineDeployments for this NodeGroup
  │   ├─ Get Machines and their status
  │   ├─ Calculate: desired, min, max, instances, failures
  │   ├─ Detect frozen MachineDeployments
  │   └─ Read zones from provider discovery secret
  │
  ├─ Calculate conditions (conditionscalc):
  │   ├─ Ready, Updating, WaitingForDisruptiveApproval, Error, Scaling
  │   └─ Based on: node states, machine states, errors
  │
  ├─ Build condition summary string
  │
  ├─ Compare new status with existing status
  │   └─ If different → Status().Patch() to API server
  │
  └─ Patch processed status annotation
```

## Status Fields Updated

| Field | Source |
|-------|--------|
| `status.nodes` | Count of nodes in NodeGroup |
| `status.ready` | Count of ready nodes |
| `status.upToDate` | Count of nodes matching configuration checksum |
| `status.desired` | From MachineDeployment replicas (CloudEphemeral only) |
| `status.min` / `status.max` | From MachineDeployment (CloudEphemeral only) |
| `status.instances` | Running machine instances (CloudEphemeral only) |
| `status.conditions` | Calculated conditions array |
| `status.conditionSummary` | Human-readable summary |
| `status.lastMachineFailures` | Recent machine creation failures |

## Sub-packages

| Package | Purpose |
|---------|---------|
| `cloud_status/` | MachineDeployment/Machine aggregation, zones, failures |
| `node_status/` | Node counting and readiness computation |
| `conditions/` | Condition summary + event creation |
| `conditionscalc/` | Condition calculation logic (Ready, Updating, etc.) |
| `processed_status/` | Processed status annotation patching + checksum |
| `nodegroupfilter/` | Spec filtering for processed status |
| `common/` | Shared constants, types, GVKs, predicates, mappers |

## Files

- `controller.go` — main reconciler, watches setup, orchestration
- `cloud_status/` — 6 files for cloud provider status
- `node_status/` — node counting
- `conditions/` — condition service
- `conditionscalc/` — condition calculation engine
- `processed_status/` — annotation management
- `common/` — shared types and helpers
