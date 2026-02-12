# Instance Controller

Single controller responsible for the full lifecycle of `Instance` objects
(`deckhouse.io/v1alpha2`, cluster-scoped).

## Package layout

```
controller/instance/
├── controller.go          ← single dynctrl.Reconciler, registered via register.RegisterController
├── capi/
│   └── capi_machine_service.go   ← reconcile logic: CAPI Machine → Instance spec/status
├── mcm/
│   └── mcm_machine_service.go    ← reconcile logic: MCM Machine → Instance spec/status
├── node/
│   └── node_service.go           ← reconcile logic: static Node → Instance create/delete
├── instance/
│   ├── instance_service.go       ← public API: ReconcileHeartbeat, ReconcileBashibleStatus, etc.
│   ├── instance_reconciler_finalizer.go   ← finalizer add/remove + linked machine deletion
│   ├── instance_reconciler_heartbeat.go   ← BashibleReady heartbeat timeout logic
│   ├── instance_reconciler_status.go      ← BashibleStatus + Message SSA patch
│   ├── instance_reconciler_source.go      ← orphan detection: delete when machine+node gone
│   ├── instance_bashible_status.go        ← bashibleStatusFromConditions
│   ├── instance_message.go                ← messageFromConditions (priority matchers)
│   └── instance_source.go                 ← getInstanceSource (machine / node / none)
└── common/
    ├── instance_helpers.go   ← EnsureInstanceExists, SetInstancePhase, RemoveInstanceControllerFinalizer
    ├── instance_status.go    ← SyncInstanceStatus, ApplyInstanceMachineStatus, ConditionEqualExceptLastTransitionTime
    ├── watch_helpers.go      ← MapObjectNameToInstance, StaticNodeEventPredicate, IsStaticNode
    └── machine/
        ├── machine_types.go        ← MachineFactory interface, Machine interface, MachineStatus, Status enum
        ├── machine_factory.go      ← NewMachineFactory, dispatch by type/apiVersion
        ├── machine_adapter_capi.go ← capiMachine adapter: GetStatus, calculatePhase, etc.
        ├── machine_adapter_mcm.go  ← mcmMachine adapter: GetStatus, calculateState, etc.
        ├── machine_state_capi.go   ← calculateCAPIState, buildMachineReadyCondition
        └── machine_state_capi_test.go
```

---

## Watch sources → reconcile trigger

```
  capiv1beta2.Machine (any change)
          │
          ▼  MapObjectNameToInstance
  ┌───────────────────────────────┐
  │                               │
  │    reconcile.Request          │
  │    { Name: machine.Name }     │
  │                               │
  └───────────────────────────────┘
          │
  mcmv1alpha1.Machine (typed watch, any change)
          │
          ▼  MapObjectNameToInstance
  ┌───────────────────────────────┐
  │    reconcile.Request          │
  │    { Name: machine.Name }     │
  └───────────────────────────────┘
          │
  corev1.Node (StaticNodeEventPredicate: label changes, static/cloud-permanent only)
          │
          ▼  MapObjectNameToInstance
  ┌───────────────────────────────┐
  │    reconcile.Request          │
  │    { Name: node.Name }        │
  └───────────────────────────────┘
          │
  deckhousev1alpha2.Instance (primary For, any change)
          │
          ▼  (direct)
  ┌───────────────────────────────┐
  │    reconcile.Request          │
  │    { Name: instance.Name }    │
  └───────────────────────────────┘
          │
          ▼
    InstanceController.Reconcile
```

---

## Reconcile pipeline

```
InstanceController.Reconcile(ctx, req)
        │
        ▼
 r.Client.Get(req.Name, &Instance)
        │
        ├── NotFound ────────────────────────────────────────────────────────┐
        │                                                                    │
        │  reconcileCreateFromSource                                         │
        │      │                                                             │
        │      ├─ capiService.EnsureInstanceFromMachine(name)                │
        │      │       │                                                     │
        │      │       ├─ CAPI Machine GET ──── NotFound                     │
        │      │       │                      found                          │
        │      │       └─ EnsureInstanceExists ← return                      │
        │      │                                                             │
        │      ├─ mcmService.EnsureInstanceFromMachine(name)                 │
        │      │       │                                                     │
        │      │       ├─ MCM Machine GET ───── NotFound                     │
        │      │       │                      found                          │
        │      │       └─ EnsureInstanceExists ← return                      │
        │      │                                                             │
        │      └─ node.ReconcileNode(name)                                   │
        │              │                                                     │
        │              ├─ Node GET ───────── NotFound → return               │
        │              │                   found                             │
        │              ├─ IsStaticNode?                                      │
        │              │            yes                                      │
        │              ├─ EnsureInstanceExists                               │
        │              └─ SetInstancePhase(Running) ← return                 │
        │                                                                    │
        └── Found ───────────────────────────────────────────────────────────┘
                │
                ▼
         ┌─────────────────────────────────────────────────────────────┐
         │  STEP 1  ReconcileHeartbeat                                  │
         │  If BashibleReady condition has not been updated for too     │
         │  long, sets it to Unknown with an appropriate reason:        │
         │    elapsed >= 10m                → HeartBeat                 │
         │    WaitingApproval + elapsed >= 20m → WaitingApproval        │
         │    WaitingDisruption + elapsed >= 20m → WaitingDisruption    │
         │  SSA-patch field owner:                                     │
         │    node-controller-instance-bashible-heartbeat              │
         └─────────────────────────────────────────────────────────────┘
                │
                ▼
         ┌─────────────────────────────────────────────────────────────┐
         │  STEP 2  ReconcileBashibleStatus                             │
         │  Derives Instance.Status.BashibleStatus from conditions:     │
         │    WaitingDisruptionApproval=True → WaitingApproval          │
         │    WaitingApproval=True + UpdateApprovalTimeout             │
         │      → WaitingApproval                                     │
         │    BashibleReady=True  → Ready                               │
         │    BashibleReady=False → Error                               │
         │    default             → Unknown                             │
         │  Also derives Instance.Status.Message from condition msgs.   │
         │  SSA-patch field owner:                                     │
         │    node-controller-instance-bashible-status                 │
         └─────────────────────────────────────────────────────────────┘
                │
                ▼
         ┌─────────────────────────────────────────────────────────────┐
         │  STEP 3  reconcileDeletion                                   │
         │  If DeletionTimestamp is set:                                │
         │    ReconcileFinalization:                                    │
         │      1. reconcileLinkedMachineDeletion:                      │
         │           get machine via machineFactory.NewMachineFromRef   │
         │           machine.EnsureDeleted(ctx, client)                 │
         │      2. if machine not gone yet → fastRequeue (1s)           │
         │      3. if machine gone → removeInstanceFinalizer → done     │
         │  → returns done=true, stops pipeline                         │
         └─────────────────────────────────────────────────────────────┘
                │  (only if NOT deleting)
                ▼
         ┌─────────────────────────────────────────────────────────────┐
         │  STEP 4  ReconcileEnsureFinalizer                            │
         │  Adds "node-manager.hooks.deckhouse.io/instance-controller"  │
         │  finalizer if not present. MergeFrom patch.                  │
         └─────────────────────────────────────────────────────────────┘
                │
                ▼
         ┌─────────────────────────────────────────────────────────────┐
         │  STEP 5  reconcileMachineStatus                              │
         │  If Instance.Spec.MachineRef != nil:                         │
         │    machineFactory.NewMachineFromRef → Machine adapter        │
         │    machine.GetStatus() → MachineStatus{Phase, Status,        │
         │                                        MachineReadyCondition}│
         │    SyncInstanceStatus:                                        │
         │      compare current vs desired                              │
         │      preserve LastTransitionTime when type+status unchanged  │
         │      if changed → SSA-patch field owner:                     │
         │        node-controller-instancestatus                        │
         │  If MachineRef == nil (static node) → skip                   │
         └─────────────────────────────────────────────────────────────┘
                │
                ▼
         ┌─────────────────────────────────────────────────────────────┐
         │  STEP 6  ReconcileSourceExistence                            │
         │  Garbage-collect orphaned instances:                         │
         │    linkedMachineExists(MachineRef)                           │
         │      → machineExists/NotFound                                │
         │    if machine exists → skip                                  │
         │    linkedNodeExists(NodeRef.Name)                            │
         │      → nodeExists/NotFound                                   │
         │    if node exists   → skip                                   │
         │    if BOTH confirmed NotFound:                               │
         │      removeInstanceFinalizer (safety: no machine deletion)   │
         │      Delete Instance                                         │
         │    if either is uncertain (error) → skip (safety net)        │
         └─────────────────────────────────────────────────────────────┘
                │
                ▼
         RequeueAfter: 1 minute
```

---

## Machine status mapping

### CAPI Machine (cluster.x-k8s.io/v1beta2)

```
Machine.Status.Phase    Machine conditions              → Instance.Status.Phase
──────────────────────────────────────────────────────────────────────────────
Pending                 any                             Pending
Provisioning            any                             Provisioning
Provisioned             any                             Provisioned
Running                 any                             Running
Deleting / Deleted      any                             Terminating
(other)                 any                             Unknown

InfrastructureReady condition priority table:
  InfrastructureReady != True     → Progressing / Error / Blocked (drain check)
  Deleting = True                 → Progressing / Blocked (drain blocked check)
  Phase = Running                 → Ready
  Ready condition present         → from Ready condition
  (default)                       → Progressing / Unknown

MachineReady condition → Instance.Status.Conditions["MachineReady"]
machine.Status (typed) → Instance.Status.MachineStatus
```

### MCM Machine (machine.sapcloud.io/v1alpha1)

```
Machine.Status.CurrentStatus.Phase     → Instance.Status.Phase
───────────────────────────────────────────────────────────────
Terminating + drain-blocked message     → Terminating / Blocked
Failed                                  → Error
Successful + Running                    → Running / Ready
Successful + other                      → Provisioned / Progressing
Processing                              → Provisioning / Progressing
Running                                 → Running / Ready
(other)                                 → Unknown

machine.Status (typed) → Instance.Status.MachineStatus
MachineReady condition → Instance.Status.Conditions["MachineReady"]
```

### Static Node (no Machine)

```
Node exists + IsStaticNode → Instance created with NodeRef, Phase=Running
Node gone                  → Instance deleted (only if MachineRef == nil)
```

---

## Instance.Status field ownership (SSA field owners)

| Field                          | Owner                                        |
|-------------------------------|----------------------------------------------|
| Status.Phase                  | node-controller-instancestatus               |
| Status.MachineStatus          | node-controller-instancestatus               |
| Status.Conditions[MachineReady] | node-controller-instancestatus             |
| Status.BashibleStatus         | node-controller-instance-bashible-status     |
| Status.Message                | node-controller-instance-bashible-status     |
| Status.Conditions[BashibleReady] heartbeat | node-controller-instance-bashible-heartbeat |
| Status.Conditions[BashibleReady] actual | bashible (external, not this controller)  |

---

## Deletion sequence

```
User deletes Instance
        │
        ▼
DeletionTimestamp set
        │
        ▼
reconcileDeletion:
  1. get Machine via MachineRef
  2. if Machine exists and not deleting → Delete Machine
  3. requeue 1s until Machine is gone
        │
        ▼
Machine confirmed gone (NotFound)
        │
        ▼
removeInstanceFinalizer → k8s garbage-collects Instance
```

If Instance has no MachineRef (static node), step 1 returns machineGone=true immediately
and the finalizer is removed at once.
