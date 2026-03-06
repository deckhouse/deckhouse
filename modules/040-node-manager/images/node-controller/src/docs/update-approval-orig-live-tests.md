# update_approval_orig live tests (bash)

This document describes a live-cluster bash harness for validating behavior parity with the legacy hook logic from `update_approval_orig.go`.

Script path: `hack/update_approval_orig_live_tests.sh`

Default test profile uses `CloudEphemeral` NodeGroups for update-approval scenarios, because that is the strict path for `update_approval` logic (`desired/ready` gating and `RollingUpdate` support).

## What is covered

- Approval flow:
  - waiting node -> approved
  - `maxConcurrent` limit is respected
- Updated-node processing:
  - approved/drain annotations are cleared when checksum matches and node is Ready
  - node is uncordoned when `drained` is removed
- Disruption flow:
  - `Automatic` + `drainBeforeApproval=false` -> `disruption-approved`
  - `Automatic` + `drainBeforeApproval=true` -> `draining=bashible`
  - `Manual` mode blocks auto-approval
  - `RollingUpdate` mode triggers `Instance` deletion (tested on `CloudEphemeral` NodeGroup)
  - disruption window can block approvals
- Checksum edge case:
  - missing checksum key in `configuration-checksums` secret leads to no mutations

## Safety model

- Every test creates unique object names (`TEST_PREFIX` + timestamp + random suffix).
- Tests only touch objects created by the harness.
- Cleanup is automatic via `trap EXIT`:
  - deletes test `NodeGroup`/`Node`/`Instance`
  - removes test keys from `d8-cloud-instance-manager/configuration-checksums`

## Prerequisites

- Access to a live cluster with Deckhouse node controller deployed.
- Permissions:
  - CRUD for `nodegroups.deckhouse.io`, `instances.deckhouse.io`, `nodes`
  - patch for `secrets` in `d8-cloud-instance-manager`
- Existing resources:
  - CRDs: `nodegroups.deckhouse.io`, `instances.deckhouse.io`
  - Secret: `d8-cloud-instance-manager/configuration-checksums`
  - Deployment: `d8-system/deckhouse` (default; configurable)

## Run

Run all cases:

```bash
bash hack/update_approval_orig_live_tests.sh
```

Run selected cases:

```bash
TEST_CASES="approve_waiting,auto_disruption_no_drain,rolling_update_deletes_instance" \
  bash hack/update_approval_orig_live_tests.sh
```

Tune polling/timeout:

```bash
TIMEOUT_SECONDS=180 POLL_INTERVAL_SECONDS=3 bash hack/update_approval_orig_live_tests.sh
```

## Environment variables

- `KUBECTL` (default `/opt/deckhouse/bin/kubectl` if exists, otherwise `kubectl`)
- `TIMEOUT_SECONDS` (default `600`)
- `POLL_INTERVAL_SECONDS` (default `2`)
- `PRE_CASE_SETTLE_SECONDS` (default `15`)
- `TEST_PREFIX` (default `ua-live`)
- `TEST_CASES` (default `all`)
- `MACHINE_NS` (default `d8-cloud-instance-manager`)
- `CHECKSUM_SECRET` (default `configuration-checksums`)
- `CONTROLLER_NS` (default `d8-system`)
- `CONTROLLER_DEPLOYMENT` (default `deckhouse`)
- `MAX_NODEGROUP_NAME_LENGTH` (default `16`)
- `STOP_ON_FIRST_FAIL` (default `true`)
- `DEBUG_ON_FAIL` (default `true`)
- `DEBUG_LOGS_SINCE` (default `15m`)

## Notes

- The harness creates synthetic `Node` objects; this is intentional to test controller logic without touching real worker nodes.
- Some legacy edge-cases from `update_approval_orig.go` are hard to test safely in a shared cluster (for example, exact `master` NodeGroup semantics) and are not included here by default.
