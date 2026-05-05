# Instance Controller

This controller owns the lifecycle view exposed by cluster-scoped
`deckhouse.io/v1alpha2` `Instance` objects.

An `Instance` can be backed by:

- CAPI `cluster.x-k8s.io/v1beta2` `Machine`;
- MCM `machine.sapcloud.io/v1alpha1` `Machine`;
- static or cloud-permanent `Node` without a CAPI machine annotation.

## Package Layout

```text
controller/instance/
├── controller.go
├── node/
│   └── node_service.go
├── instance/
│   ├── instance_service.go
│   ├── instance_reconciler_finalizer.go
│   ├── instance_reconciler_heartbeat.go
│   ├── instance_reconciler_source.go
│   ├── instance_reconciler_status.go
│   ├── instance_bashible_status.go
│   ├── instance_message.go
│   └── instance_source.go
└── common/
    ├── instance_helpers.go
    ├── instance_status.go
    ├── watch_helpers.go
    └── machine/
        ├── machine_types.go
        ├── machine_factory.go
        ├── machine_adapter_capi.go
        ├── machine_adapter_mcm.go
        └── machine_state_capi.go
```

`controller.go` keeps watches, source lookup, reconcile order, and
controller-runtime result handling.

`instance/` contains operations on existing `Instance` objects: finalization,
source existence checks, bashible heartbeat/status aggregation, and message
aggregation.

`common/machine/` hides CAPI/MCM differences behind one `Machine` adapter
interface.

## Watches

The controller reconciles `Instance` objects directly and also maps related
objects to an `Instance` with the same name:

```text
CAPI Machine  ─┐
MCM Machine   ├─ MapObjectNameToInstance ─► InstanceController.Reconcile(name)
static Node   ┘
```

Node watches are filtered by `StaticNodeEventPredicate`; only static or
cloud-permanent nodes without a CAPI machine annotation are relevant.

## Create Path

When the requested `Instance` does not exist, the controller creates it from the
first source found:

1. CAPI `Machine` in `d8-cloud-instance-manager`;
2. MCM `Machine` in `d8-cloud-instance-manager`;
3. static/cloud-permanent `Node`.

Machine-backed instances are created with `MachineRef` and, when known,
`NodeRef`. Static node instances are created with `NodeRef` and phase
`Running`.

## Existing Instance Pipeline

For an existing `Instance`, reconcile runs these steps in order:

```text
1. reconcileDeletion
2. EnsureInstanceFinalizer
3. reconcileMachineRef
4. ReconcileSourceExistence
5. ReconcileBashibleHeartbeat
6. ReconcileBashibleStatus
7. reconcileMachineStatus
```

The loop uses two step shapes:

- terminal steps return `(done bool, error)` and may stop the pipeline;
- non-terminal steps return `error` and are adapted with `nonTerminalStep`.

### Step Details

`reconcileDeletion`

Stops the pipeline when `DeletionTimestamp` is set. It asks the linked machine
to delete, waits until the machine is gone, then removes the instance finalizer.
If there is no `MachineRef`, finalization can remove the finalizer immediately.

`EnsureInstanceFinalizer`

Adds `node-manager.hooks.deckhouse.io/instance-controller` to active instances.

`reconcileMachineRef`

Self-heals an existing instance whose `MachineRef` is missing. It looks for a
CAPI or MCM machine with the same name and patches `Spec.MachineRef` if found.
This must run before source existence checks.

`ReconcileSourceExistence`

Garbage-collects orphaned instances. Source type is selected by
`getInstanceSource`: `MachineRef` has priority over `NodeRef`. If the selected
source is confirmed missing, the controller removes its finalizer without
machine-deletion side effects and deletes the instance.

`ReconcileBashibleHeartbeat`

Updates `BashibleReady` to `Unknown` when bashible stops refreshing the
condition:

```text
normal state                  elapsed >= 5m   -> HeartBeat
WaitingApproval=True          elapsed >= 20m  -> WaitingApproval
WaitingDisruptionApproval=True elapsed >= 20m -> WaitingDisruptionApproval
```

Explicit `BashibleReady=False` is preserved.

`ReconcileBashibleStatus`

Aggregates bashible-related conditions into `Status.BashibleStatus` and
`Status.Message`.

Status priority:

```text
WaitingDisruptionApproval=True                  -> WaitingApproval
WaitingApproval=True + UpdateApprovalTimeout    -> WaitingApproval
BashibleReady=True                              -> Ready
BashibleReady=False                             -> Error
otherwise                                      -> Unknown
```

Message priority:

```text
MachineReady problem
BashibleReady problem
WaitingDisruptionApproval=True
WaitingApproval=True
BashibleReady message
```

`reconcileMachineStatus`

Reads the linked CAPI/MCM machine and synchronizes `Instance.Status.Phase`,
`Status.MachineStatus`, and the `MachineReady` condition.

## Deletion

```text
Instance DeletionTimestamp set
        │
        ▼
get linked Machine from MachineRef
        │
        ├─ missing/no MachineRef ─► remove finalizer
        │
        └─ exists ─► machine.EnsureDeleted()
                       │
                       ├─ machine still present ─► keep finalizer
                       └─ machine gone ──────────► remove finalizer
```

The next reconcile is triggered by machine watch events or by the normal
periodic requeue.

## Machine Status Mapping

CAPI machine phases:

```text
Pending             -> Pending
Provisioning        -> Provisioning
Provisioned         -> Provisioned
Running             -> Running
Deleting/Deleted    -> Terminating
other               -> Unknown
```

MCM current status phase:

```text
Pending/Creating/Available       -> Pending
Running                          -> Running
Terminating                      -> Terminating
Unknown/Failed/CrashLoopBackOff  -> Unknown
other                            -> Unknown
```

MCM `LastOperation.State` drives `Status.MachineStatus`: successful running
machines become `Ready`, processing machines become `Progressing`, failed
machines become `Error`, and drain-blocked failures become `Blocked`.

## Status Field Owners

| Field | Field owner |
| --- | --- |
| `Status.Phase` | `node-controller-instancestatus` |
| `Status.MachineStatus` | `node-controller-instancestatus` |
| `Status.Conditions[MachineReady]` | `node-controller-instancestatus` |
| `Status.BashibleStatus` | `node-controller-instance-bashible-status` |
| `Status.Message` | `node-controller-instance-bashible-status` |
| `Status.Conditions[BashibleReady]` heartbeat | `node-controller-instance-bashible-heartbeat` |
| `Status.Conditions[BashibleReady]` actual | bashible |
