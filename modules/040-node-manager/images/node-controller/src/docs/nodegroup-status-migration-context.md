# NodeGroup Status Migration Context

This file captures the current migration context from old hook-based logic to controller-runtime logic, so work can continue without re-discovery.

## Scope

- Old reference implementation:
  - `modules/040-node-manager/images/node-controller/src/internal/controller/update_node_group_status.go`
- New controller implementation:
  - `modules/040-node-manager/images/node-controller/src/internal/controller/nodegroup_status.go`

## Why migration was needed

- `node-controller` is a separate Go module: `github.com/deckhouse/node-controller`.
- Imports like `github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/...` are blocked by Go `internal` visibility rules from this module.

## What was changed already

1. Added local conditions package in `node-controller`:
   - `modules/040-node-manager/images/node-controller/src/internal/conditions/types.go`
   - `modules/040-node-manager/images/node-controller/src/internal/conditions/calculate.go`
2. Replaced `hooks/internal/...` imports in `nodegroup_status.go` with local package:
   - now uses `github.com/deckhouse/node-controller/internal/conditions`.
3. Reworked conversion helpers in `nodegroup_status.go`:
   - `convertToCalcConditions`
   - `convertFromCalcConditions`
4. `update_node_group_status.go` marked as excluded from build:
   - `//go:build ignore`
   - `// +build ignore`
5. Status patch behavior moved closer to old `buildUpdateStatusPatch`:
   - removed writing `ng.Status.Error = statusMsg` in new controller path.

## Current compile status

- Controller package compiles:
  - `go test ./internal/controller -run TestDoesNotExist` passes.
- Full `go test ./internal/controller/...` currently has pre-existing test failures unrelated to blocked imports.

## Confirmed behavior differences (old vs new)

1. Runtime model:
   - Old: hook snapshots + `PatchCollector`.
   - New: controller-runtime Reconcile + typed status patch.
2. `UpToDate` counting:
   - Old code counted with broader snapshot loop behavior.
   - New code counts only nodes belonging to current NodeGroup.
3. Zones fallback:
   - Old could use `defaultZonesNum` from snapshot (can remain 0).
   - New fallback in `getZonesCount()` returns `1` when secret/marshal data unavailable.
4. Frozen handling:
   - New controller checks `Frozen` in MCM and CAPI machine deployments.
5. Events:
   - Old creates `events.k8s.io/v1` object manually.
   - New uses `Recorder.Event(...)`.
6. `SetProcessedStatus`:
   - Old calls `set_cr_statuses.SetProcessedStatus(...)`.
   - New controller does not call equivalent yet.

## About `ConditionTypeFrozen`

- `ConditionTypeFrozen` constant exists in new controller.
- It is not emitted as a separate condition in calculated conditions.
- Frozen is currently used as an input to error condition behavior (`HasFrozenMachineDeployment`).

## About `buildUpdateStatusPatch`

- The helper function was provided and reviewed in chat.
- In current checked tree, only call site existed in `update_node_group_status.go`; function body was not found in active files.
- New controller currently applies equivalent status updates inline in `Reconcile()`.

## Open decision: `SetProcessedStatus` in new controller

Why this matters:
- CRD includes columns:
  - `.status.deckhouse.synced`
  - `.status.deckhouse.observed.lastTimestamp`
  - `.status.deckhouse.processed.lastTimestamp`
- `SetObservedStatus` for NodeGroup is still used in `hooks/get_crds.go`.
- Without processed updates, `synced` can become stale/always false.

Options:
1. Compatibility-first:
   - import `github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses`
   - add adapter in controller-runtime flow and update processed fields.
2. Decouple from hook libs:
   - implement local equivalent with checksum parity.

Current preferred direction discussed:
- Use imported `set_cr_statuses` package for strict behavior parity, then potentially refactor later.

## Files relevant for next steps

- New controller:
  - `modules/040-node-manager/images/node-controller/src/internal/controller/nodegroup_status.go`
- Local conditions:
  - `modules/040-node-manager/images/node-controller/src/internal/conditions/calculate.go`
  - `modules/040-node-manager/images/node-controller/src/internal/conditions/types.go`
- Old hook filter contract:
  - `modules/040-node-manager/hooks/get_crds.go` (`applyNodeGroupCrdFilter`)
- Processed/observed implementation:
  - `go_lib/hooks/set_cr_statuses/hook.go`
- CRD columns/status contract:
  - `modules/040-node-manager/crds/node_group.yaml`

## Suggested next implementation step

- Add processed-status update to `nodegroup_status` reconcile loop (preferably via a small wrapper helper), preserving checksum/filter parity with old hook behavior.
