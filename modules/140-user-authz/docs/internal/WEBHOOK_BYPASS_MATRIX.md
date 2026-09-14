# Webhook bypass matrix

Which identities each validating webhook of user-authz does **not** check, where the exemption is
enforced, and why. Two mechanisms exist. **matchConditions** are evaluated by the API server before
the request reaches the webhook-handler: an exempted request never touches the module, and the
exemption holds while the handler is down — which is what makes a fail-closed webhook safe for the
components that must be able to repair the cluster. **Body checks** live in the hook code; they matter
for the unit tests and for a configuration that lost its conditions, and are otherwise redundant with
the matchConditions.

Cluster administrators are exempt only where the table says so. Elsewhere `system:masters` is policed
like any user; the e2e suite impersonates a non-masters cluster-admin (`e2e-priv@example.com`) where
a denial from a masters-skipping webhook is under test.

| Webhook (binding) | Object | Skipped at the API server (matchConditions) | Body check | Why |
|---|---|---|---|---|
| `rbacv2-cluster-roles.deckhouse.io` (`cluster_roles.py`) | ClusterRole CREATE/UPDATE — `d8:` names reserved, kind labels, custom-role rules, lineage | `system:apiserver`, SA `d8-system:deckhouse`, SA `kube-system:clusterrole-aggregation-controller` | none | The chart and the aggregation controller write the model's roles; nobody else may claim a `d8:` name. Masters is policed. |
| `role-validating.deckhouse.io` (`role_binding.py`) | ClusterRoleBinding CREATE — no CRB on tenant (`d8:namespace:*`, `d8:project:*`) roles | `system:apiserver`, SA `d8-system:deckhouse`, SA `d8-user-authz:controller` | `PRIVILEGED_USERS` = the same three | user-authz-controller materialises the rule bindings; a tenant role bound cluster-wide would leak out of its namespaces. Masters is policed. |
| `rbacv2-system-resource-edit.deckhouse.io` (`system_resources.py`) | UPDATE/DELETE of objects labelled `deckhouse.io/system-resource` or `heritage: multitenancy-manager` | `system:apiserver`, SA `d8-system:deckhouse`, SA `kube-system:clusterrole-aggregation-controller`, SA `d8-multitenancy-manager:multitenancy-manager`; groups `system:masters`, `kubeadm:cluster-admins`, `superadmins`, `system:sudousers`, `system:nodes`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system` | `PRIVILEGED_USERS` + `BYPASS_GROUPS` (same sets) + holders of `PRIVILEGED_ROLES` (`d8:{namespace,project,system}:superadmin`, `cluster-admin`, `user-authz:super-admin`) for system resources; heritage objects are refused even to them | Break-glass for administrators and components; the multitenancy-manager admission policy still protects heritage objects from administrators independently of this webhook. |
| `rbacv2-system-resource-exec.deckhouse.io` (`system_resources.py`) | CONNECT `pods/exec`, `pods/attach`, `pods/portforward` on a system pod | the same four users and the same groups as the edit binding (the two service accounts were added by spec 004) | same as the edit binding; the denial names every role of `PRIVILEGED_ROLES` and nothing else | Exec matches every pod of the cluster; a handler that is down must not stand between a component or an administrator and a pod. |
| `rbacv2-role-escalation.deckhouse.io` (`role_escalation.py`) | Role/ClusterRole CREATE/UPDATE — mutating verbs on project-management kinds | `system:apiserver`, SA `d8-system:deckhouse`, SA `kube-system:clusterrole-aggregation-controller`; groups `system:masters`, `kubeadm:cluster-admins` | `PRIVILEGED_USERS` + `BYPASS_GROUPS` (same) | A cluster administrator may author any role; a project admin may not grant themselves project management through a custom role. |
| `d8-user-authz-identity-assign.deckhouse.io` (`identity_privilege.py`, logic in `identity_assign.py`) | user-authn User CREATE/UPDATE/DELETE — an identity may not be assigned privileges above its assigner's | `system:apiserver`, `system:sudouser`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability`, SA `d8-system:deckhouse`; group `system:masters` | `EXEMPT_USERS` (the same plus SA `kube-system:clusterrole-aggregation-controller`) and `EXEMPT_GROUPS` (`system:masters`, `system:serviceaccounts:kube-system`) | Components and the cluster administrator manage identities; everyone else is bounded by their own privilege. |
| `d8-user-authz-group-authorization-rule-collision.deckhouse.io`, `d8-user-authz-user-authorization-rule-collision.deckhouse.io` (`identity_collision.py`) | user-authn Group / User CREATE/UPDATE/DELETE — a name that a ClusterAuthorizationRule or AuthorizationRule already binds | `system:apiserver`, `system:sudouser`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability`, SA `d8-system:deckhouse` | `EXEMPT_USERS` (the same plus SA `d8-commander:cluster-manager`) and `EXEMPT_GROUPS` (`system:masters`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system`) | Installers and the cluster administrator seed identities; the collision guard exists for self-service. **Discrepancy**: masters is exempt in the body but not in the matchConditions, so with the handler down a masters request is refused (fail-closed) although the body would let it through. |
| `d8-user-authz-car-multitenancy-related-options.deckhouse.io`, `d8-user-authz-module-multitenancy-related-options.deckhouse.io` (`multitenancy.py`) | ClusterAuthorizationRule / ModuleConfig CREATE/UPDATE — EE-only multitenancy fields, `enableMultiTenancy` gating | none (the module binding is narrowed to `request.name == "user-authz"`) | none | Configuration validation applies to everyone, masters included: an EE-only field on a CE cluster is wrong whoever writes it. |

## Related guards outside this module

- multitenancy-manager's admission points (Project/ProjectTemplate webhooks, grant webhooks, the two
  admission policies) have their own table: `modules/160-multitenancy-manager/docs/internal/ADMISSION_BYPASS_MATRIX.md`.
- The EE authorizer (`ee/be/.../authorizer/multitenancy/engine.go`) treats `system:masters`,
  `kubeadm:cluster-admins` and `system:sudousers` as privileged for the AccessibleNamespaces answer —
  the same three groups the system-resource webhook exempts, plus `superadmins`, which only the
  webhook lists. The two lists are meant to agree; the difference is recorded here until one of them
  is changed on purpose.

## Maintaining the table

- Add an exemption in the matchConditions first (that is what keeps a module unlockable), in the body
  only if the handler must behave the same in isolation, then update this table and, for the
  system-resource webhook, the config contract tests in `system_resources_test.py`.
- A denial names the webhook (`admission webhook "…" denied the request`); the wording of the
  system-resource denials is derived from `PRIVILEGED_ROLES`, so the roles it points at are always
  the roles the check accepts.
