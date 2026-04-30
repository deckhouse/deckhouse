# fencing-agent e2e tests

End-to-end tests for the fencing-agent component of the `node-manager` module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Scope

Verifies that the fencing-agent **deploys** and **runs** when `spec.fencing` is enabled on a NodeGroup.

## Pre-conditions

- A NodeGroup named `worker` exists with at least one schedulable node.
- Worker nodes support the `softdog` kernel module.
- The fencing-agent image is built and available in the deckhouse registry.

## What it does

1. Patches the `worker` NodeGroup with `spec.fencing.mode: Watchdog`, `watchdog.timeout: 60s`.
2. Waits up to 5 minutes for the `fencing-agent-worker` DaemonSet to become Ready (`numberReady > 0` and `numberReady == desiredNumberScheduled`).
3. Asserts at least one fencing-agent pod is `Running` with the `fencing-agent` container ready.
4. Asserts the worker node carries labels `node-manager.deckhouse.io/fencing-enabled` and `node-manager.deckhouse.io/fencing-mode: Watchdog` (proof the agent process started and reached the K8s API).
5. Reverts `spec.fencing` via chainsaw's native `patch` op (JSON Merge Patch with `fencing: null`).
6. Asserts the DaemonSet is deleted within 2 minutes.

JUnit reports are written to `tests/fencing-agent-deployment/reports/`.
