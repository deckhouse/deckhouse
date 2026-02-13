# UpdateApprovalReconciler

Controller that manages node update approval workflow.

Replaces Go hook `hooks/update_approval.go` from `040-node-manager` module.

---

## Table of Contents

- [Overview](#overview)
- [Watches](#watches)
- [Reconcile Logic](#reconcile-logic)
- [Update Workflow](#update-workflow)
- [Concurrency Calculation](#concurrency-calculation)

---

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  UpdateApprovalReconciler                                                    │
│                                                                             │
│  Input (from cache):                    Output:                             │
│  ├─ NodeGroup                           └─ Node annotations                 │
│  ├─ Node (with update annotations)          ├─ approved                     │
│  └─ Secret (configuration-checksums)        ├─ waiting-for-approval         │
│                                             ├─ disruption-required          │
│                                             ├─ disruption-approved          │
│                                             ├─ draining                      │
│                                             └─ drained                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Watches

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  WATCHES                                                                    │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  For(&v1.NodeGroup{})                           [PRIMARY]            │   │
│  │                                                                     │   │
│  │  On NodeGroup change → Reconcile(nodeGroup.Name)                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Watches(&corev1.Node{})                        [SECONDARY]          │   │
│  │                                                                     │   │
│  │  Predicate: only Nodes with label node.deckhouse.io/group           │   │
│  │  MapFunc: extract nodeGroup name from label                         │   │
│  │  On Node change → Reconcile(nodeGroupName)                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Watches(&corev1.Secret{})                      [SECONDARY]          │   │
│  │                                                                     │   │
│  │  Predicate: only d8-cloud-instance-manager/configuration-checksums  │   │
│  │  MapFunc: return all NodeGroups                                     │   │
│  │  On Secret change → Reconcile(all NodeGroups)                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Reconcile Logic

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Reconcile(ctx, Request{Name: "worker"})                                    │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│  STEP 1: Get NodeGroup and configuration checksums                          │
│  ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│    ng := r.Client.Get(NodeGroup "worker")                                   │
│    checksums := r.Client.Get(Secret "configuration-checksums")              │
│    ngChecksum := checksums["worker"]                                        │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│  STEP 2: Get nodes and build node info                                      │
│  ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│    nodes := r.Client.List(Nodes, MatchingLabels{"node.deckhouse.io/group": "worker"})│
│                                                                             │
│    for each node:                                                           │
│      nodeInfo = {                                                           │
│        Name, NodeGroup, ConfigurationChecksum,                              │
│        IsReady, IsApproved, IsDisruptionApproved,                           │
│        IsWaitingForApproval, IsDisruptionRequired,                          │
│        IsUnschedulable, IsDraining, IsDrained, IsRollingUpdate              │
│      }                                                                      │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│  STEP 3: processUpdatedNodes — mark nodes as UpToDate                       │
│  ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│    For each node where:                                                     │
│      • IsApproved = true                                                    │
│      • ConfigurationChecksum == ngChecksum                                  │
│      • IsReady = true                                                       │
│                                                                             │
│    Action: Remove all update annotations                                    │
│      ├─ update.node.deckhouse.io/approved = null                            │
│      ├─ update.node.deckhouse.io/waiting-for-approval = null                │
│      ├─ update.node.deckhouse.io/disruption-required = null                 │
│      ├─ update.node.deckhouse.io/disruption-approved = null                 │
│      └─ update.node.deckhouse.io/drained = null                             │
│                                                                             │
│    If node.IsDrained: also set spec.unschedulable = null                    │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│  STEP 4: approveDisruptions — handle disruptive updates                     │
│  ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│    For each node where:                                                     │
│      • IsApproved = true                                                    │
│      • IsDisruptionRequired = true (or IsRollingUpdate = true)              │
│      • NOT IsDraining, NOT IsDisruptionApproved                             │
│                                                                             │
│    Check ApprovalMode:                                                      │
│      ├─ "Manual" → skip (user must approve manually)                        │
│      ├─ "Automatic" → check if current time is in allowed window            │
│      └─ "RollingUpdate" → check if current time is in allowed window        │
│                                                                             │
│    Actions based on mode:                                                   │
│      ├─ RollingUpdate: Delete Instance resource                             │
│      ├─ No drain needed OR already drained: Set disruption-approved         │
│      └─ Needs drain: Set draining annotation                                │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│  STEP 5: approveUpdates — approve waiting nodes                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│    Calculate concurrency:                                                   │
│      concurrency = ng.Spec.Update.MaxConcurrent (default: 1)                │
│      • If percentage: totalNodes * percent / 100                            │
│                                                                             │
│    Count currentUpdates (nodes with IsApproved = true)                      │
│                                                                             │
│    If currentUpdates >= concurrency → skip                                  │
│    If no waiting nodes → skip                                               │
│                                                                             │
│    countToApprove = concurrency - currentUpdates                            │
│                                                                             │
│    Approval priority:                                                       │
│      1. If all nodes ready (or desired <= ready for CloudEphemeral):        │
│         → approve waiting nodes                                             │
│      2. Otherwise: approve not-ready waiting nodes first                    │
│                                                                             │
│    Action: For each approved node:                                          │
│      ├─ update.node.deckhouse.io/approved = ""                              │
│      └─ update.node.deckhouse.io/waiting-for-approval = null                │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Update Workflow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  NODE UPDATE STATE MACHINE                                                  │
│                                                                             │
│  ┌──────────────┐                                                          │
│  │   UpToDate   │ ◄─────────────────────────────────────────────────────┐  │
│  │              │                                                       │  │
│  │ checksum ==  │                                                       │  │
│  │ ngChecksum   │                                                       │  │
│  └──────┬───────┘                                                       │  │
│         │                                                               │  │
│         │ bashible detects config change                                │  │
│         ▼                                                               │  │
│  ┌──────────────┐                                                       │  │
│  │  WaitingFor  │                                                       │  │
│  │   Approval   │                                                       │  │
│  │              │                                                       │  │
│  │ annotation:  │                                                       │  │
│  │ waiting-for- │                                                       │  │
│  │ approval     │                                                       │  │
│  └──────┬───────┘                                                       │  │
│         │                                                               │  │
│         │ controller approves (respecting concurrency)                  │  │
│         ▼                                                               │  │
│  ┌──────────────┐                                                       │  │
│  │   Approved   │                                                       │  │
│  │              │                                                       │  │
│  │ annotation:  │                                                       │  │
│  │ approved     │                                                       │  │
│  └──────┬───────┘                                                       │  │
│         │                                                               │  │
│         ├─────────────────────────────────────────┐                     │  │
│         │ non-disruptive update                   │ disruptive update   │  │
│         │                                         ▼                     │  │
│         │                                  ┌──────────────┐             │  │
│         │                                  │  Disruption  │             │  │
│         │                                  │   Required   │             │  │
│         │                                  │              │             │  │
│         │                                  │ annotation:  │             │  │
│         │                                  │ disruption-  │             │  │
│         │                                  │ required     │             │  │
│         │                                  └──────┬───────┘             │  │
│         │                                         │                     │  │
│         │              ┌──────────────────────────┼──────────────┐      │  │
│         │              │                          │              │      │  │
│         │              │ Manual mode              │ Automatic    │      │  │
│         │              │ (user approves)          │ mode         │      │  │
│         │              ▼                          │              │      │  │
│         │       ┌──────────────┐                  │              │      │  │
│         │       │   Waiting    │                  │              │      │  │
│         │       │  For Manual  │                  │              │      │  │
│         │       │  Approval    │                  │              │      │  │
│         │       └──────┬───────┘                  │              │      │  │
│         │              │                          │              │      │  │
│         │              │ user sets                │              │      │  │
│         │              │ disruption-approved      │              │      │  │
│         │              ▼                          ▼              │      │  │
│         │       ┌─────────────────────────────────────────┐      │      │  │
│         │       │                                         │      │      │  │
│         │       │           DrainBeforeApproval?          │      │      │  │
│         │       │                                         │      │      │  │
│         │       └────────────────┬────────────────────────┘      │      │  │
│         │                        │                               │      │  │
│         │         ┌──────────────┴──────────────┐                │      │  │
│         │         │                             │                │      │  │
│         │         │ yes                         │ no             │      │  │
│         │         ▼                             │                │      │  │
│         │  ┌──────────────┐                     │                │      │  │
│         │  │   Draining   │                     │                │      │  │
│         │  │              │                     │                │      │  │
│         │  │ annotation:  │                     │                │      │  │
│         │  │ draining     │                     │                │      │  │
│         │  └──────┬───────┘                     │                │      │  │
│         │         │                             │                │      │  │
│         │         │ bashible drains node        │                │      │  │
│         │         ▼                             │                │      │  │
│         │  ┌──────────────┐                     │                │      │  │
│         │  │   Drained    │                     │                │      │  │
│         │  │              │                     │                │      │  │
│         │  │ annotation:  │                     │                │      │  │
│         │  │ drained      │                     │                │      │  │
│         │  └──────┬───────┘                     │                │      │  │
│         │         │                             │                │      │  │
│         │         └─────────────┬───────────────┘                │      │  │
│         │                       ▼                                │      │  │
│         │                ┌──────────────┐                        │      │  │
│         │                │  Disruption  │                        │      │  │
│         │                │   Approved   │                        │      │  │
│         │                │              │                        │      │  │
│         │                │ annotation:  │                        │      │  │
│         │                │ disruption-  │                        │      │  │
│         │                │ approved     │                        │      │  │
│         │                └──────┬───────┘                        │      │  │
│         │                       │                                │      │  │
│         │                       │ bashible performs disruption   │      │  │
│         │                       │ (reboot, etc.)                 │      │  │
│         │                       │                                │      │  │
│         └───────────────────────┴────────────────────────────────┘      │  │
│                                 │                                       │  │
│                                 │ bashible updates checksum             │  │
│                                 │ node becomes Ready                    │  │
│                                 │                                       │  │
│                                 └───────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Concurrency Calculation

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  calculateConcurrency(maxConcurrent, totalNodes)                            │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                                                                     │   │
│  │  maxConcurrent = nil                 → return 1                     │   │
│  │                                                                     │   │
│  │  maxConcurrent = 3 (int)             → return 3                     │   │
│  │                                                                     │   │
│  │  maxConcurrent = "5" (string)        → return 5                     │   │
│  │                                                                     │   │
│  │  maxConcurrent = "25%" (percentage)  → return totalNodes * 25 / 100 │   │
│  │                                         (minimum 1)                 │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  Examples (10 nodes in NodeGroup):                                          │
│                                                                             │
│  │ maxConcurrent │ Result │                                                │
│  │───────────────│────────│                                                │
│  │ nil           │ 1      │                                                │
│  │ 1             │ 1      │                                                │
│  │ 3             │ 3      │                                                │
│  │ "25%"         │ 2      │ (10 * 25 / 100 = 2)                            │
│  │ "50%"         │ 5      │ (10 * 50 / 100 = 5)                            │
│  │ "5%"          │ 1      │ (10 * 5 / 100 = 0 → 1 minimum)                 │
│  │ "100%"        │ 10     │                                                │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Drain Skip Conditions

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  needDrainNode(node, nodeGroup) returns false when:                         │
│                                                                             │
│  1. Single control-plane node                                               │
│     nodeGroup.Name == "master" && nodeGroup.Status.Nodes == 1               │
│     → Can't drain: deckhouse webhook will evict, reboot is safe             │
│                                                                             │
│  2. Node running deckhouse with no other ready nodes                        │
│     node.Name == DECKHOUSE_NODE_NAME && nodeGroup.Status.Ready < 2          │
│     → Can't drain: deckhouse will not run after drain                       │
│                                                                             │
│  3. DrainBeforeApproval is explicitly false                                 │
│     nodeGroup.Spec.Disruptions.Automatic.DrainBeforeApproval == false       │
│     → User opted out of draining                                            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```
