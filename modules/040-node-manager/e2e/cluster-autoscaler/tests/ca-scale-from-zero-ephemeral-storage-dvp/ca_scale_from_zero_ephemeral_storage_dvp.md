# Cluster Autoscaler Scale From Zero with Ephemeral Storage (DVP)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **a pod requesting `ephemeral-storage` scales a CAPI NodeGroup up from zero** on a DVP / Cluster API cluster.

**What it does:** Clones the `worker` DVPInstanceClass with a 20Gi root disk, creates a NodeGroup with `minPerZone: 0`, checks the `capacity.cluster-autoscaler.kubernetes.io/ephemeral-disk` annotation on its MachineDeployment, restarts CA, deploys a pod requesting 1Gi of `ephemeral-storage`, and verifies that CA scales the group up and the pod runs on the new node.

## Prerequisites

- Deckhouse cluster with DVP / CAPI cloud provider
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- CA container arguments contain `clusterapi`
- Existing `DVPInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Background

With no node in a MachineDeployment, the `clusterapi` provider of CA builds the template node from the `capacity.cluster-autoscaler.kubernetes.io/*` annotations of that MachineDeployment and puts them into the template's capacity and allocatable as they are. Before the fix node-controller wrote `cpu`, `memory`, `labels` and `taints`, but no `ephemeral-disk`, so the template had no ephemeral-storage at all:

- a pod requesting any failed `NodeResourcesFit` on the template (`Insufficient ephemeral-storage`) and never scaled the group up; every Deckhouse module requests `ephemeral-storage` through helm_lib;
- the DaemonSet pods CA adds to the template drove its free ephemeral-storage negative, which broke `--balance-similar-node-groups` across zones (covered by `ca-zone-balancing-yandex`, DVP has a single zone).

node-controller now writes the root disk less kubelet's deductions (`evictionHard nodefs.available` 5% and `systemReserved` 1Gi in the default `Auto` mode): 18Gi for the 20Gi disk of this test.

CA keeps the template node of a group it has seen a node of in memory, so the test restarts CA before deploying the pod: the template is then built from the annotations.

## Test Steps

| Step | Name                                                  | Description                                                                                 |
| ---- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| 1    | `assert-dvpinstanceclass-worker-exists`               | Asserts `DVPInstanceClass worker` exists (cleanup waits for test nodes removal)             |
| 2    | `cleanup-leftover-resources`                          | Deletes leftover NodeGroup and instance class                                               |
| 3    | `create-e2e-worker-ephemeral-instanceclass`           | Clones `worker` → `e2e-worker-ephemeral` with a 20Gi root disk                              |
| 4    | `apply-nodegroup`                                     | Applies NodeGroup `e2e-worker-ephemeral` (`minPerZone: 0`, `maxPerZone: 1`)                 |
| 5    | `assert-cluster-autoscaler-exists`                    | Asserts CA deployment is ready                                                              |
| 6    | `assert-ca-uses-clusterapi-provider`                  | Verifies CA arguments contain `clusterapi`                                                  |
| 7    | `verify-machine-deployment-ephemeral-disk-annotation` | Verifies the MachineDeployment has `ephemeral-disk: 18Gi`                                   |
| 8    | `restart-cluster-autoscaler`                          | Rollout restart, so CA has no node info cached for the group                                |
| 9    | `wait-for-ca-initialization`                          | Sleep 15s                                                                                   |
| 10   | `apply-deployment`                                    | Applies a 1-replica Deployment requesting 1Gi of `ephemeral-storage`                        |
| 11   | `assert-ca-scales-group-up`                           | Waits for CA to set the MachineDeployment replicas to 1; fails fast on `Insufficient ephemeral-storage` |
| 12   | `assert-pod-running`                                  | Waits for 1 ready replica                                                                   |
| 13   | `assert-pod-on-ephemeral-node`                        | Verifies the pod runs on an `e2e-worker-ephemeral` node, prints its allocatable             |

**Cleanup:** NodeGroup, instance class, and Deployment are deleted via step cleanup blocks.

## Files

| File                        | Purpose                                                                       |
| --------------------------- | ----------------------------------------------------------------------------- |
| `chainsaw-test.yaml`        | Chainsaw test definition                                                      |
| `manifests/nodegroup.yaml`  | NodeGroup `e2e-worker-ephemeral` with `minPerZone: 0` and a `dedicated` taint |
| `manifests/deployment.yaml` | 1-replica Deployment requesting 1Gi of `ephemeral-storage`                    |

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-scale-from-zero-ephemeral-storage-dvp:run
```

## Pass/Fail Criteria

- **Pass:** the MachineDeployment has `ephemeral-disk: 18Gi`; CA scales the group up from zero; the pod runs on an `e2e-worker-ephemeral` node
- **Fail:** the annotation is missing or different; CA logs `Insufficient ephemeral-storage` for the group's template node; the pod stays Pending

## Troubleshooting

### Step 7 fails — annotation missing

The node-controller does not write `ephemeral-disk`: the cluster runs a release without the fix.

### Step 11 fails with `Insufficient ephemeral-storage`

CA built the template node without ephemeral-storage, or with less than the pod and the DaemonSet pods request. Check the MachineDeployment annotations and that CA was restarted after the NodeGroup was created.
