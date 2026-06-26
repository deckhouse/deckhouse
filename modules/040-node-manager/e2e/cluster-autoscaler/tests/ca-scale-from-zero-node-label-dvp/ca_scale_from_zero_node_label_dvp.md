# Cluster Autoscaler Scale From Zero with Node Label Selector (DVP)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **scale-from-zero works when pods use `nodeSelector` with `node.deckhouse.io/group`** on a DVP / Cluster API cluster.

**What it does:** Creates a NodeGroup without `node.deckhouse.io/group` in `nodeTemplate.labels`, deploys pods with `nodeSelector: node.deckhouse.io/group: e2e-worker-infra`, and verifies that CA correctly matches the selector during scale-from-zero simulation and triggers scale-up.

This test covers the fix from [PR #20174](https://github.com/deckhouse/deckhouse/pull/20174): system labels like `node.deckhouse.io/group` (added by kubelet at bootstrap) must be included in the `capacity.cluster-autoscaler.kubernetes.io/labels` annotation on MachineDeployment.

## Prerequisites

- Deckhouse cluster with DVP / CAPI cloud provider
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- CA container args contain `clusterapi`
- PR #20174 fix applied (system labels in capacity annotation)
- Existing `DVPInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Background

Before the fix, the `serializeLabels` function only included labels from `spec.nodeTemplate.labels` in the MachineDeployment capacity annotation. The label `node.deckhouse.io/group` is added by kubelet during node bootstrap, not from NodeGroup template — so CA could not match pods using this label in `nodeSelector` during scale-from-zero simulation.

## Test Steps

| Step | Name                                          | Description                                                                                             |
| ---- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| 1    | `assert-dvpinstanceclass-worker-exists`       | Asserts `DVPInstanceClass worker` exists (cleanup waits for test nodes removal)                         |
| 2    | `cleanup-leftover-resources`                  | Deletes leftover NodeGroup and instance class                                                           |
| 3    | `create-e2e-worker-small-instanceclass`       | Clones `worker` → `e2e-worker-small`                                                                    |
| 4    | `apply-nodegroup-infra`                       | Applies NodeGroup `e2e-worker-infra` (no `node.deckhouse.io/group` in nodeTemplate.labels)              |
| 5    | `assert-cluster-autoscaler-exists`            | Asserts CA deployment is ready                                                                          |
| 6    | `assert-ca-uses-clusterapi-provider`          | Verifies CA args contain `clusterapi`                                                                   |
| 7    | `restart-cluster-autoscaler`                  | Rollout restart and wait for readiness                                                                  |
| 8    | `wait-for-ca-initialization`                  | Sleep 15s                                                                                               |
| 9    | `verify-machine-deployment-labels-annotation` | Verifies MachineDeployment has `node.deckhouse.io/group=e2e-worker-infra` in capacity labels annotation |
| 10   | `apply-deployment`                            | Applies Deployment with `nodeSelector: node.deckhouse.io/group: e2e-worker-infra`                       |
| 11   | `wait-for-deckhouse-processing`               | Sleep 30s                                                                                               |
| 12   | `assert-ca-triggers-scale-up`                 | Polls CA logs for scale-up decision on `e2e-worker-infra`                                               |
| 13   | `assert-pods-running`                         | Asserts 3 ready replicas                                                                                |
| 14   | `assert-pods-on-infra-nodes`                  | Verifies all pods are on `e2e-worker-infra` nodes                                                       |

**Cleanup:** NodeGroup, instance class, and Deployment are deleted via step cleanup blocks.

## Files

| File                                                      | Purpose                                                                             |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `chainsaw-test.yaml`                                      | Chainsaw test definition                                                            |
| `../common/manifests/nodegroup-infra-dvp.yaml`            | NodeGroup without `node.deckhouse.io/group` in template labels                      |
| `../common/manifests/deployment-node-label-selector.yaml` | 3-replica Deployment with `nodeSelector: node.deckhouse.io/group: e2e-worker-infra` |
| `../common/asserts/assert-deployment-ready.yaml`          | Asserts 3 ready replicas                                                            |

## Key Verification (Step 9)

The test explicitly checks that the MachineDeployment annotation contains the system label:

```
capacity.cluster-autoscaler.kubernetes.io/labels: node.deckhouse.io/group=e2e-worker-infra
```

If this annotation is missing, step 9 fails with a message indicating PR #20174 fix is not applied.

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-scale-from-zero-node-label-dvp:run
```

## Pass/Fail Criteria

- **Pass:** MachineDeployment has correct capacity labels annotation; CA triggers scale-up for `e2e-worker-infra`; 3 pods running on correct nodes
- **Fail:** Missing capacity labels annotation, CA does not trigger scale-up, pods Pending or on wrong nodes

## Troubleshooting

### Step 9 fails — annotation missing

The PR #20174 fix is not applied. Check MachineDeployment:

```bash
kubectl get machinedeployments.cluster.x-k8s.io -n d8-cloud-instance-manager \
  -l node-group=e2e-worker-infra -o yaml | grep capacity.cluster-autoscaler
```

### CA does not trigger scale-up (step 12)

Pods remain Pending because CA cannot match `nodeSelector` to any NodeGroup:

```bash
kubectl describe pods -n $NAMESPACE -l app=e2e-nginx
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler -c cluster-autoscaler --tail=200
```

### Pods on wrong NodeGroup

Verify nodes have label `node.deckhouse.io/group=e2e-worker-infra` (set by kubelet, not NodeGroup template):

```bash
kubectl get nodes -l node.deckhouse.io/group=e2e-worker-infra --show-labels
```
