# Admission bypass matrix

Who is *not* checked by each admission point of multitenancy-manager, where the exemption lives, and why.
Two mechanisms exist. **matchConditions** are evaluated by the API server before the request reaches
the webhook or the CEL rules: an exempted request never touches the module. **Handler backstops** are
checks inside the webhook code; they matter for unit tests and for a webhook configuration that has
lost its conditions, and are otherwise redundant with the matchConditions.

`system:masters` is exempted only where the table says so. In every other row a cluster administrator
is policed like any other user; the answer to "why can't I do X as admin" is usually this table.

| Admission point | Object | Skipped at the API server (matchConditions) | Handler backstop | Why the exemption |
|---|---|---|---|---|
| `projects.multitenancy-webhook` (`/validate/v1alpha3/projects`) | Project CREATE/UPDATE | `system:apiserver`, SA `d8-system:deckhouse`, SA `d8-multitenancy-manager:multitenancy-manager`, SA `d8-user-authz:controller`; groups `system:serviceaccounts:d8-system`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-user-authz`, `system:masters`, `system:nodes` (`templates/admission/validation.yaml`, `$excludeSystemWriters`) | none | A module's Helm release and adoption create Projects from system identities; with `failurePolicy: Fail` a denied or slow webhook would lock the module queue. `system:masters` is here because the harness identity of the platform (super-admin.conf) must be able to repair a Project the webhook would refuse. Name-prefix and reserved-name rules therefore do not apply to masters. |
| `projecttemplates.multitenancy-webhook` (`/validate/v1alpha1/templates`) | ProjectTemplate | same `$excludeSystemWriters` | `templatewebhook.Register(..., serviceAccount)` skips the controller SA when it installs the built-in templates | Same reasoning; the controller rewrites the built-in templates on every start. |
| `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces` webhooks | PRB / CPRB / ProjectNamespace | **nobody** | PRB webhook: `isPrivileged` for the controller SA on its own fan-out writes | These objects are written by users; the controller does not create them. `system:masters` is policed: a masters identity gets the same denials as a project admin (e2e `--as T_PRIV` checks mirror this). |
| `/is-granted` (grant guardrail) | any usage object a GrantableClusterResourceReference names | `system:apiserver`, SA `d8-system:deckhouse`, SA `d8-user-authz:controller`, SA `d8-multitenancy-manager:multitenancy-manager`; groups `system:serviceaccounts:d8-system`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-user-authz`, `system:masters`, `system:nodes` (`hooks/configure_grant_validation_webhook.go`, `systemWriterMatchConditions`) | `isAutomatedSystemWriter`: groups `system:nodes`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system`, `system:serviceaccounts:d8-user-authz` — **no** `system:masters`, no usernames (`internal/webhooks/is_granted.go`) | Module Helm releases and user-authz fan-out land RoleBindings and other governed objects in project namespaces; a denial would lock a module. The backstop is narrower than the conditions on purpose: a direct handler call (unit tests) still polices a cluster-admin. |
| `/defaults` (mutating: FillEmpty / Coerce) | same | same `systemWriterMatchConditions` | `isSystemRequest`: usernames + groups incl. `system:masters` (`internal/webhooks/protect.go`) | Same as `/is-granted`; the backstop is the wider one because a mutation applied to a system writer's object would be undone by its owner on the next apply. |
| `/protect` (read-only catalog) | AvailableClusterResource | same `systemWriterMatchConditions` | `isSystemRequest` + the controller SA | The catalog is the controller's; since spec 004 it also carries `heritage: multitenancy-manager` and the heritage policy below refuses it fail-closed even when the webhook is down. |
| VAP `d8-multitenancy-manager` (heritage protection) | any object with `heritage: multitenancy-manager`, Namespace included | groups `system:nodes`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system`; users `system:sudouser`, `system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability`; a `rollout restart` (restartedAt annotation) (`templates/validation.yaml`) | n/a (CEL) | Only cluster components. **`system:masters` is not exempt**: a template-owned object is changed through the ProjectTemplate or the Project. On a project **Namespace**, an UPDATE that touches only labels and annotations outside the module-owned keys is allowed to anyone with RBAC (spec 004, Q29). |
| VAP `d8-multitenancy-manager-namespace-ownership` | Namespace CREATE/UPDATE setting or changing `projects.deckhouse.io/project` | SA `d8-multitenancy-manager:multitenancy-manager`, SA `d8-system:deckhouse` | n/a (CEL) | The ownership label is what the controller reads to decide whose namespace it is; a hand-made one was neither adopted nor protected (spec 004, Q13c). **`system:masters` is not exempt.** Removing the label is not refused here. |
| user-authz `system_resources` webhook (edit/exec on system pods, heritage objects) | Pods, module objects | see `modules/140-user-authz/docs/internal/WEBHOOK_BYPASS_MATRIX.md` | — | Owned by user-authz; listed here because it is the other guard around project namespaces that *does* exempt cluster administrators (superadmin roles, `cluster-admin`). |

## Reading the table

- A denial from a webhook names the webhook (`admission webhook "…" denied the request`); a denial
  from a policy names the policy (`ValidatingAdmissionPolicy '…' with binding '…' denied request`).
- Tests that need a denial from a masters-skipped webhook impersonate a non-masters cluster-admin
  (`e2e-priv@example.com`, bound to `cluster-admin`) — the two admission policies deny that identity
  as well.
- When you add an exemption, add it in the matchConditions (that is what makes a module unlockable)
  and, only if the handler must behave the same in isolation, in the backstop; then update this table.
