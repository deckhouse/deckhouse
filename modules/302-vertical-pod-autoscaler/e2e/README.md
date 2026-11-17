# Vertical Pod Autoscaler E2E Tests

End-to-end tests for the `vertical-pod-autoscaler` module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

The suite checks the behavior the module gives to users:

- the module components (recommender, updater, admission controller), CRDs and admission webhook are healthy;
- all four update modes (`Off`, `Initial`, `Recreate`, `InPlaceOrRecreate`) do what they promise;
- the Deckhouse patches work: the default update mode is `InPlaceOrRecreate` on Kubernetes 1.33+, and the per-resource `recommendationFactor` is validated and applied.

Each case lives in `tests/<case>/` and is self-contained: it creates its own namespace, checks its preconditions, and cleans up after itself, even when it fails. Cases do not depend on each other or on the run order, so they run in parallel (`parallel: 8` in `chainsaw-config.yaml`, one slot per case). The workload is an idle `agnhost` pause Deployment with requests below the VPA minimum, so the updater acts right away instead of after the 12-hour grace period. Negative checks (for example, "`Off` does not touch pods") rely on a control Deployment with a `Recreate` VPA in the same namespace, so a stopped updater cannot make them pass by accident. The control VPA is created only after the subject VPA has a recommendation, so every eviction of the control pods happens in an updater cycle that already saw the subject's recommendation. The updater evicts, the ReplicaSet controller recreates the pod, and the admission controller sets the requests and the `vpaUpdates` annotation. The checks of the `Off` and `Initial` cases also require that the subject pods were created before the subject VPA, so a pod that was evicted and recreated cannot pass as an original one.

Shared steps and manifests are in `tests/common/`. A workload Deployment and its VerticalPodAutoscaler have the same name, which the shared steps take as the binding `workload` (default `subject`). The handlers that show the pods, events, VerticalPodAutoscalers and component logs when a step fails are in `chainsaw-config.yaml` (`spec.error.catch`), so run the suite with `--config chainsaw-config.yaml`, as the Taskfile does. `e2e.yaml` is intentionally empty: it is reserved for the future suite configuration and the constraints for running it.

## Cases

| Case                                    | What it checks                                                                                                                                      |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `module-ready`                          | CRDs are established, the three components are rolled out, the admission webhook Service has a ready endpoint and the webhook configuration is wired |
| `default-update-mode`                   | On Kubernetes 1.33+ a VPA without `updatePolicy` is stored with `InPlaceOrRecreate`; explicit `Recreate` and `Auto` are kept unchanged               |
| `update-mode-off`                       | A VPA in `Off` publishes a recommendation but pods are not evicted or resized, while the control Deployment is recreated                            |
| `update-mode-initial`                   | A VPA in `Initial` applies the recommendation only to pods created after it exists (a pod added by scaling up), running pods stay untouched         |
| `update-mode-recreate`                  | A VPA in `Recreate` evicts pods with requests below the recommendation, records the events, and the pods come back with new requests                |
| `update-mode-in-place`                  | A VPA in `InPlaceOrRecreate` resizes pods in place, without container restarts or evictions, and records the resize events                          |
| `recommendation-factor`                 | The webhook accepts a valid `recommendationFactor` and rejects zero, too large, too precise and unsupported values; a CPU factor changes the CPU target only |
| `known-defect-recommendation-conditions` | The recommender publishes the `RecommendationProvided` condition. Skipped because of a known module defect, see below                              |

## Prerequisites

- A Deckhouse cluster on Kubernetes 1.33 or newer; the `kubectl` context must point to it.
- The `vertical-pod-autoscaler`, `prometheus` and `prometheus-metrics-adapter` modules are enabled.
- `registry.k8s.io` is reachable from the cluster nodes (the workload image is `registry.k8s.io/e2e-test-images/agnhost:2.53`).
- Tools: [Chainsaw](https://kyverno.github.io/chainsaw/) 0.2.14 or newer, [Task](https://taskfile.dev/), `kubectl`.

The suite changes nothing in the module settings and never touches ModuleConfigs.

## Running

Run these commands from `modules/302-vertical-pod-autoscaler/e2e`:

```bash
# Lint the Chainsaw configuration and every test
task lint

# Run the whole suite
task run

# Run one case
task run TEST=update-mode-off
```

`task run TEST=<case>` passes `--test-dir tests/<case>` to Chainsaw. A case name without a directory makes Chainsaw exit with an error. Do not use `--include-test-regex` to select a case: it maps to Go's `-test.run`, needs the form `chainsaw/^vpa-<case>$`, and a plain name silently runs nothing and exits 0.

Extra Chainsaw flags can be passed after `--`, for example `task run -- --parallel 1`.

The cases that wait for a recommendation or for the updater take from about a minute (`recommendation-factor`) to about three minutes (`update-mode-off` and `update-mode-initial`, whose control VPA starts only after the subject's recommendation); `module-ready` and `default-update-mode` take under half a minute. With the seven runnable cases in parallel, the whole suite took 205 seconds in the last full run and 189 to 195 seconds in earlier runs: the waits end on the recommender (1 minute) and updater (30 seconds) cycles, so the total depends on where the run starts in them.

### Report

The JUnit report is written to `reports/vertical-pod-autoscaler.xml`, or to `$E2E_REPORTS_DIR/vertical-pod-autoscaler.xml` when the variable is set. `reports/` is git-ignored. Attach the report to the pull request.

Two runs of the suite on the same cluster at the same time are not supported: the namespace names are fixed, so the second run fails its leftover check and its cleanup deletes the namespaces of the first run.

## Test namespaces

Every case declares its own namespace, `e2e-vpa-<case>`, and Chainsaw creates it. The first step of every case, `prepare-namespace`, uses the shared template `tests/common/steps/test-namespace.yaml`. It checks the namespace is `Active`, the default ServiceAccount exists, and no Deployment or VPA is left in the namespace by an interrupted run.

When multitenancy-manager is enabled, it adopts every new namespace into a Project of the `simple` template and forbids deleting such a namespace directly. So the cleanup of the template deletes the Project `e2e-vpa-<case>` (which deletes the namespace), or the namespace itself when the `Project` resource does not exist or no Project owns the namespace, and then waits until the namespace is gone. The delete and the wait are repeated once: a Project adopts a namespace after a 15 second grace period, which is about the time short cases end, so the first delete may choose the namespace just before the Project appears, and the second one then deletes the Project that would otherwise recreate the namespace. The cleanup looks for the Project API `deckhouse.io/v1alpha3` first, then `deckhouse.io/v1alpha2`. The cleanup of the step runs before Chainsaw's own namespace deletion, which then finds the namespace already deleted and does nothing. The `simple` template creates no LimitRange or ResourceQuota, so VPA behavior is not affected.

This deviates from `testing/e2e/AGENTS.md`, which asks for one suite namespace created and deleted by the Taskfile with the scripts from `testing/e2e/lib`. That approach keeps bash in the Taskfile, does not allow running a case with a plain `chainsaw test`, and makes the cases share a namespace and run one by one. Here each case owns its namespace, which is what allows the parallel run. The approach relies on Chainsaw running cleanups in the order of the operations; it was verified on Chainsaw 0.2.14.

### After an interrupted run

If a run is interrupted (for example, Chainsaw is killed), the next run of that case fails at `prepare-namespace` because of the leftover objects, but its cleanup still deletes the namespace, so the run after that passes. Alternatively, delete the leftovers by hand: the Project named `e2e-vpa-<case>`, or the namespace of that name when multitenancy-manager is disabled. The shared helper does both and also handles a late Project: run `bash testing/e2e/lib/delete-namespace.sh e2e-vpa-<case>` from the repository root.

```bash
kubectl get ns,projects.deckhouse.io -o name | grep e2e-vpa
```

## Notes for maintainers

- `chainsaw lint` checks only the Test and Configuration documents, so `task lint` also loads every test with `--no-cluster` and a test filter that matches nothing: loading validates the StepTemplates the tests use, and nothing runs.
- The suite depends on these JMESPath conventions of Chainsaw 0.2.14:
  - an expression under `status:` is evaluated relative to `status`; a mixed expression (for example, one that compares `status` with `spec`) goes to the top level of the resource;
  - `==` between a number of an object (int64) and a literal is false; use `<`, `>=`, `to_number()` or a value in the tree instead;
  - never call a function on a possibly-null operand: guard it with `!= null &&`, and use ``|| `[]` `` for lists; a field of null is null, so guard the operand itself (`lowerBound.cpu != null`), not each parent;
  - `x_k8s_server_version` takes `$config`.

## Not covered

- Changes of the module settings (ModuleConfig) and destructive scenarios.
- The patches for DaemonSet targets (OpenKruise DaemonSet, `spec.scope`) and the recommendation limits through LimitRange.
- Monitoring: the `VerticalPodAutoscalerDeprecatedUpdateModeAuto` alert, dashboards and PodMonitors.
- Exact recommendation values: they depend on the actual consumption, so cases check only the relations between values.
- Running in CI: the suite follows the runner contract of `testing/e2e/AGENTS.md` (`run` task, `E2E_REPORTS_DIR`, own report name), but the module test runner is not in `main` yet.

## Known module defects

- **The recommender never publishes `status.conditions`.** The case `known-defect-recommendation-conditions` is skipped (`spec.skip: true`) because of it. The cause is in `modules/302-vertical-pod-autoscaler/images/vertical-pod-autoscaler/patches/004-daemonset-scope-node-label.patch:864`: the patch takes the status snapshot with `vpa.AsStatus()` before the conditions are updated, so the stale snapshot is sent. The change that fixes the defect removes `spec.skip: true` from the case and this entry.
