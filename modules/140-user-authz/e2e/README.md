# End-to-end tests of the user-authz module

The suite checks the `user-authz` module on a live cluster with [Chainsaw](https://kyverno.github.io/chainsaw/).
It runs in the e2e pipeline on the first master of a fresh cluster (edition FE),
as described in `.gitlab/scripts/e2e-module-tests.sh` and `testing/e2e/run-module-tests.sh`.

## How access is checked

Most cases compare authorization decisions with a reference matrix.
The tests create `SubjectAccessReview` objects with the cluster-admin kubeconfig
instead of calling `kubectl auth can-i --as` for every row:

- the review goes through the same authorizer chain (Node, the user-authz webhook, RBAC);
- the namespace, the resource name and the groups are set explicitly
  (`system:authenticated`, like for an impersonated user);
- `status.denied` tells an explicit deny of the authorization webhook from the "no opinion" of RBAC.

The matrices are the `*.txt` files next to each test, one check per line:
`USER VERB GROUP RESOURCE SUBRESOURCE NAMESPACE EXPECT`.
In the matrices, `-` is an empty cell and `@ns` is the test namespace.
`EXPECT` is `yes`, `no` (not allowed) or `deny` (denied by the authorization webhook).
`tests/common/scripts/sar-check.sh` re-evaluates a matrix until it matches or a timeout expires.
Several cases also cross-check decisions through `kubectl auth can-i --as`.

## Functional cases

The functional suite (`task run`, directory `tests/`) contains the following cases:

| Case | What it asserts |
|------|-----------------|
| `car-access-levels` | One ClusterAuthorizationRule per access level (User … SuperAdmin): the rule is `Ready` with 2 bindings (1 for SuperAdmin), `user-authz:<rule>:<level>` and `user-authz:<rule>:<level>:custom` exist with the controller labels and owner reference, and about 70 decisions match the matrix derived from `templates/cluster-roles.yaml`. |
| `car-lifecycle` | Editor → User → deleted: the bindings are replaced, `status.observedGeneration` follows the generation, decisions change within 60 s, no binding with the prefix `user-authz:<rule>:` is left after the deletion. |
| `car-portforward-scale` | `portForwarding` and `allowScale` add the `port-forward` and `scale` bindings and grant `pods/portforward` and `deployments/scale`, `statefulsets/scale`; switching `portForwarding` off removes only its binding and access. |
| `car-custom-role-aggregation` | A ClusterRole annotated `user-authz.deckhouse.io/access-level: Editor` gets the label from the hook, its rules reach `user-authz:editor:custom` and `user-authz:cluster-admin:custom` but not `user-authz:user:custom`, and the access appears and disappears with the role. |
| `rbacv2-namespace-roles` | RoleBindings to `d8:namespace:viewer`, `user`, `manager`, `admin`, `superadmin`: decision matrix inside and outside the namespace, the `d8:dict:*` ClusterRoleBinding per subject (annotation `rbac.deckhouse.io/subject`) appears, and disappears with the last RoleBinding of the subject. |
| `rbacv2-capability-aggregation` | The capabilities of the module are aggregated into `d8:namespace:viewer`, `d8:namespace:manager`, `d8:subsystem:security:manager` and `d8:system:manager`; a custom namespace capability reaches `d8:namespace:viewer` and `d8:namespace:admin`, grants its rules and leaves with its ClusterRole. |
| `admission-webhooks` | Exact messages of `webhooks/validating/*.py`: CAR subjects, an RE2-incompatible `limitNamespaces`, `limitNamespaces` while multitenancy is off, ClusterRoleBindings to scoped roles and capabilities, reserved `d8:` names, foreign aggregation labels, `heritage: deckhouse`, project-management escalation through a Role, exec into and edits of objects labeled `deckhouse.io/system-resource=true` (denied for Editor, allowed for SuperAdmin). |
| `controller-convergence` | A deleted binding is recreated, edited subjects are restored, an unrenderable rule reports `Ready=False`/`InvalidSpec` with the controller message, and a namespaced AuthorizationRule renders RoleBindings and a dictionary binding and grants access in its namespace only. |
| `mt-limit-namespaces` | Switches `enableMultiTenancy` on, waits for the webhook DaemonSet, permission-browser and every kube-apiserver with `--authorization-config`, then checks `limitNamespaces`, rules without limits and system namespaces, `allowAccessToSystemNamespaces`, `namespaceSelector`, the deny reasons of the webhook, `AccessibleNamespaces`, `BulkSubjectAccessReview` and `WhoCan`; restores the ModuleConfig afterwards. |

## Destructive cases

The destructive suite (`task run:destructive`, directory `tests-destructive/`) runs one case, `mt-webhook-resilience`;
`known-defect-webhook-rollout-gap` is skipped (see below).
It switches the multitenancy mode on once and then performs the following checks:

1. Restarts `user-authz-controller` under a probe of unique-name reviews
   and expects zero false denies, false allows and failed reviews.
1. Stops the controller under the same probe, creates a rule, starts the controller again
   and expects the rule to converge.
1. Makes the webhook DaemonSet unschedulable and expects kube-apiserver to fail closed for a rule subject,
   while the `d8-system:deckhouse` ServiceAccount and the cluster-admin kubeconfig keep working.
   The DaemonSet is restored in the same script and again in `finally`.

The kubeconfig of the runner (`/etc/kubernetes/admin.conf`, user `kubernetes-admin`) is excluded
from the authorization webhook by the `matchConditions` of
`modules/040-control-plane-manager/templates/_authorization_config.tpl`,
so the runner keeps working while the webhook is down.

**Note:** the restart of the webhook DaemonSet is a known module defect and is skipped,
see [Known module defects](#known-module-defects).

The probe classifies every review by the arguments of `probe-during.sh`:

- a review with `evaluationError` (the webhook is unreachable) or a failed request counts against
  `MAX_ERRORS`, the fourth argument;
- a review that is not allowed although it should be, without an evaluation error, counts against
  `MAX_FALSE_DENIES`, the third argument;
- a review that is allowed although it must not be fails the case regardless of the arguments.

Both maximums are 0 in the test.
The probe reports the counts and the length of the false deny window and of the error window,
which is the outage of the webhook as seen by kube-apiserver.

## Known module defects

A check that fails only because of a confirmed module defect is moved into a separate test
with `spec.skip: true`, so that the rest of the suite stays green.
The check itself is not weakened: it fails on the defect again once `skip` is removed.

| Skipped test | Defect | What the module fix has to change |
|--------------|--------|-----------------------------------|
| `tests-destructive/known-defect-webhook-rollout-gap` | A rollout restart of the `user-authz-webhook` DaemonSet makes kube-apiserver deny requests: the webhook runs on the host network, the DaemonSet rolls out with `maxSurge: 0`, kube-apiserver calls only `https://127.0.0.1:40443/` and uses `failurePolicy: Deny`, so nothing answers between the old and the new Pod. | Keep a listener on the node during a rollout, for example a surge rollout with `SO_REUSEPORT` or a hand-over of the socket, and remove `skip: true` from the test in the same merge request. |

## Prerequisites

The suite makes the following assumptions about the cluster:

- `user-authz` is enabled in the Default bundle; if it is not Ready, the Taskfile enables it through ModuleConfig `user-authz`
  with `testing/e2e/lib/module-enabled.sh` and puts `spec.enabled` back after the suite;
- the edition contains the EE part of the module (FE or EE), otherwise the multitenancy cases fail;
- `settings.controlPlaneConfigurator.enabled` of ModuleConfig `user-authz` is not `false`;
- the multitenancy mode is off when the suite starts; if it is on, the suite leaves the ModuleConfig as is
  and `admission-webhooks` checks the "accepted" branch of the multitenancy-only fields;
- `kubernetes-admin` has full access (`kubeadm:cluster-admins` is bound to `cluster-admin`, see
  `forceWildcardClusterAdmin` in `modules/040-control-plane-manager/hooks/reconcile_kubeadm_cluster_admins_binding.go`).

Switching the multitenancy mode restarts kube-apiserver.
The scripts wait for every kube-apiserver Pod to be Ready with or without `--authorization-config`,
so the next suite starts on a settled control plane.
Each switch can take several minutes.

## Namespaces

All tests share one suite namespace, `e2e-authz`.
multitenancy-manager adopts every new namespace into a Project of the same name
and then refuses a direct deletion of the namespace, even for a cluster administrator.
For this reason, the tests do not use the ephemeral namespaces of Chainsaw:

- the Taskfile deletes the leftovers of a killed run with `cleanup-leftovers.sh`,
  then creates `e2e-authz` with `testing/e2e/lib/create-namespace.sh` and passes it with `--namespace`;
- the additional namespaces of `rbacv2-namespace-roles`, `controller-convergence`, `mt-limit-namespaces`
  and `mt-webhook-resilience` are created from script steps with `testing/e2e/lib/create-namespace.sh`,
  which sets every label at creation and recreates a namespace that is still `Terminating` after an earlier run;
- every namespace is deleted with `testing/e2e/lib/delete-namespace.sh`, shared by the module suites,
  which deletes the Project when it exists and the namespace otherwise.

The namespace, ModuleConfig and Deckhouse queue helpers are shared by the module suites and live in `testing/e2e/lib`.
The Taskfile finds them through `E2E_LIB_DIR`: the CI bundle sets it, in a checkout it defaults to that directory.

## Running the suite

The tasks expect `chainsaw`, `task`, `kubectl` and `jq` in `PATH` and a cluster-admin `KUBECONFIG`.
Run them from this directory:

```bash
task run
task run:destructive
task lint
```

JUnit reports are written to `$E2E_REPORTS_DIR` (default `./reports`).
Both suites start and end with `tests/common/scripts/cleanup-leftovers.sh`, which deletes the labeled test objects,
starts `user-authz-controller` again if a killed test left it scaled to zero,
restores ModuleConfig `user-authz` if a test left it changed,
and finally puts back `spec.enabled` with `module-enabled.sh restore`.
When a section fails, the script keeps `E2E_STATE_DIR`, so that a repeated run can finish the restore.

## Cases that are not automated

The following planned cases are not automated:

- Alerts (`D8UserAuthzWebhookRulesQuarantined`, deprecated bindings, foreign aggregation labels):
  the quarantine alert needs a pattern that passes the admission check but fails RE2, and the other alerts
  need Prometheus queries and rule evaluation intervals, which make the result depend on the monitoring stack
  rather than on the module.
- Failure of a whole master in a multi-master cluster: the pipeline cluster may have a single master,
  and stopping it would stop the runner.
