# Cluster Autoscaler Zone Balancing on Scale From Zero (Yandex, CAPI)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **CA spreads a scale-up from zero of a multi-zone CAPI NodeGroup across zones** (`--balance-similar-node-groups`) on Yandex Cloud.

**What it does:** Clones the `worker` YandexInstanceClass, creates a CAPI NodeGroup over every zone with `minPerZone: 0` and `maxPerZone: 3`, checks that every zonal MachineDeployment carries the `ephemeral-disk` annotation, restarts CA, deploys three pods that need a node each and request `ephemeral-storage`, and verifies that the scale-up went into more than one zone.

## Prerequisites

- Deckhouse 1.77+ on Yandex Cloud, where NodeGroups run on CAPI by default (the test NodeGroup has no `node.deckhouse.io/use-mcm`)
- At least two zones in the provider configuration
- A Cluster Autoscaler Deployment with `--cloud-provider=clusterapi` in `d8-cloud-instance-manager`
- Existing `YandexInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Background

Each zone of a CAPI NodeGroup is a MachineDeployment, which CA treats as a node group. With `--balance-similar-node-groups` CA splits a scale-up between node groups whose template nodes are similar; with no nodes those templates are built from the MachineDeployment annotations.

Before node-controller wrote the `capacity.cluster-autoscaler.kubernetes.io/ephemeral-disk` annotation, the templates had no ephemeral-storage. CA adds the DaemonSet pods to every template, and their ephemeral-storage requests drove its free ephemeral-storage negative. CA's tolerance check `larger - smaller <= larger * ratio` fails for two equal negative values, so it logged `free resources not within tolerance`, treated identical zones as dissimilar and put the whole scale-up into one zone: 0->3 in one zone instead of one node per zone. MCM was not affected, its templates come from the autoscaler patch with a fixed ephemeral storage.

DVP clusters have a single zone, so this scenario has no DVP variant; `ca-scale-from-zero-ephemeral-storage-dvp` covers the other half of the same bug there.

## Test Steps

| Step | Name                                       | Description                                                                                        |
| ---- | ------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| 1    | `assert-yandexinstanceclass-worker-exists` | Asserts `YandexInstanceClass worker` exists (cleanup waits for test nodes removal)                 |
| 2    | `cleanup-leftover-resources`               | Deletes leftover NodeGroup and instance class                                                      |
| 3    | `create-e2e-worker-zones-instanceclass`    | Clones `worker` → `e2e-worker-zones` (2 cores, 4Gi, 30Gi disk)                                     |
| 4    | `apply-nodegroup`                          | Applies CAPI NodeGroup `e2e-worker-zones` over every zone (`minPerZone: 0`, `maxPerZone: 3`)       |
| 5    | `verify-machine-deployments`               | Verifies at least two zonal MachineDeployments, each with `ephemeral-disk: 28160Mi`                |
| 6    | `restart-cluster-autoscaler`               | Finds the CA Deployment with `--cloud-provider=clusterapi` and restarts it                         |
| 7    | `wait-for-ca-initialization`               | Sleep 15s                                                                                          |
| 8    | `apply-deployment`                         | Applies 3 replicas, one per node, each requesting 1Gi of `ephemeral-storage`                       |
| 9    | `assert-scale-up-spread-across-zones`      | Waits until three nodes are requested and checks they are spread over more than one MachineDeployment |
| 10   | `assert-pods-running`                      | Waits for 3 ready replicas                                                                         |
| 11   | `assert-pods-in-several-zones`             | Verifies the pods run in more than one zone                                                        |

**Cleanup:** NodeGroup, instance class, and Deployment are deleted via step cleanup blocks.

## Files

| File                        | Purpose                                                                          |
| --------------------------- | -------------------------------------------------------------------------------- |
| `chainsaw-test.yaml`        | Chainsaw test definition                                                         |
| `manifests/nodegroup.yaml`  | CAPI NodeGroup `e2e-worker-zones` over every zone                                |
| `manifests/deployment.yaml` | 3-replica Deployment with hostname anti-affinity and `ephemeral-storage` requests |

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-zone-balancing-yandex:run
```

## Pass/Fail Criteria

- **Pass:** every zonal MachineDeployment has `ephemeral-disk: 28160Mi`; the three requested nodes are spread over more than one zone; the pods run in more than one zone
- **Fail:** fewer than two zones or a missing annotation; CA puts the whole scale-up into one zone (the catch block prints CA's `not similar` and `Final scale-up plan` lines)
