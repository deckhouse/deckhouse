# E2E tests of the admission-policy-engine module

The suites check the chain the unit and gator tests cannot reach:
from the Helm templates and hooks of the module to the Gatekeeper constraints,
the routing of the webhooks and the Rego verdict on a real API server.
The rules of each constraint template are covered by the gator tests in `charts/constraint-templates/tests`,
so every field gets one violating and one compliant object here, not a matrix.

Most probes send a workload with `kubectl create --dry-run=server`.
A dry run goes through every admission webhook and policy but persists nothing,
so a probe is fast and leaves no objects behind.

## Functional suite

The functional suite lives in `tests/` and runs with `task run`.
Each case asserts an admission result, a status field or a metric value with the exact name or message from the module code.

| Case | What it asserts |
|------|-----------------|
| `bootstrap-state` | Every ConstraintTemplate of the module reports `status.created: true`; the webhooks `admission-policy-engine`, `system-namespaces` and `system-namespaces-enforce` and the warn-only constraints `d8-pod-security-<standard>-system` exist; the hook metric `d8_admission_policy_engine_not_bootstrapped` scraped from the Deckhouse leader has no sample equal to 1. |
| `pss-matrix-smoke` | With `security.deckhouse.io/pod-policy` and `security.deckhouse.io/pod-policy-action` on the namespace, Baseline denies a privileged container and admits a root container, Restricted denies a root container, the `warn` action admits a privileged container with a warning, and compliant pods are admitted. |
| `securitypolicy-field-table` | One SecurityPolicy renders a constraint per field. A pod that breaks `allowPrivileged`, `allowPrivilegeEscalation`, `allowHostNetwork`, `allowHostPID`, `allowHostIPC`, `allowedHostPaths`, `allowedCapabilities`, `requiredDropCapabilities`, `runAsUser`, `seccompProfiles`, `readOnlyRootFilesystem` or `automountServiceAccountToken` is denied with the message of the template; compliant pods are admitted; `status.deckhouse.synced` becomes `True`; a policy named `d8-system-default-*` is rejected by the module webhook. |
| `exception-applies` | A SecurityPolicyException admits a host-network pod that Baseline denies, only for the pod labelled `security.deckhouse.io/security-policy-exception` with its name. A pod without the label, a pod referring to a missing exception and a privileged pod with the label stay denied. |
| `system-namespaces-safe` | No controller in a `d8-*` or `kube-*` namespace is blocked by the module. In a system namespace a violation only warns through `d8-pod-security-<standard>-system`; with `security.deckhouse.io/enable-security-policy-check=true` both standards deny. A denying SecurityPolicy that spans all namespaces denies in a user namespace and warns in system namespaces through `d8-system-default-<name>`. |
| `operationpolicy-smoke` | One OperationPolicy denies an image outside `allowedRepos`, a container without a required limit or request, and an image with a disallowed tag; a compliant pod is admitted; a violating Deployment is checked against the cluster's `podSecurityStandards.controllerValidation`, which the suite does not change: denied at the controller level when it is `true` (default), admitted when it is `false`. Only the branch of the current setting runs. |
| `audit-and-metrics` | A warn-only SecurityPolicy admits a violating pod with a warning; within two audit cycles the constraint reports `totalViolations` of at least 1 with the pod in `status.violations`; the constraint exporter reports it in `d8_gatekeeper_exporter_constraint_violations`. |
| `mutation` | An Assign and an AssignMetadata make Deckhouse render the mutating webhook; a pod of the matched namespace gets the label and the grace period, a pod of another namespace does not. The ValidatingAdmissionPolicies reject an Assign targeting `metadata`, an AssignMetadata targeting `spec`, a non-string AssignMetadata value and a mutator without `match.kinds`. |
| `mutator-status` | An Assign and an AssignMetadata report `status.byPod` with `enforced: true`, the way constraints do. |
| `protected-pod-labels` | The ValidatingAdmissionPolicies deny a pod with `security.deckhouse.io/skip-pss-check=true`, and a pod or a pod template with `gatekeeper.sh/operation=webhook` for a user; the Deckhouse service account may set the webhook label. |
| `verify-image-signatures` | `verifyImageSignatures` deploys Ratify and the `D8VerifyImageSignatures` constraint; the agnhost image, which has no signature made with a key generated at runtime, is denied with `Image signature verification failed` (a denial caused by a failed fetch of the image fails the case); an image outside the policy reference is admitted. The `ratify-provider` Provider reports `status.byPod` with `active: true`. |

## Destructive suite

The destructive suite lives in `tests-destructive/` and runs with `task run:destructive`.
It takes the Gatekeeper webhook down, so the pipeline runs it after the functional suites of all modules.
`gatekeeper-unavailable` restores the webhook in `finally` and again in `cleanup`, and waits until admission works.
If the test is killed before that, `tests/common/scripts/restore-gatekeeper.sh` restores the recorded replicas
before the suite namespace is deleted, and again before the next destructive run.
`gatekeeper-rollout-restart` waits in `cleanup` until the Deployment has completed its rollout.

| Case | What it asserts |
|------|-----------------|
| `gatekeeper-unavailable` | With `gatekeeper-controller-manager` scaled to zero, a pod in a user namespace is rejected with `failed calling webhook "admission-policy-engine.deckhouse.io"`, and pods in `d8-system` and `kube-system` are admitted. After the restore, Baseline denies again and compliant pods are admitted. |
| `gatekeeper-rollout-restart` | A rollout restart of the webhook under a dry-run probe sent every 200 ms replaces every pod; in high availability mode no request fails to reach the webhook. |

Deckhouse may put the replicas back if the module converges during the outage.
The case checks the Deployment after the probes and repeats the outage, up to three times, if Deckhouse reverted it in between.

## Known module defects

A check that fails only because of a confirmed module defect is moved into its own test with `spec.skip: true`,
so the rest of the suite stays green. The merge request that fixes the module removes `skip: true`.

No check is skipped because of a module defect at the moment.

## Prerequisites

The suites expect the environment of the e2e pipeline:
an FE cluster built from the branch, a cluster-admin kubeconfig on the first master,
and `kubectl`, `jq`, `curl` and `openssl` in `PATH`.

Each case starts with `tests/common/scripts/ensure-module.sh`.
The script calls `testing/e2e/lib/module-enabled.sh ensure`, which enables the module through its ModuleConfig
when it is not Ready and records the original `spec.enabled` in `E2E_STATE_DIR`.
The script then waits for the Gatekeeper Deployments and the bootstrap of the module.
The deferred command of each task runs `module-enabled.sh restore`, which puts the value back.
No other setting of the cluster is changed.

Some operations are allowed only to the Deckhouse service account, and the scripts impersonate it for them:

- creating a namespace named `d8-*` (the `system-ns.deckhouse.io` policy);
- setting the `heritage: deckhouse` label (the `deny-set-heritage-label.deckhouse.io` and `label-objects.deckhouse.io` policies);
- scaling and restarting `gatekeeper-controller-manager`, whose pod template carries `gatekeeper.sh/operation: webhook`.

All namespaces and cluster-scoped objects of the suites are named `e2e-ape-*`.
multitenancy-manager adopts every new user namespace into a Project and then refuses to delete the namespace
or change its managed labels, `security.deckhouse.io/pod-policy` among them.
For this reason the tasks create the suite namespace `e2e-ape` and pass it to Chainsaw with `--namespace`,
and every test creates its namespaces with `testing/e2e/lib/create-namespace.sh`, with all labels set at creation.
`testing/e2e/lib/delete-namespace.sh` deletes a namespace through its Project, or directly when there is none,
so the suites work with multitenancy-manager enabled or disabled.
The namespace and ModuleConfig helpers are shared by the module suites and live in `testing/e2e/lib`.
The Taskfile finds them through `E2E_LIB_DIR`: the CI bundle sets it, in a checkout it defaults to that directory.
The Taskfile sets `E2E_NAMESPACE_PREFIX=e2e-ape`, so the helpers refuse any other namespace.
The system namespaces of `system-namespaces-safe` must start with `d8-` and are named `d8-e2e-ape-*`.
Before each suite and in a deferred command after it, `tests/common/scripts/cleanup-leftovers.sh` deletes
the namespaces `e2e-ape-*` and `d8-e2e-ape-*` and the cluster-scoped `e2e-ape-*` objects that a killed run left behind.

## How to run

To run the suites on the master of a cluster, copy the `e2e` directory there and run the tasks:

```bash
task -d <E2E_DIR> run
task -d <E2E_DIR> run:destructive
```

The CPU starvation scenarios in `cpu-starvation/` stress a node for 10–15 minutes each,
so neither task above includes them.
Run them on demand with `task -d <E2E_DIR> run:cpu-starvation`,
or in the e2e pipeline with `E2E_MODULE_TESTS_TARGET=run:cpu-starvation`.

JUnit reports are written to `E2E_REPORTS_DIR`, or to `./reports` when the variable is not set.
To check the tests and scripts without a cluster, run `task -d <E2E_DIR> lint`.

## Cases that are not automated

The following planned checks are not automated or are reduced, for the reasons given:

- `denyVulnerableImages` is not tested. The Trivy provider needs the `operator-trivy` module and a populated vulnerability database, which the suite cannot provide in its time budget, and the setting is deprecated in favour of `operator-trivy`.
- High availability is not switched on for `gatekeeper-rollout-restart`. The module takes it from the global `highAvailability` setting, which would change every module of the cluster, and the required pod anti-affinity would leave the second replica Pending on a cluster with a single eligible node. On a single replica the case reports the number of webhook errors instead of requiring zero.
- The Prometheus alerts of the module (`D8AdmissionPolicyEngineWebhookUnavailable` and others) are not checked. They fire after `for: 2m` or longer, and the outage of the destructive case is kept shorter to limit the impact on the cluster.
