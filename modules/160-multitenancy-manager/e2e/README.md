# Multitenancy-manager e2e tests

This directory holds the Chainsaw e2e suites of the `multitenancy-manager` module.
The GitLab e2e pipeline runs them on a fresh cluster through `testing/e2e/run-module-tests.sh`.

## Covered cases

The functional suite (`tests/`) covers the following cases:

| Case | What it asserts |
|------|-----------------|
| `project-default-lifecycle` | A Project on `default` reaches `Deployed` with all conditions `True`; the namespace gets the project, template, pod security and monitoring labels; `isolated` NetworkPolicy, `d8-project-quota`, `d8-administrators` and its RoleBinding, the `storageclasses` catalog and the Helm release exist; a parameter change is rendered; deletion removes the namespace and the release. |
| `project-secure-enforced` | A Project on `secure` renders the OperationPolicy and the quota; Gatekeeper refuses a privileged pod; the quota refuses a pod over it; `agnhost connect` reaches a `netexec` server from the same project and is blocked from another project. |
| `template-update-propagates` | A change of a custom ProjectTemplate reaches two projects within 60 seconds; afterwards the resourceVersion and Helm revision of both projects stay the same for 45 seconds; a template in use cannot be deleted. |
| `project-delete-cleanup` | A ProjectNamespace adds an additional namespace that inherits labels and the NetworkPolicy; misplaced ProjectNamespaces are refused; the `projects/delete` webhook refuses a user (`--as`) who cannot delete each namespace and names it; deletion removes both namespaces and the release. |
| `webhook-rejections` | The Project, ProjectTemplate and ProjectNamespace checks refuse invalid objects with the exact messages of the code; a Project on a missing template is admitted with a warning and reports `ProjectTemplateFound=False`. |
| `drift-and-protection` | A project administrator cannot delete `d8-administrators`; `system:masters` cannot change a template object or a module-owned namespace label but can set its own label; a quota change made as `system:sudouser` and a removed Project label are reverted. |
| `projectrolebinding-access` | A ProjectRoleBinding fans out into every project namespace, including one added later, and grants nothing outside; wrong roles, foreign ServiceAccounts, misplaced bindings and privilege escalation are refused. |
| `clusterprojectrolebinding-fanout` | A ClusterProjectRoleBinding reaches existing and new projects but not the `default` namespace; deleting it removes the RoleBindings; `cluster-admin` is refused. |
| `cluster-resource-grant` | A ClusterResourceGrantPolicy narrows the catalog; a PVC class is coerced with a warning; a Service class outside the grant is refused, a missing one is defaulted; removing a GrantableClusterResourceDefinition removes its AvailableClusterResource. |
| `namespace-adoption` | Directly created namespaces become projects (`simple`, or `default` for the monitoring label) and cannot be deleted directly; with `allowNamespacesWithoutProjects: false` a user is refused a namespace while `system:masters` is not. |
| `security-policy-in-template` | `allowedUIDs` and `allowedGIDs` of `secure` render a SecurityPolicy; a pod outside the range is refused, a pod inside it is admitted. |
| `helm-to-structured-switch` | A Helm text template moved to structured fields and `manifests` keeps the UIDs of the NetworkPolicy and the ConfigMap; the project ends `Deployed` with no kept objects. |
| `api-conversion` | Project `v1alpha2` and `v1alpha3`, ClusterResourceGrantPolicy `v1alpha2` to `v1alpha1` and ProjectTemplate `v1alpha2` to `v1alpha1` convert as the conversion webhooks describe. |
| `project-name-collision` | A Project named `<project>-<suffix>` is admitted with the webhook warning and gets the `NamedLikeAdditionalNamespace` event. |
| `project-namespace-restored` | The main namespace of a Project deleted directly as `system:sudouser` is recreated with the project labels, and the Project stays `Deployed`. |
| `clusterprojectrolebinding-status` | While one project namespace is held in `Terminating`, a ClusterProjectRoleBinding still reaches the other projects and writes `boundProjects` and `Ready=False` naming that namespace; once the namespace is gone, the controller recreates it, the RoleBinding comes back into it within 2 minutes, and `Ready` no longer names it. |

The destructive suite (`tests-destructive/`) has one case.
In `rollout-restart-under-probe`, the controller Deployment is restarted
while a probe sends dry-run creates of a Project and a ProjectRoleBinding every 200 ms.
The case asserts that no request fails,
that the Helm revision and the generation of an existing project do not change,
and that pods of only one revision remain.
The `PreviousRevisionStillRunning` event is not checked:
the controller sends it only after a 10-minute wait for the old pods.

## Prerequisites

Each test enables the modules it needs with `tests/common/scripts/ensure-module.sh`:
`multitenancy-manager`, `user-authz` and `admission-policy-engine`.
The script calls `testing/e2e/lib/module-enabled.sh ensure`, which records the original `spec.enabled` value.
`tests/common/scripts/module-setting.sh` changes only one setting of a ModuleConfig.
`restore-modules.sh` runs before and after each suite.
It deletes the `e2e-mtm-` objects a killed run left behind and puts the ModuleConfig changes back;
`spec.enabled` is restored with `module-enabled.sh restore-all`.

The tests run in the existing `default` namespace.
Every user namespace is adopted into a Project, and a project namespace cannot be deleted directly.
An ephemeral `chainsaw-*` namespace would therefore fail its own cleanup.
All objects use the `e2e-mtm-` prefix.
The helpers `module-enabled.sh` and `wait-deckhouse-queue.sh` are shared by the module suites and live in `testing/e2e/lib`.
The Taskfile finds them through `E2E_LIB_DIR`: the CI bundle sets it, in a checkout it defaults to that directory.

The tests assume the following about the cluster:

- the CNI enforces NetworkPolicy;
- the `registry.k8s.io/e2e-test-images/agnhost:2.53` image can be pulled;
- the kubeconfig user can impersonate users and groups,
  including `system:masters` and `system:sudouser`.

## How to run

To run the suites, use the following commands:

```shell
task -d modules/160-multitenancy-manager/e2e run
task -d modules/160-multitenancy-manager/e2e run:destructive
task -d modules/160-multitenancy-manager/e2e lint
```

The `run` tasks exit with code 0 when every case passes.
The JUnit reports `multitenancy-manager.xml` and `multitenancy-manager-destructive.xml`
are written to `E2E_REPORTS_DIR`, or to `./reports` when it is not set.
The `lint` task prints `The document is valid` for the configuration and for every test.

## Cases that are not automated

The following planned checks are not automated:

- Firing of the `MultitenancyManagerProjectNamedLikeAdditionalNamespace`
  and `MultitenancyManagerPreviousRevisionStillRunning` alerts.
  The second has `for: 15m`, and the suite has no Prometheus access.
  The tests check the underlying event and pod revisions instead.
- A ClusterProjectRoleBinding with a selector.
  The resource has no selector and always covers every non-virtual project, which the test checks.
- `Ready=True` of a ClusterProjectRoleBinding.
  It also depends on the namespaces of other suites, which are projects too.
  `clusterprojectrolebinding-status` checks only `Ready=False` for a namespace that refuses the binding
  and that the message no longer names the namespace once it is recreated.
