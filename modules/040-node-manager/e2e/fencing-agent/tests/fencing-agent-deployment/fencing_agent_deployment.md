# Fencing Agent Deployment

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the **fencing-agent** component of the `node-manager` module deploys and runs when `spec.fencing` is enabled on a NodeGroup.

**What it does:** Patches the existing `worker` NodeGroup with `spec.fencing.mode: Watchdog`, waits for the `fencing-agent-worker` DaemonSet to become ready, verifies fencing labels on worker nodes, then reverts fencing and confirms the DaemonSet is deleted.

## Prerequisites

- Existing NodeGroup named `worker` with at least one schedulable node
- Worker nodes support the `softdog` kernel module
- Fencing-agent image built and available in the Deckhouse registry
- Chainsaw CLI and `kubectl` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                       | Description                                                                                             |
| ---- | -------------------------- | ------------------------------------------------------------------------------------------------------- |
| 1    | `enable-fencing`           | Patches `worker` NodeGroup with `spec.fencing` (Watchdog mode, 60s timeout)                             |
| 2    | `wait-for-daemonset`       | Waits up to 5 minutes for `fencing-agent-worker` DaemonSet fully rolled out                             |
| 3    | `assert-pods-running`      | Asserts at least one fencing-agent pod is Running with container ready                                  |
| 4    | `assert-node-labels`       | Asserts worker node has `node-manager.deckhouse.io/fencing-enabled` and `fencing-mode: Watchdog` labels |
| 5    | `revert-fencing`           | Reverts `spec.fencing` on worker NodeGroup via JSON Merge Patch                                         |
| 6    | `assert-daemonset-deleted` | Asserts DaemonSet is deleted within 2 minutes                                                           |

**Cleanup:** Step 1 cleanup always reverts `spec.fencing` on the `worker` NodeGroup (runs even on failure).

## Files

| File                                  | Purpose                                                 |
| ------------------------------------- | ------------------------------------------------------- |
| `chainsaw-test.yaml`                  | Chainsaw test definition                                |
| `manifests/nodegroup-patch.yaml`      | NodeGroup patch enabling Watchdog fencing (60s timeout) |
| `asserts/assert-daemonset-ready.yaml` | Asserts DaemonSet has ready replicas                    |
| `asserts/assert-node-labels.yaml`     | Asserts fencing labels on worker node                   |
| `asserts/assert-daemonset-gone.yaml`  | Error-assert: DaemonSet deleted after fencing disabled  |

## Fencing Configuration

The test enables:

```yaml
spec:
  fencing:
    mode: Watchdog
    watchdog:
      timeout: 60s
```

Watchdog mode uses the kernel `softdog` module to detect node health. When the agent starts successfully, it sets labels on the node via the Kubernetes API — this is the proof that the agent process reached the API server.

## Running

```bash
# From the test directory
task run

# From fencing-agent root
task fencing-agent-deployment:run

# Or directly
chainsaw test --test-dir . --config ../../chainsaw-config.yaml
```

## Pass/Fail Criteria

- **Pass:** DaemonSet becomes ready; fencing-agent pod Running; worker node has fencing labels; DaemonSet deleted after revert
- **Fail:** DaemonSet not ready within 5 minutes, no fencing labels on node, or DaemonSet persists after fencing disabled

## Troubleshooting

### DaemonSet not ready (step 2)

Check fencing-agent pod status and events:

```bash
kubectl get daemonset -n d8-cloud-instance-manager fencing-agent-worker
kubectl get pods -n d8-cloud-instance-manager -l app=fencing-agent
kubectl describe pod -n d8-cloud-instance-manager -l app=fencing-agent
```

Common causes: missing `softdog` kernel module, image pull errors, or insufficient node resources.

### Missing node labels (step 4)

Labels are set by the agent after it successfully starts. If the DaemonSet is ready but labels are missing, check agent logs:

```bash
kubectl logs -n d8-cloud-instance-manager -l app=fencing-agent -c fencing-agent
```

### DaemonSet not deleted (step 6)

Verify fencing was reverted on the NodeGroup:

```bash
kubectl get nodegroup worker -o jsonpath='{.spec.fencing}'
kubectl get daemonset -n d8-cloud-instance-manager fencing-agent-worker
```

If cleanup did not run (test process killed), manually revert:

```bash
kubectl patch nodegroup worker --type=merge -p '{"spec":{"fencing":null}}'
```

## Safety

This test modifies the production `worker` NodeGroup. The cleanup block in step 1 and explicit revert in step 5 ensure fencing is disabled after the test. If the test process is killed, manually revert fencing as shown above.
