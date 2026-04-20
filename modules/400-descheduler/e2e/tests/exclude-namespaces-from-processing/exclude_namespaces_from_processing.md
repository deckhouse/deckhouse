# Descheduler Exclude Namespaces from Processing

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the Deckhouse descheduler patch (`001-filter-pods-in-deckhouse-namespaces.patch`) which prevents eviction of pods in `d8-*` namespaces.

**What it does:** Creates a Deployment in the `d8-descheduler` namespace (protected by the Deckhouse patch) and bare pods in a regular test namespace. After the descheduler runs with LowNodeUtilization, verifies that protected pods were NOT evicted and no eviction events exist for them.

## Prerequisites

- Multi-node Kubernetes cluster (minimum 3 nodes including master)
- Descheduler pre-installed in the `d8-descheduler` namespace (with the Deckhouse patch applied)
- Chainsaw CLI installed. See `../../E2E.md` for instructions.

## Why d8-descheduler Namespace

The test needs to verify that pods in `d8-*` namespaces are protected from eviction. Creating a new `d8-*` namespace is blocked by the `ValidatingAdmissionPolicy` (`system-ns.deckhouse.io`), so the test uses the existing `d8-descheduler` namespace. A Deployment with unique labels ensures test pods don't interfere with the descheduler itself.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-descheduler-ready` | Asserts descheduler deployment exists and has ready replicas |
| 2 | `check-minimum-nodes` | Verifies cluster has at least 2 worker nodes |
| 3 | `create-protected-workload` | Creates Deployment (5 replicas) in d8-descheduler namespace (cleanup deletes it) |
| 4 | `wait-protected-deployment-ready` | Waits for protected Deployment Available condition and 5 ready replicas |
| 5 | `create-regular-workload` | Selects a worker node and creates 5 regular pods in the test namespace |
| 6 | `wait-regular-pods-ready` | Waits for all regular pods (by label selector) to be Ready |
| 7 | `apply-descheduler-cr` | Applies Descheduler CR with LowNodeUtilization strategy (cleanup deletes CR) |
| 8 | `assert-configmap-updated` | Asserts descheduler policy ConfigMap contains the new profile (native assert) |
| 9 | `wait-descheduler-ready` | Waits for descheduler deployment Available condition (native wait) |
| 10 | `wait-for-descheduler-cycle` | Polls descheduler logs for LowNodeUtilization execution |
| 11 | `verify-protected-deployment-not-evicted` | Asserts protected Deployment still has 5 ready replicas |
| 12 | `verify-no-evictions-in-protected-namespace` | Asserts zero eviction events for protected pods in d8-descheduler |
| 13 | `verify-namespace-filtering-logs` | Checks descheduler logs for namespace filtering messages |

**Note:** The workloads are created BEFORE the descheduler CR to ensure pods are stable before eviction starts.

**Cleanup:** Step 3 cleanup deletes the protected Deployment from d8-descheduler. Step 7 cleanup deletes the Descheduler CR. Test namespace (regular pods) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `files/descheduler-cr.yaml` | Descheduler CR with LowNodeUtilization strategy and tuned thresholds |
| `files/protected-deployment.yaml` | Deployment with 5 pause pod replicas in d8-descheduler namespace |
| `files/regular-pods.yaml` | Template with 5 pause pods in the test namespace (uses `($targetNode)` binding) |

## Policy Config

- **Thresholds** (underutilized): cpu < 55%, memory < 65%, pods < 50%
- **TargetThresholds** (overutilized): cpu > 70%, memory > 80%, pods > 70%

### How namespace exclusion works

The Deckhouse descheduler patch adds a filter to `DefaultEvictor` that rejects pods from protected namespaces (`d8-*` and `kube-system`). When the LowNodeUtilization plugin identifies pods for eviction, the evictor checks each pod's namespace and skips those in protected namespaces.

The test verifies this by placing a Deployment in `d8-descheduler` (protected) and bare pods in the test namespace (unprotected). After the descheduler runs, protected pods must still be running with all replicas ready.

## Running

```bash
# From the e2e directory
task run:exclude-namespaces

# Or directly
chainsaw test --test-dir ./tests/exclude-namespaces-from-processing/
```

## Pass/Fail Criteria

- **Pass:** Protected Deployment still has 5 ready replicas; zero eviction events for protected pods; descheduler plugin executes
- **Fail:** Protected pods evicted, descheduler not found, or plugin doesn't execute within 7 minutes

## Troubleshooting

**Protected pods evicted (test fails at step 11)**

The Deckhouse namespace exclusion patch may not be applied. Check if the descheduler image includes the patch:

```bash
kubectl logs -n d8-descheduler -l app=descheduler -c descheduler | grep "pod in the deckhouse namespace"
```

**Plugin executes but no filtering logs (step 13 warning)**

The log format may differ between descheduler versions. This step only warns, it does not fail the test. The actual protection is verified in steps 11-12.
