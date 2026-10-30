---
title: "The user-authz module: FAQ"
description: "Answers to common questions about user-authz: limiting access, combining the role models, custom roles, migrating to the new label scheme, and troubleshooting the controller and the authorization webhook."
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

<div style="height: 0;" id="how-do-i-limit-user-rights-to-specific-namespaces-obsolete-role-based-model"></div>

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces in the granular role-based model, use RoleBinding with the [namespace role](./#namespace-roles) that has the appropriate level of access. See the [example of assigning administrative rights to a user within a namespace](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

In the basic role-based model, use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule) CR. These parameters require the [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy) parameter: while it is disabled, a ClusterAuthorizationRule with them is refused. A rule that already has them is applied without the limits, see [Why is a rule reported as needing multi-tenancy?](#why-is-a-rule-reported-as-needing-multi-tenancy)

## What if there are two ClusterAuthorizationRules matching a single user?

In the example, the user `jane.doe@example.com` is in the `administrators` group. There are two cluster authorization rules:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: jane
spec:
  subjects:
    - kind: User
      name: jane.doe@example.com
  accessLevel: User
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: Group
    name: administrators
  accessLevel: ClusterAdmin
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: env
        operator: In
        values:
        - prod
        - stage
```

1. `jane.doe@example.com` has the right to get and list any objects in the namespaces labeled `env=review`
2. `Administrators` can get, edit, list, and delete objects at the cluster level and in the namespaces labeled `env=prod` and `env=stage`.

Because `Jane Doe` matches two rules, the resulting permissions are calculated as follows:

* `Jane Doe` will have the highest accessLevel across all matching rules — `ClusterAdmin`.
* The `namespaceSelector` options will be combined, so that Jane will have access to all the namespaces with the `env` label set to one of the following values: `review`, `stage`, or `prod`.

{% alert level="warning" %}
If there is a rule without the `namespaceSelector` option and `limitNamespaces` deprecated option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.
{% endalert %}

## Can the basic and the granular role-based models be used at the same time?

Yes. Both models ultimately boil down to the standard Kubernetes RBAC mechanism, and RBAC is a permissive model: permissions from all sources are **summed up**. If an action is allowed by at least one source — a ClusterAuthorizationRule, an AuthorizationRule, a RoleBinding to a granular-model role, or a ProjectRoleBinding — it will be allowed. Nothing needs to be "switched over": you can keep the existing ClusterAuthorizationRule objects and gradually add granular-model role bindings.

This holds in the multitenancy mode ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) as well, with one rule about levels. The namespace restriction of a ClusterAuthorizationRule (`limitNamespaces` or `namespaceSelector`) limits the permissions of that rule only: in a namespace outside the list the user keeps whatever a RoleBinding, an AuthorizationRule or a ProjectRoleBinding grants there, at the level of that binding — the `accessLevel` of the ClusterAuthorizationRule does not leak into it. A namespace where the user has no source at all stays denied. See [the module description](./#using-clusterauthorizationrule-and-authorizationrule-together-with-rbac) for the full set of sources and an example.

## How do I get an equivalent of the ClusterAdmin and SuperAdmin roles in the granular model?

There is no single-object counterpart of the basic model's `ClusterAdmin` and `SuperAdmin` roles in the granular model — it deliberately separates platform administration (system roles) from application access (namespace and project roles). The equivalent is assembled from **two bindings**: a ClusterRoleBinding to a system role and a [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) to a project role (the latter applies in all projects, including those created later).

Approximate level mapping:

| Basic model role | Granular model equivalent |
|--------------------|-------------------------------|
| `User` | `d8:namespace:viewer` (via RoleBinding or ProjectRoleBinding). |
| `PrivilegedUser` | `d8:namespace:user`. |
| `Editor` | `d8:namespace:manager` — the same level; the scope differs: the granular role is bound per namespace (RoleBinding) or per project (ProjectRoleBinding), not cluster-wide with a namespace filter. |
| `Admin` | `d8:namespace:admin`. |
| `ClusterEditor` | No direct counterpart. `ClusterEditor` is the `Editor` level in every namespace, system ones included, plus cluster-scoped objects. Assemble it from a ClusterProjectRoleBinding to `d8:project:manager` (all projects) and a system role for the platform part; `d8:system:manager` alone is not an equivalent: it gives no access to the user namespaces. |
| `ClusterAdmin` | `d8:system:manager` + a ClusterProjectRoleBinding to `d8:project:admin`. |
| `SuperAdmin` | `d8:system:superadmin` + a ClusterProjectRoleBinding to `d8:project:superadmin`. |

An example for `ClusterAdmin` (the `k8s-admins` group):

```yaml
# Platform: Deckhouse Platform module configuration, cluster-wide resources, system namespaces.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-admins-platform
subjects:
  - kind: Group
    name: k8s-admins
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
---
# Applications: administrator in all namespaces of all projects (including future ones).
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: k8s-admins-projects
spec:
  subjects:
    - kind: Group
      name: k8s-admins
  roleRef:
    kind: ClusterRole
    name: d8:project:admin
```

For `SuperAdmin`, replace the roles with `d8:system:superadmin` and `d8:project:superadmin`.

Specifics:

- With the `multitenancy-manager` module enabled, [every user namespace is a project](/modules/multitenancy-manager/#automatic-project-creation-for-namespaces), so the "system role + ClusterProjectRoleBinding" pair covers both the platform and all user namespaces. Only the `default` namespace is not covered — it is neither a project nor a system namespace.
- You cannot create a custom "all permissions" role (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`): such a role gives, among other things, project management permissions and is rejected by the [built-in protections](./#built-in-protection-mechanisms-of-the-role-model). If you need truly unrestricted access to the whole API (outside the platform role model), use a ClusterRoleBinding to the built-in Kubernetes `cluster-admin` role — only someone who already has such permissions can assign it.

## How do I grant a user access to the resources of one module only?

A typical request: a user in a namespace should only work with the resources of one module (for example, only with virtual machines) without seeing the other resources (Pod, Deployment, etc.).

Every Deckhouse Platform module ships separate capabilities for its resources, so such access is granted without writing RBAC rules. Assemble a [custom role](#creating-a-custom-namespace-or-project-role) that aggregates only the capabilities of the desired module. Select each capability by its `rbac.deckhouse.io/capability` label: a role with the `rbac.deckhouse.io/delegatable: "true"` label may select capabilities only by this label or by the `rbac.deckhouse.io/aggregate-to-namespace-as` and `rbac.deckhouse.io/aggregate-to-project-as` labels, and a selector by `module` or by any other label is refused. To list the capabilities of the module, run:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability,module=virtualization,rbac.deckhouse.io/scope=namespace -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

An example of the role, where `<ACTION>` is to be replaced with the values from the list:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-only
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Allows using the role in a RoleBinding inside projects.
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
rules: []
```

Values without the `custom.` prefix are reserved for the capabilities Deckhouse ships, and a ClusterRole a user writes with such a value is refused, so these selectors pick only the capabilities of the module.

Grant the role via a RoleBinding in the desired namespace or via a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) across the whole project. The user will get access only to the module's resources — the standard Kubernetes resources will not be visible to them.

{% alert level="info" %}
Do not bind the capability of the module directly. Inside **project** namespaces a plain RoleBinding may only reference roles [available to the project](/modules/multitenancy-manager/usage.html#roles-available-in-a-rolebinding-inside-a-project), and capabilities are not among them by default. The custom role above has the `rbac.deckhouse.io/delegatable: "true"` label, which makes it available there, so the same role works in every namespace.
{% endalert %}

## What happens to the bindings when the cluster is updated to the release with the controller?

Nothing is recreated and access is not interrupted. In Deckhouse Platform 1.78, the `user-authz-controller` component adopts the bindings that the Helm chart used to render: the same objects with the same names, with an `ownerReference` to the rule added.

Plan for the release that performs the migration: it processes every existing binding once, so it takes longer than usual. Subsequent releases are not affected.

| Bindings in the release | What to expect |
|---|---|
| Up to 5000 | The release runs longer than usual and completes on its own |
| More than 5000 | The release can exceed the 20-minute module timeout and fail |

Count the bindings before you update. To get the number of cluster role bindings, use the following command:

```bash
d8 k get clusterrolebindings -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

To get the number of namespaced bindings, use the following command:

```bash
d8 k get rolebindings -A -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

If there are more than 5000, raise the release timeout or switch to the previous release engine for the duration of the update, and revert the setting afterwards:

To increase the release timeout, use the following command:

```bash
d8 k -n d8-system set env deploy/deckhouse HELM_TIMEOUT=60m
```

To switch back to the previous engine, use the following command:

```bash
d8 k -n d8-system set env deploy/deckhouse USE_NELM=false
```

If the migration cannot prepare every binding, the release does not start and the bindings stay as they were.

After the migration, the release no longer grows with the number of rules. The `D8UserAuthzReleaseSlow` alert fires when applying the module release repeatedly takes more than 5 minutes, and `D8UserAuthzRenderSlow` fires when rendering its templates repeatedly takes more than 30 seconds. Both point at the API server or at a Deckhouse Pod short of CPU rather than at the rules.

## How do I check that the controller keeps the bindings in sync?

The `user-authz-controller` component reconciles the ClusterRoleBinding and RoleBinding objects of every ClusterAuthorizationRule and AuthorizationRule, the `d8:dict` ClusterRoleBindings of the granular model, and the RoleBinding objects that give the subjects of system and subsystem roles the namespace role named by `rbac.deckhouse.io/use-role` in module namespaces. Its state is shown by the object status, the metrics, and the alerts.

**Object status.** Every rule carries a `Ready` condition and the number of its bindings:

To list all ClusterAuthorizationRules in the cluster, run:

```bash
d8 k get clusterauthorizationrules
```

Example output:

```console
NAME        ACCESS LEVEL   READY   BINDINGS   AGE
my-rule     Admin          True    3          5d
```

To list all AuthorizationRules in all namespaces of the cluster, run:

```bash
d8 k get authorizationrules -A
```

Example output:

```console
NAMESPACE   NAME       ACCESS LEVEL   READY   BINDINGS   AGE
default     app-rule   Editor         True    2          3d
team-a      dev-rule   User           False   0          1h
```

To get only the conditions of a specific ClusterAuthorizationRule, run:

```bash
d8 k get clusterauthorizationrule <name> -o jsonpath='{.status.conditions}'
```

Example output:

```json
[
  {
    "lastTransitionTime": "2026-09-11T10:00:00Z",
    "message": "3 bindings applied",
    "observedGeneration": 1,
    "reason": "BindingsApplied",
    "status": "True",
    "type": "Ready"
  }
]
```

`Ready=True` with the reason `BindingsApplied` means the bindings match the rule. `Ready=False` indicates the problem: `InvalidSpec` (the rule cannot be rendered into bindings; the message says why) or `ApplyError` (the API server rejected a write; the message names the binding and the error). Events with the same reasons are recorded on the rule.

**Metrics.** The controller exposes them on the `metrics` port of its Pod and the `PodMonitor` `user-authz-controller` collects them (the `operator-prometheus` module must be enabled). The `kind` label is `ClusterAuthorizationRule`, `AuthorizationRule`, `dict` or `manage`.

| Metric | Description |
|---|---|
| `d8_user_authz_authorization_rules{kind}` | Objects of the kind known to the controller |
| `d8_user_authz_bindings_desired{kind}` | Bindings the objects of the kind must have |
| `d8_user_authz_bindings_actual{kind}` | Bindings of the kind that exist |
| `d8_user_authz_bindings_drift{kind,reason}` | Bindings not in the desired state at the last reconcile; `reason` is `missing`, `extra` or `changed`. Stays above zero only while the controller fails to converge |
| `d8_user_authz_bindings_apply_total{kind,op,result}` | Write operations issued by the controller (`op`: `create`, `update`, `delete`; `result`: `success`, `error`) |
| `d8_user_authz_authorization_rules_invalid{kind,reason}` | Rules whose `Ready` condition is `False`, by kind and reason |
| `d8_user_authz_authorization_rule_invalid{kind,name,rule_namespace,reason}` | `1` for every such rule (at most 50 rules are named; the aggregate above is always complete) |
| `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` | Standard controller-runtime metrics per reconciler (`controller` label: `clusterauthorizationrule-bindings`, `authorizationrule-bindings`, `dict-bindings`, `manage-bindings`) |

Deckhouse exports the following metrics of the module hooks:

| Metric | Description |
|---|---|
| `d8_user_authz_custom_cluster_roles{level}` | Custom `ClusterRole` objects (annotated with `user-authz.deckhouse.io/access-level`) per access level |
| `d8_user_authz_custom_aggregation_missing{level}` | `1` when the aggregated ClusterRole `user-authz:<level>:custom` has no rules although custom roles it must aggregate exist |
| `d8_user_authz_bindings_keep_stamped_total`, `d8_user_authz_keep_stamp_duration_seconds` | Work of the migration hook that protects chart-rendered bindings from being pruned by the release: objects stamped and the duration of its last run |

**Alerts.** The module ships the following alerts for the controller:

- `D8UserAuthzControllerUnavailable` and `D8UserAuthzControllerTargetDown` when the controller has unavailable replicas or is not scraped for 5 minutes.
- `D8UserAuthzControllerReconcileErrorsHigh` on a sustained reconcile error rate.
- `D8UserAuthzBindingsDrift` when bindings of some kind stay out of the desired state for 15 minutes.
- `D8UserAuthzAuthorizationRulesInvalid` and `D8UserAuthzAuthorizationRuleInvalid` counting and naming the rules whose bindings cannot be applied for 10 minutes.
- `D8UserAuthzCustomRolesNotAggregated` when the aggregated ClusterRole of an access level stays empty although custom roles for it exist.

When a rule stays `Ready=False` with `ApplyError` because a binding's `roleRef` was changed by hand, delete that binding: `roleRef` is immutable and the controller recreates the binding with the right one.

## How do I check that the authorization webhook has the current rules?

The authorization webhook and Permission Browser read the multi-tenancy options of ClusterAuthorizationRules (`limitNamespaces`, `namespaceSelector`, `allowAccessToSystemNamespaces`) from the API, so a change reaches them within a few seconds.

To check that the webhook Pods are running, run:

```bash
d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
```

Example output (one Pod per master, on the host network; both containers must be ready):

```console
NAME                       READY   STATUS    RESTARTS   AGE   IP           NODE       NOMINATED NODE   READINESS GATES
user-authz-webhook-qrm2n   2/2     Running   0          2d    10.0.0.11    master-0   <none>           <none>
user-authz-webhook-h6jbh   2/2     Running   0          2d    10.0.0.12    master-1   <none>           <none>
user-authz-webhook-97fvs   2/2     Running   0          2d    10.0.0.13    master-2   <none>           <none>
```

To see the last 100 log lines of the webhook container of every such Pod, run:

```bash
d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
```

Example output:

```console
2026/09/11 10:00:00 server is starting to listen on  127.0.0.1:40443 ...
2026/09/11 10:00:01 rules source: directory rebuilt from 12 rules (18 subjects, 0 quarantined) in 4.2ms
```

The `rules source: directory rebuilt from N rules` line shows the informer has listed the rules and how many it holds; `quarantined` counts the rules the instance could not fully use: a `limitNamespaces` pattern or a `namespaceSelector` that does not compile, or a rule that cannot be read.

An instance that has not read the rules yet is not ready and answers authorization requests with an error instead of a denial. In the seconds between a rule being created and the webhook observing it, the subjects of that rule that no observed rule covers yet are denied access to all namespaces. A subject that another rule already covers keeps the scope of that rule.

The webhook exposes metrics through a `kube-rbac-proxy` sidecar; the PodMonitor `user-authz-webhook` collects them, and the `operator-prometheus` module must be enabled. There is one series per master node.

| Metric | Description |
|---|---|
| `user_authz_webhook_rules_informer_synced` | `1` once the instance has read the ClusterAuthorizationRules at least once |
| `user_authz_webhook_rules_observed` | Rules the instance currently uses |
| `user_authz_webhook_rules_subjects` | Distinct subjects in those rules |
| `user_authz_webhook_rules_max_resource_version` | Highest `resourceVersion` among the observed rules. Compare it with the cluster to measure the lag |
| `user_authz_webhook_rules_quarantined` | Rules the instance could not fully use: a `limitNamespaces` pattern or `namespaceSelector` that does not compile, or a rule that cannot be read at all |
| `user_authz_webhook_rules_directory_updated_timestamp_seconds` | Time of the last rules update |
| `user_authz_webhook_rules_directory_rebuilds_total`, `user_authz_webhook_rules_directory_rebuild_duration_seconds` | Number of rules updates and their duration |
| `user_authz_webhook_rules_watch_errors_total` | List and watch errors of the rules informer |

Permission Browser exposes the same metrics with the `user_authz_permission_browser` prefix; the PodMonitor `permission-browser-apiserver` collects them.

The module ships the following alerts for both components:

| Alert | Fires when |
|---|---|
| `D8UserAuthzWebhookTargetDown` | Prometheus has not scraped at least one webhook instance for 5 minutes |
| `D8UserAuthzWebhookRulesQuarantined` | A rule has not compiled for 10 minutes |
| `D8UserAuthzWebhookRulesWatchErrors` | The rules informer has been failing to watch the rules for 10 minutes |
| `D8UserAuthzWebhookDirectoryDiverged` | Instances have been using different sets of rules for 10 minutes, so the same request is answered differently depending on the master node that receives it |
| `D8UserAuthzRulePropagationLag` | One instance has not updated its rules for an hour while another one has |
| `D8UserAuthzPermissionBrowserUnavailable` | Permission Browser has unavailable replicas |

## Why does a change to a ClusterAuthorizationRule take up to 30 seconds to take effect?

The API server caches the answers of the authorization webhook. In the `AuthorizationConfiguration` that `control-plane-manager` renders, the webhook has `authorizedTTL: 5m`, `unauthorizedTTL: 30s` and `timeout: 3s`. The cache key is the whole SubjectAccessReview, so an identical repeated request is answered from the cache.

The webhook never allows a request: it either denies it or gives no opinion, and both answers are cached for `unauthorizedTTL`, which is 30 seconds. Creating a rule or narrowing an existing one therefore takes effect for a particular request within 30 seconds of the last identical request:

```bash
d8 k auth can-i --as=user@example.com get pods -n other-namespace
sleep 30
d8 k auth can-i --as=user@example.com get pods -n other-namespace
```

The cache cannot be flushed without restarting `kube-apiserver`.

For the resource of a CRD installed a moment ago, the delay reaches 40 seconds. The webhook learns whether a resource is namespaced from discovery, which it queries no more often than once every 10 seconds, and the API server then caches the answer for another 30 seconds.

During those seconds the namespace limits do not apply to that resource: the webhook does not know it exists, gives no opinion, and RBAC alone answers — so a subject whose rule limits it to some namespaces can list the new resource across the whole cluster. Installing a CRD requires far more privilege than that yields, which is why discovery is queried at a bounded rate rather than on every request. Permission Browser has a window of the same order for its own report, up to 30 seconds.

## Why is a rule reported as needing multi-tenancy?

The `limitNamespaces`, `namespaceSelector` and `allowAccessToSystemNamespaces` options are enforced by the authorization webhook, which is deployed only when [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy) is enabled. A rule that uses these options while multi-tenancy is disabled is applied without them: the subjects named in the rule hold its access level in **every** namespace of the cluster, including the system ones.

While multi-tenancy is disabled, the `d8-user-authz-car-multitenancy-related-options.deckhouse.io` admission webhook refuses a ClusterAuthorizationRule with any of these options, even `allowAccessToSystemNamespaces: false`. It also refuses disabling `enableMultiTenancy` while a rule uses them. So a rule reaches this state only if it was written before these checks applied, for example before an update. Any change to such a rule must also remove the options.

Only options that ask for a limit are counted. A rule with `allowAccessToSystemNamespaces: false`, or with an empty `limitNamespaces` list, is not reported.

Two metrics report the situation:

| Metric | Description |
|---|---|
| `d8_user_authz_rule_needs_multitenancy{name,options}` | One series per affected rule, up to 50, with the options that will not take effect |
| `d8_user_authz_rules_needing_multitenancy` | Total number of affected rules, including those past the first 50 |

An individual rule is listed in the `D8UserAuthzRuleNeedsMultiTenancy` alert, and `D8UserAuthzRulesNeedMultiTenancy` fires when there are more affected rules than the first alert lists. Both belong to the `D8UserAuthzMisconfigured` group.

To resolve the situation, either enable multi-tenancy, or remove the options from the rules.

To check whether multi-tenancy is enabled, use the following command:

```bash
d8 k get moduleconfig user-authz -o jsonpath='{.spec.settings.enableMultiTenancy}'
```

To retrieve the names of ClusterAuthorizationRules that restrict access by namespace or allow access to system namespaces, use the following command:

```bash
d8 k get clusterauthorizationrule -o json | jq -r '.items[] | select((.spec.limitNamespaces // [] | length > 0) or (.spec.namespaceSelector != null) or (.spec.allowAccessToSystemNamespaces == true)) | .metadata.name'
```

## How do I extend a role or create a new one?

[The granular role model](./#granular-role-based-model) is based on the aggregation principle; it compiles smaller roles into larger ones,
thus providing easy ways to enhance the model with custom roles.

### Creating a new role subsystem

Suppose that the current subsystems do not fit the role distribution in the company. You need to create a new [subsystem](./#subsystems-of-the-role-based-model)
that includes roles from the `cluster` subsystem, the `network` subsystem and the user-authn module.

To meet this need, create the following role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: viewer
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-network-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

The labels for the new role listed at the top suggest that:

- The namespace role `user-authz-controller` gives the subjects of the role in the module namespaces; a custom role is projected as `viewer` at most:

  ```yaml
  rbac.deckhouse.io/use-role: viewer
  ```

- The role is a custom role (custom roles never define their own rules, they only aggregate capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  {% alert level="info" %}
  This label is mandatory.
  {% endalert %}

- The role is a subsystem one, and it shall be handled accordingly:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- There is a subsystem for which the role is responsible:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

Then there are selectors that implement aggregation:

- This one aggregates the manager role from the `cluster` subsystem:

  ```yaml
  rbac.deckhouse.io/aggregate-to-cluster-as: manager
  ```

- This one aggregates all the system-scope capabilities defined for the `user-authn` module:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

This way, your role will combine the permissions of the `cluster` subsystem, `network` subsystem, and the `user-authn` module.

Notes:

* Custom roles and capabilities must be named with the `d8:custom:` prefix (the rest of the `d8:` prefix space is reserved for Deckhouse Platform built-in objects). The name must agree with the declared scope: a subsystem role is `d8:custom:<subsystem>:<name>` (the segment is the subsystem itself, as in the example above), a namespace or project role is `d8:custom:namespace:<name>` or `d8:custom:project:<name>`, and a capability is `d8:custom:<scope>-capability:<name>`. A name that disagrees with the `rbac.deckhouse.io/scope` label is rejected.
* RoleBindings with a namespace role (`d8:namespace:<level>`) will be created in the namespaces of the aggregated subsystems' modules, the level is specified by the `rbac.deckhouse.io/use-role` label, `viewer` at most for a custom role. Without the label no such RoleBinding is created.
* A custom role cannot be aggregated into `d8:system:<level>`, because only a custom capability may carry the `rbac.deckhouse.io/aggregate-to-system-as` label. To give the role to the holders of `d8:system:manager`, bind it to them with a ClusterRoleBinding.

### Extending the custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a manage role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource.

First, you have to add a new selector to the role:

```yaml
rbac.deckhouse.io/aggregate-to-mycustom-as: manager
```

This selector would enable capabilities to be aggregated to a new subsystem by specifying this label. After adding the new selector, the role will look as follows:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: d8:custom:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: viewer
     rbac.deckhouse.io/kind: custom-role
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-cluster-as: manager
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-network-as: manager
     - matchLabels:
         rbac.deckhouse.io/scope: system
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-mycustom-as: manager
 rules: []
 ```

 Next, you need to create a new capability and define permissions for the new resource, e. g., the read-only permission:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-mycustom-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/capability: "custom.subsystem-capability.mycustom.superresource_view"
   name: d8:custom:subsystem-capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

The capability will add its permissions to the subsystem role. so that holders of the role will be able to view the new object.

Notes:

* Name a custom capability `d8:custom:<SCOPE>-capability:<NAME>`, where `<SCOPE>` is the value of its `rbac.deckhouse.io/scope` label. A name that disagrees with the label is rejected.

### Extending the existing subsystem roles

To extend an existing role, follow the procedure outlined in the section above. Be sure to change the labels and the role name!

For example, here's how you can extend the manager role from the `cluster` (`d8:subsystem:cluster:manager`) subsystem:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-cluster-as: manager
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: cluster
    rbac.deckhouse.io/capability: "custom.subsystem-capability.cluster.superresource_view"
  name: d8:custom:subsystem-capability:cluster:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

This way, the new capability will extend the `d8:subsystem:cluster:manager` role.

### Extending subsystem roles and adding a new namespace

The namespaces where the holders of a system or subsystem role get a RoleBinding with the namespace role are the namespaces of its modules: `user-authz-controller` takes them from the `rbac.deckhouse.io/namespace` label of the system capabilities Deckhouse ships, which have a `d8:` name and the `heritage: deckhouse` label. On a custom capability the label is ignored: the capability adds its rules to the roles that aggregate it, but no namespace.

To give the subjects of such a role access to another namespace, bind them to a namespace role there with a RoleBinding:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cluster-managers-viewer
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:namespace:viewer
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: cluster-managers
```

### Extending the existing namespace roles

If the resource belongs to a namespace, you need to extend the namespace role instead of the system/subsystem role. The only difference is the labels and the name:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-namespace-as: user
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: namespace
     rbac.deckhouse.io/capability: "custom.namespace-capability.mycustom.superresource_view"
   name: d8:custom:namespace-capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

This capability will be added to the `d8:namespace:user` role.

### Creating a custom namespace or project role

Sometimes the built-in hierarchy of levels does not fit: for example, you need a "developer" role — full view of the namespace plus terminal access to pods (`exec`, `attach`), but without the right to change quotas or RBAC. Such a role is assembled from ready-made capabilities, without writing RBAC rules by hand.

The rules for custom roles:

- the name must start with `d8:custom:` (for example, `d8:custom:namespace:developer`);
- the role must carry the `rbac.deckhouse.io/kind: custom-role` label;
- a namespace or project role that will be used in a RoleBinding must also carry `rbac.deckhouse.io/delegatable: "true"`. Every user namespace is a project, and a RoleBinding there is accepted only for roles with this label. Do not put it on system or subsystem roles — the admission webhook rejects that;
- the role **cannot contain its own rules** (`rules`) — it may only aggregate capabilities via `aggregationRule`. Permissions are described in separate capabilities, so the contents of the role stay transparent;
- a single role cannot aggregate capabilities of the user-facing scopes (`namespace`, `project`) together with the administrative ones (`system`, subsystems) — such a role is rejected.

An example: a role that includes everything allowed by `d8:namespace:viewer`, plus one specific capability (connecting to pods), selected precisely by its unique `rbac.deckhouse.io/capability` label:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:developer
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Required for a RoleBinding inside a project namespace.
  annotations:
    custom.meta.deckhouse.io/title: "Developer"
    custom.meta.deckhouse.io/description: "View resources and connect to pods, without managing quotas and RBAC"
aggregationRule:
  clusterRoleSelectors:
    # Everything included in the viewer level of the namespace lineage.
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-namespace-as: viewer
    # Plus one specific capability selected by its unique name.
    - matchLabels:
        rbac.deckhouse.io/capability: "namespace-capability.kubernetes.access_terminal"
rules: []
```

If no existing capability grants the permissions you need, create your own (a `custom-capability` may contain rules) and add a selector by its `rbac.deckhouse.io/capability` label to the role's `aggregationRule` (in the example below — `matchLabels: {rbac.deckhouse.io/capability: "custom.namespace-capability.port-forward"}`). The label value of a custom capability is `custom.<SCOPE>-capability.<NAME>`, and a role bindable in a project may select only the `namespace-capability` and `project-capability` scopes:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:port-forward
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/capability: "custom.namespace-capability.port-forward"
rules:
  - apiGroups: [""]
    resources: ["pods/portforward"]
    verbs: ["create"]
```

To get a list of all available capabilities and their unique names, use the command:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

The created role is assigned exactly like a built-in one: via a RoleBinding in a namespace or via a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) across a whole project (for project roles, use `rbac.deckhouse.io/scope: project` and aggregate `aggregate-to-project-as`). A RoleBinding in a project namespace requires the `delegatable` label on the role; a ProjectRoleBinding does not (it has its own prefix allow-list). The role cannot be assigned via a ClusterRoleBinding — just like the built-in roles of these scopes.

{% alert level="info" %}
You can also assemble such a role without YAML — with the access grant wizard in the Deckhouse web UI: it shows the available capabilities, builds a role out of them, and immediately creates the required binding.
{% endalert %}

<div style="height: 0;" id="how-do-i-migrate-custom-roles-to-the-new-scheme-in-dp-178"></div>
<div style="height: 0;" id="how-do-i-migrate-custom-roles-to-the-new-scheme-in-dkp-178"></div>

## How do I migrate custom roles to the new scheme?

Along with the [role renaming](./#deprecated-role-names), the label scheme that drives aggregation has changed.

{% alert level="warning" %}
The built-in capabilities no longer carry the `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/kind: use` labels. A custom role of the old scheme that selects capabilities by these labels has lost their permissions, and so have its subjects. Custom roles get no compatibility aliases, so repair such roles as described below.
{% endalert %}

An aggregation selector of a custom role that requires one of these labels (for example, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<SUBSYSTEM>-as`) matches none of the built-in capabilities.
From such a selector, the Kubernetes aggregation controller takes only the rules of the custom capabilities that still carry the old labels.
Deckhouse Platform creates the RoleBinding objects for the subjects of a role with the `rbac.deckhouse.io/use-role` label in the namespaces of the modules only when the role has the `rbac.deckhouse.io/scope: system` or `subsystem` label. A custom role of the old scheme with `use-role` and without that scope has lost these objects, even if its selectors are already rewritten.
A selector that requires the aggregation label of a lineage that no built-in role collects matches none of the built-in capabilities either, because the modules label their capabilities only with the lineages that the built-in roles collect.
Such a label is, for example, `rbac.deckhouse.io/aggregate-to-all-as` of the old scheme, the label of a subsystem the role model no longer has, or the label of a subsystem a module has left.
A lineage that a custom capability carries is an exception, because the selector still gets the rules of that capability. A custom capability here is a ClusterRole without the `heritage: deckhouse` label and without an `aggregationRule` field.
So is the lineage of the role's own subsystem, from its `rbac.deckhouse.io/subsystem` label, because the custom capabilities of that subsystem carry it (for details, see [Extending the custom role](#extending-the-custom-role)).
Neither exception applies to the `all`, `deckhouse`, `infrastructure`, `kubernetes` and `networking` lineages, which do not count as custom ones.
A compatibility alias of an old role, a ClusterRole with the `rbac.deckhouse.io/deprecated: "true"` label, does not count as a built-in role that collects a lineage, because it collects the lineage of an old subsystem only for the bindings to the old name.
A custom capability of the old scheme may also have moved to other roles (for details, see [Replacing a capability](#replacing-a-capability)).

Such roles are listed in the `D8UserAuthzLegacyRBACv2CustomRoleFound` alert.
It fires for every ClusterRole without the `heritage: deckhouse` label and with an `aggregationRule` field that has a selector requiring `rbac.deckhouse.io/kind: manage` or `use`, has a selector requiring such an aggregation label, or carries `rbac.deckhouse.io/use-role` without `rbac.deckhouse.io/scope: system` or `subsystem`, whatever the name of the role.
A role without `aggregationRule`, such as a [role frozen earlier](#roles-frozen-earlier), is not listed.
To list such roles, use the command:

```shell
d8 k get clusterroles -o json | jq -r '
  def lineages: [((.matchLabels // {}) | keys[]), (.matchExpressions[]? | select(.operator == "In" or .operator == "Exists") | .key)]
    | map(capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage);
  ([.items[] | select(.metadata.labels.heritage == "deckhouse" and .metadata.labels["rbac.deckhouse.io/deprecated"] != "true")
        | .aggregationRule.clusterRoleSelectors[]? | lineages[]] + ["system", "namespace", "project"]) as $collected
  | ([.items[] | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule == null) | (.metadata.labels // {}) | keys[]
        | capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage]) as $carried
  | .items[]
  | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule != null)
  | (.metadata.labels["rbac.deckhouse.io/subsystem"] // "") as $own
  | select(
      (
        ((.metadata.labels["rbac.deckhouse.io/use-role"] // "") != "")
        and (((.metadata.labels["rbac.deckhouse.io/scope"] // "") | IN("system", "subsystem")) | not)
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]?
          | (.matchLabels["rbac.deckhouse.io/kind"] // empty),
            (.matchExpressions[]? | select(.key == "rbac.deckhouse.io/kind" and .operator == "In") | .values[])
        ]
        | any(IN("manage", "use"))
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]? | lineages[]
          | select((IN($collected[]) | not)
              and (IN("all", "deckhouse", "infrastructure", "kubernetes", "networking") or (. != $own and (IN($carried[]) | not))))
        ]
        | length > 0
      )
    )
  | .metadata.name'
```

Mapping between the old and the new scheme:

| Before (old scheme) | After (new scheme) |
|---------------------|--------------------|
| Arbitrary role name (for example, `custom:manage:mycustom:manager`) | Mandatory `d8:custom:` prefix (for example, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` or `use` on your role | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` or `use` on your capability | `rbac.deckhouse.io/kind: custom-capability`, name prefixed with `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-all-as: <level>` on your capability | `rbac.deckhouse.io/aggregate-to-system-as: <level>` |
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-all-as: <level>` on your capability | `rbac.deckhouse.io/kind: custom-capability` + `rbac.deckhouse.io/scope: namespace` + `rbac.deckhouse.io/aggregate-to-namespace-as: <level>`, name `d8:custom:namespace-capability:<name>` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-all-as: <level>` on your role | A ClusterRoleBinding of the role to the subjects of `d8:manage:all:<level>` and `d8:system:<level>`, because a custom role cannot carry `rbac.deckhouse.io/aggregate-to-system-as` |
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` on your capability | `rbac.deckhouse.io/kind: custom-capability` + `rbac.deckhouse.io/scope: namespace` + `rbac.deckhouse.io/aggregate-to-namespace-as: <level>`, name `d8:custom:namespace-capability:<name>` |
| Aggregation selector: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` | Only `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` |
| Selector for use permissions: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>` |
| Per-module selector: `rbac.deckhouse.io/kind: manage` + `module: <module>` | `rbac.deckhouse.io/scope: system` + `module: <module>` |
| Labels of the `deckhouse`, `infrastructure` and `kubernetes` subsystems (`rbac.deckhouse.io/aggregate-to-deckhouse-as` and the others) | One label of the `cluster` subsystem, `rbac.deckhouse.io/aggregate-to-cluster-as` |
| Label of the `networking` subsystem, `rbac.deckhouse.io/aggregate-to-networking-as` | `rbac.deckhouse.io/aggregate-to-network-as` |
| The `user-authz`, `user-authn` and `multitenancy-manager` modules with the labels of the `security` and `deckhouse` subsystems | The label of the `iam` subsystem, `rbac.deckhouse.io/aggregate-to-iam-as` |

The names of the built-in capabilities changed as well (no aliases):

* `d8:manage:permission:module:<module>:view|edit` → `d8:system-capability:<module>:view|edit`
* `d8:use:capability:module:<module>:view|edit` → `d8:namespace-capability:<module>:view|edit`

Aggregation selectors match labels, not names. Bind roles, not capabilities.

### Fixing a role

Fix every role that is listed in the `D8UserAuthzLegacyRBACv2CustomRoleFound` alert.
Keep the order of the steps, so that the old role and its bindings stay in place until the new ones grant the access:

1. Create a role of the new scheme next to the old one, with the `d8:custom:` name prefix, the `rbac.deckhouse.io/kind: custom-role` label, and the aggregation selectors of the new scheme, without `rbac.deckhouse.io/kind` (for details, see the [before and after example](#custom-role-before-and-after)). Replace a selector by the aggregation label of a lineage that no built-in role collects as described in [Selectors by a lineage no built-in role collects](#selectors-by-a-lineage-no-built-in-role-collects). Choose the scope of the role, and name it accordingly:

   - A role with the `rbac.deckhouse.io/use-role` label, or with the `rbac.deckhouse.io/level: subsystem` or `all` label, gets `rbac.deckhouse.io/scope: subsystem` and `rbac.deckhouse.io/subsystem: <SUBSYSTEM>` with the name `d8:custom:<SUBSYSTEM>:<NAME>`, or `rbac.deckhouse.io/scope: system` with the name `d8:custom:system:<NAME>`.
     Set the `rbac.deckhouse.io/use-role` label on it to `viewer`. For the subjects of its ClusterRoleBinding objects, Deckhouse then creates RoleBinding objects to `d8:namespace:viewer` in the namespaces of the modules whose system capabilities the role aggregates: a custom role is projected as `viewer` at most, and the webhook refuses a higher value. Without the label the role is not projected into those namespaces at all.
     The `rbacv2-cluster-roles.deckhouse.io` webhook refuses a custom role without the `rbac.deckhouse.io/scope` label whose selectors name the aggregation label of a subsystem or of the system scope, such as `rbac.deckhouse.io/aggregate-to-deckhouse-as`.
   - A namespace or project role that you bind with a RoleBinding gets `rbac.deckhouse.io/scope: namespace` and `rbac.deckhouse.io/delegatable: "true"` with the name `d8:custom:namespace:<NAME>` (for details, see [Creating a custom namespace or project role](#creating-a-custom-namespace-or-project-role)).

1. Create capabilities of the new scheme in place of the custom capabilities of the old scheme that the role aggregates, as described in [Replacing a capability](#replacing-a-capability). Keep the old capabilities for now.
1. For every RoleBinding and ClusterRoleBinding object to the old role, create an object with the same subjects to the new role. The `roleRef` field is immutable, so create a new object instead of editing the old one.
   If the old role has the `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/aggregate-to-all-as` labels, also bind the new role as described in [Roles aggregated into the system roles](#roles-aggregated-into-the-system-roles). With `rbac.deckhouse.io/kind: use`, it extended the namespace roles, see the same section.
1. Check the permissions of the subjects:

   Check whether the specified user can perform an action on a resource across the whole cluster (for cluster-wide resources) or in the current default namespace (for namespaced resources):

   ```shell
   d8 k auth can-i <VERB> <RESOURCE> --as=<USER_NAME>
   ```

   Check whether the specified user can perform an action on a resource in the specified namespace:

   ```shell
   d8 k auth can-i <VERB> <RESOURCE> -n <NAMESPACE> --as=<USER_NAME>
   ```

   To check the permissions of a group, add `--as-group=<GROUP_NAME>` to `--as`.

1. Delete the old RoleBinding and ClusterRoleBinding objects, the old role, and the old capabilities.

If the objects are applied from a Git repository (for example, by Argo CD or Flux), make the same changes in the repository.
Otherwise, the next apply creates the old objects again.

#### Roles aggregated into the system roles

In the old scheme, the `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` label meant one of two things, depending on the kind label next to it.
With `rbac.deckhouse.io/kind: use`, it extended the `d8:use:role:<LEVEL>` roles in every namespace. Their counterpart is `d8:namespace:<LEVEL>`, which selects `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>`, so move such permissions into a namespace capability as described in [Replacing a capability](#replacing-a-capability).
With `rbac.deckhouse.io/kind: manage`, a role with this label extended `d8:manage:all:<LEVEL>`, and the rest of this section is about it.
No role of the new model selects this label. Only the deprecated `d8:manage:all:<LEVEL>` and `d8:use:role:<LEVEL>` aliases still select it, depending on the kind label next to it, until they are removed.
The `d8:manage:all:<LEVEL>` alias and the `d8:system:<LEVEL>` role select `rbac.deckhouse.io/aggregate-to-system-as: <LEVEL>`, and the `rbacv2-cluster-roles.deckhouse.io` webhook admits this label only on a custom capability.
A custom role therefore cannot extend these roles, and the subjects that got the old role through them get the new role only through a ClusterRoleBinding of their own.

The following commands create such a ClusterRoleBinding for each ClusterRoleBinding to `d8:manage:all:<LEVEL>` and `d8:system:<LEVEL>`:

```shell
NEW_ROLE=<NEW_ROLE_NAME>
LEVEL=<LEVEL>
d8 k get clusterrolebindings -o json > clusterrolebindings.json
jq --arg role "$NEW_ROLE" --arg level "$LEVEL" '
  {apiVersion: "v1", kind: "List", items: [.items[]
    | select(.roleRef.kind == "ClusterRole" and (.roleRef.name | IN("d8:manage:all:" + $level, "d8:system:" + $level)))
    | {apiVersion, kind, metadata: {name: ($role + ":" + .metadata.name)},
       roleRef: {apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: $role}, subjects}]}' \
  clusterrolebindings.json > new-clusterrolebindings.json
d8 k apply -f new-clusterrolebindings.json
```

Review `new-clusterrolebindings.json` before you create the objects, and remove the subjects that must not get the role.
Deckhouse Platform does not keep these objects in sync with the ClusterRoleBinding objects they are copied from.
When you revoke the access of a subject, delete both. The name of a copy ends with the name of its source.

#### Selectors by a lineage no built-in role collects

A selector by the aggregation label of a lineage selected the capabilities that carried this label.
For example, `rbac.deckhouse.io/aggregate-to-virtualization-as: <LEVEL>` selected the capabilities of the `virtualization` module.
Once the module labels them with the lineage of the subsystem it is in now, a selector by that lineage selects the capabilities of every module of the subsystem, which can be much more than the role gave before.
To keep the new role as narrow as the old one, select the capabilities themselves by the `rbac.deckhouse.io/capability` label.
Its value does not depend on the subsystem. It is `system-capability.<MODULE_NAME>.<ACTION>` for a system capability and `namespace-capability.<MODULE_NAME>.<ACTION>` for a namespace capability.

1. List the capabilities of the module with the levels of the roles that collect them:

   ```shell
   d8 k get clusterroles -l rbac.deckhouse.io/kind=capability,module=<MODULE_NAME> -o json | jq -r '
     .items[]
     | [.metadata.labels["rbac.deckhouse.io/capability"],
        (.metadata.labels | to_entries
          | map(select(.key | test("^rbac\\.deckhouse\\.io/aggregate-to-.+-as$")) | (.key | ltrimstr("rbac.deckhouse.io/aggregate-to-") | rtrimstr("-as")) + "=" + .value)
          | join(","))]
     | @tsv'
   ```

1. Keep the capabilities that the old selector matched. A selector with the `<LEVEL>` value matched the capabilities of this level and, through the built-in roles of the lower levels, the capabilities of those levels as well.
1. In the new role, put one selector by the `rbac.deckhouse.io/capability` label for each of them. To give all the system capabilities of the module whatever their level, a single selector by `rbac.deckhouse.io/scope: system` and `module: <MODULE_NAME>` is enough.

For example, the following namespace role gives in a namespace what a selector by `rbac.deckhouse.io/aggregate-to-virtualization-as: user` gave, without the other modules of the subsystem. Replace `<ACTION>` with the values from the list:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-user
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
rules: []
```

A role of the system or subsystem scope selects the `system-capability.<MODULE_NAME>.<ACTION>` values the same way.

### Replacing a capability

A custom capability of the old scheme carries the `rbac.deckhouse.io/kind: manage` or `use` label and an aggregation label, and it is listed in the `D8UserAuthzForeignAggregationLabel` alert.
While it carries the aggregation label of the system scope, of the namespace or project scope, or of a built-in subsystem, the `rbacv2-cluster-roles.deckhouse.io` webhook refuses every update of it.
The webhook admits such a label only together with `rbac.deckhouse.io/kind: custom-capability`, and that label requires the `d8:custom:` name prefix, which an existing object cannot get.
Create a capability of the new scheme with the same rules in its place, and delete the old one once the new one grants the access.

The labels of the old capability define the new one:

| Old capability | New capability |
|----------------|----------------|
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL>` | `d8:custom:namespace-capability:<NAME>` with `rbac.deckhouse.io/scope: namespace` and `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<SUBSYSTEM>-as: <LEVEL>`, for a built-in subsystem or a subsystem of your own | `d8:custom:subsystem-capability:<NAME>` with `rbac.deckhouse.io/scope: subsystem`, `rbac.deckhouse.io/subsystem: <SUBSYSTEM>` and the same aggregation label |
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` | `d8:custom:namespace-capability:<NAME>` with `rbac.deckhouse.io/scope: namespace` and `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` | `d8:custom:system-capability:<NAME>` with `rbac.deckhouse.io/scope: system` and `rbac.deckhouse.io/aggregate-to-system-as: <LEVEL>` |

Each new capability also gets the `rbac.deckhouse.io/kind: custom-capability` label.

An old capability with the `rbac.deckhouse.io/namespace: <NAMESPACE>` label gave the subjects of the roles that aggregate it RoleBinding objects in that namespace, the namespace of a module. Deckhouse Platform reads this label only on the ClusterRoles it ships, so neither the old capability nor its custom replacement gives such RoleBinding objects. To keep the access in that namespace, bind the subjects to a namespace role there with a RoleBinding (for details, see [Extending subsystem roles and adding a new namespace](#extending-subsystem-roles-and-adding-a-new-namespace)).

A capability with the `rbac.deckhouse.io/kind: use` and `rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL>` labels extended the `d8:use:role:<LEVEL>` roles.
Only the deprecated `d8:use:role:<LEVEL>` aliases still select it, until they are removed. `d8:namespace:<LEVEL>` and `d8:project:<LEVEL>` select `rbac.deckhouse.io/aggregate-to-namespace-as`, so a RoleBinding moved to a new role does not get the rules of the capability. Such a capability is listed in the `D8UserAuthzLegacyUseCapability` alert.
Replace it before you move the RoleBinding objects to the new roles. For example, the following commands create its counterpart for the namespace roles:

```shell
OLD_CAPABILITY=<OLD_CAPABILITY_NAME>
NEW_CAPABILITY=d8:custom:namespace-capability:<NAME>
LEVEL=<LEVEL>
d8 k get clusterrole "$OLD_CAPABILITY" -o json | jq --arg name "$NEW_CAPABILITY" --arg level "$LEVEL" '
  {apiVersion, kind, metadata: {name: $name, labels: {
     "rbac.deckhouse.io/kind": "custom-capability",
     "rbac.deckhouse.io/scope": "namespace",
     "rbac.deckhouse.io/aggregate-to-namespace-as": $level}},
   rules}' > new-capability.json
d8 k apply -f new-capability.json
```

Check the permissions of the subjects of the namespace roles, and then delete the old capability:

```shell
d8 k delete clusterrole "$OLD_CAPABILITY"
```

The `d8:use:role:<LEVEL>` aliases collect the new capability as well, so deleting the old one takes nothing away from their subjects.

### Roles frozen earlier

A role frozen before the built-in capabilities got the new labels, by removing its `aggregationRule` field and keeping its rules, keeps its permissions and is not listed in the `D8UserAuthzLegacyRBACv2CustomRoleFound` alert.

* The explicit RoleBinding objects that replaced its automatic ones point at `d8:use:role:<LEVEL>`, which the new scheme keeps only as a deprecated alias of `d8:namespace:<LEVEL>`. They are listed in the `D8UserAuthzDeprecatedRBACv2RoleInUse` alert.
  Move them to `d8:namespace:<LEVEL>`, because a binding to an alias grants nothing once the alias is removed.
  The following commands do this for the RoleBinding objects with the `explicit:` name prefix:

  ```shell
  d8 k get rolebindings -A -o json | jq '{apiVersion: "v1", kind: "List", items: [.items[]
    | select(.roleRef.kind == "ClusterRole" and (.roleRef.name | startswith("d8:use:role:")) and (.metadata.name | startswith("explicit:")))]}' \
    > use-role-rolebindings.json
  jq '.items |= map({apiVersion, kind,
      metadata: {name: ("namespace:" + .metadata.name), namespace: .metadata.namespace,
        annotations: ((.metadata.annotations // {}) | with_entries(select(.key == "rbac.deckhouse.io/related-with")))},
      roleRef: (.roleRef | .name |= (sub("^d8:use:role:"; "d8:namespace:") | sub(":kubernetes$"; ""))), subjects})' \
    use-role-rolebindings.json > namespace-rolebindings.json
  d8 k apply -f namespace-rolebindings.json && d8 k get -f namespace-rolebindings.json > /dev/null && d8 k delete -f use-role-rolebindings.json
  ```

  The old objects are deleted only when all the new ones exist, and the commands can be run again after a failure.

* If the role still carries the `rbac.deckhouse.io/aggregate-to-all-as` label, it is listed in the `D8UserAuthzForeignAggregationLabel` alert, and the `rbacv2-cluster-roles.deckhouse.io` webhook refuses every update of the role that keeps the label, a new apply of its manifest included.
  No role of the new model selects this label. The deprecated `d8:manage:all:<LEVEL>` alias still selects it on a role with `rbac.deckhouse.io/kind: manage`, and the deprecated `d8:use:role:<LEVEL>` alias on a role with `rbac.deckhouse.io/kind: use`, until the aliases are removed.

  With `rbac.deckhouse.io/kind: manage`, the subjects of `d8:manage:all:<LEVEL>` keep the role through the ClusterRoleBinding objects that the freeze created for them, so remove the label:

  ```shell
  d8 k label clusterrole <ROLE_NAME> rbac.deckhouse.io/aggregate-to-all-as-
  ```

  With `rbac.deckhouse.io/kind: use`, the role gives its rules to the subjects of every RoleBinding to `d8:use:role:<LEVEL>` through this label, and the freeze created no binding for them. Removing the label takes these rules away from them.
  First create a namespace capability with the rules of the role as described in [Replacing a capability](#replacing-a-capability). Both `d8:namespace:<LEVEL>` and the `d8:use:role:<LEVEL>` alias collect it. Check the permissions of the subjects, and only then remove the label from the role or delete the role.

* A frozen role does not gain permissions for resources added later, such as the resources of a new module. To get them, replace it with a role of the new scheme as described in [Fixing a role](#fixing-a-role).

The RoleBindings that the previous release created in the module namespaces for a binding to a custom role with `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/use-role` are removed on the upgrade. Once the new role carries `rbac.deckhouse.io/scope: system` or `subsystem` and `rbac.deckhouse.io/use-role` and the binding refers to it, they come back as `d8:namespace:<level>:binding:<ClusterRoleBinding name>`, but only in the namespaces of the modules that already carry the new labels and whose system capabilities the new role aggregates. A module that still carries the old labels, such as `rbac.deckhouse.io/kind: manage`, gets no such RoleBinding until it moves to the new ones.
Until then, create RoleBinding objects in the namespace of that module for the subjects of the binding: one to `d8:namespace:<level>` and one to each ClusterRole of the module that is listed in the `D8UserAuthzLegacyUseCapability` alert. None of these roles is an alias, so the RoleBinding objects do not hold the next release, and the webhook that checks RoleBinding objects in project namespaces does not check the namespace of a module. Delete them once the module carries the new labels and the controller creates its own RoleBinding there.
A RoleBinding to `d8:use:role:<level>` gives these rights as well, but the `D8UserAuthzDeprecatedRBACv2RoleInUse` alert reports it, and it holds the release that removes the aliases.

### Examples

#### Custom role before and after

The following is a configuration example of a role combining the permissions of the `deckhouse` and `kubernetes` subsystems and the `user-authn` module.

* Before (old scheme):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: custom:manage:mycustom:manager
    labels:
      rbac.deckhouse.io/use-role: admin
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/level: subsystem
      rbac.deckhouse.io/subsystem: custom
      rbac.deckhouse.io/aggregate-to-all-as: manager
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/kind: manage
          rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
      - matchLabels:
          rbac.deckhouse.io/kind: manage
          rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
      - matchLabels:
          rbac.deckhouse.io/kind: manage
          module: user-authn
  rules: []
  ```

* After (new scheme):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:mycustom:manager
    labels:
      rbac.deckhouse.io/use-role: viewer
      rbac.deckhouse.io/kind: custom-role
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: mycustom
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-cluster-as: manager
      - matchLabels:
          rbac.deckhouse.io/scope: system
          module: user-authn
  rules: []
  ```

What changed:

- The name got the mandatory `d8:custom:` prefix and names the subsystem of the role
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`
- The `rbac.deckhouse.io/aggregate-to-all-as` label is removed. The subjects of `d8:system:manager` get the role through a ClusterRoleBinding (for details, see [Roles aggregated into the system roles](#roles-aggregated-into-the-system-roles))
- The `rbac.deckhouse.io/kind: manage` label is removed from the aggregation selectors
- All system permissions of a module are now selected with `rbac.deckhouse.io/scope: system` + `module: <module>`
- The `deckhouse` and `kubernetes` subsystems are part of the `cluster` subsystem now, so their two selectors became one. It selects the whole `cluster` subsystem, which is wider than the two old ones. To keep the role as narrow as before, select the system capabilities of the modules you need by `module: <module>` and `rbac.deckhouse.io/scope: system` instead

#### Custom capability before and after

The following is an example configuration of a capability, which grants read access to the MySuperResource resource and is aggregated into the role from the example above (its `aggregationRule` field must contain the `rbac.deckhouse.io/aggregate-to-mycustom-as: manager` selector).

* Before (old scheme):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: custom:manage:permission:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/aggregate-to-mycustom-as: manager
  rules:
    - apiGroups:
        - mygroup.io
      resources:
        - mysuperresources
      verbs:
        - get
        - list
        - watch
  ```

* After (new scheme):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:subsystem-capability:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: custom-capability
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: mycustom
      rbac.deckhouse.io/aggregate-to-mycustom-as: manager
  rules:
    - apiGroups:
        - mygroup.io
      resources:
        - mysuperresources
      verbs:
        - get
        - list
        - watch
  ```

### Labels and annotations: before and after

Labels on ClusterRole objects:

| Label | Before | After | Purpose |
|-------|--------|-------|---------|
| `rbac.deckhouse.io/kind` | `manage` or `use` | `custom-role` / `custom-capability` for your own objects; `role` / `capability` on built-in ones (reserved) | The object type in the role model. Mandatory, objects without it are not processed |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Removed | The old role level; replaced by the `scope` label |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | The scope of a role or capability |
| `rbac.deckhouse.io/subsystem` | Subsystem name | Unchanged | The role's subsystem; used with `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | A use-role level | A namespace-role level | Defines which namespace role is automatically granted to the holder of a system/subsystem role in the system namespaces of its modules (via automatically created RoleBinding objects); `viewer` at most for a custom role |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<level>` | With `rbac.deckhouse.io/kind: manage`, replaced by `rbac.deckhouse.io/aggregate-to-system-as` on a custom capability, which a custom role cannot carry. With `rbac.deckhouse.io/kind: use`, replaced by `rbac.deckhouse.io/aggregate-to-namespace-as` on a custom capability | Aggregates the object into the system-wide role (`d8:system:<level>`) |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as` | Used in selectors together with `rbac.deckhouse.io/kind: manage` | Used in selectors on its own | Aggregates the object into the subsystem role of the given level |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<level>` (for use and manage permissions) | Not used. The kubernetes subsystem is part of the `cluster` subsystem (`aggregate-to-cluster-as`), and the old *use*-role meaning moved to `aggregate-to-namespace-as` | Aggregated into the old `d8:manage:kubernetes:<level>` and `d8:use:role:<level>:kubernetes` roles |
| `rbac.deckhouse.io/namespace` | Namespace | Unchanged | An additional namespace where a RoleBinding is automatically created for the role holders; read only on the capabilities Deckhouse ships, ignored on a custom one |
| `rbac.deckhouse.io/capability` | — | A unique capability name (for example, `system-capability.deckhouse.view`) | A machine-readable identifier of a built-in capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` on alias roles | The role is deprecated and will be removed; migrate the bindings to the new role |
| `module` | Module name | Unchanged | Marks a built-in object as belonging to a Deckhouse Platform module; handy in aggregation selectors together with `scope` |
| `heritage: deckhouse` | Platform object marker | Unchanged | Must not be set on custom objects |

Annotations on ClusterRole objects (the old scheme did not use annotations):

| Annotation | Purpose |
|------------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | The displayed name and description of a role/capability in Russian (the platform sets them on built-in objects; you can set your own on custom ones) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Same in English |
| `rbac.deckhouse.io/deprecated-replaced-by` | Set on the alias roles of the previous names and names the new role to move the bindings to. An alias aggregates the capabilities of the **new** role, so the existing bindings to the previous name keep granting access until the aliases are removed. The permissions are not identical to the previous ones, because `d8:use:role:admin` no longer grants ServiceAccount token minting or impersonation |

### Adding a custom capability (in the new scheme)

A capability is a regular ClusterRole object with rules that is automatically included into the chosen role via an aggregation label. In the new scheme, a custom capability is created as follows:

1. Decide which role you want to extend: a namespace role, a subsystem role, the system role, or your own custom role.
1. Create a ClusterRole named `d8:custom:<SCOPE>-capability:<NAME>` with the `rbac.deckhouse.io/kind: custom-capability` label, the `rbac.deckhouse.io/scope: <SCOPE>` label, and the aggregation label of the target role:
   - `rbac.deckhouse.io/aggregate-to-namespace-as: <viewer|user|manager|admin|superadmin>`: Into the `d8:namespace:<level>` namespace role.
   - `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <viewer|manager|superadmin>`: Into the `d8:subsystem:<subsystem>:<level>` subsystem role.
   - `rbac.deckhouse.io/aggregate-to-system-as: <viewer|manager|superadmin>`: Into the `d8:system:<level>` system role.
   - `rbac.deckhouse.io/aggregate-to-<your subsystem name>-as: <level>`: Into your own custom role (its `aggregationRule` field must contain such a selector).
1. Define the permissions in `rules`.

Kubernetes aggregates the rules automatically: right after the capability is created, its permissions appear for all holders of the target role. You can verify the result with `d8 k auth can-i --as <user>` or by inspecting the resulting role rules: `d8 k get clusterrole <role> -o yaml`.

For configuration examples, refer to the ["Custom role before and after"](#custom-role-before-and-after) and ["Custom capability before and after"](#custom-capability-before-and-after) subsections.

## How do I rename a built-in role?

The permissions of built-in roles cannot be changed, but their display title and description can — for example, so that the interface shows them in your company's terms. Add the `custom.meta.deckhouse.io/title` and `custom.meta.deckhouse.io/description` annotations to the role:

```shell
d8 k annotate clusterrole d8:namespace:admin custom.meta.deckhouse.io/title='Team administrator' custom.meta.deckhouse.io/description='Manages resources and access in the team namespace'
```

This is the only modification allowed for objects with the `d8:` prefix (except `d8:custom:*`): an attempt to change the rules, aggregation, or labels of a built-in role is rejected.

## How do I find out who has access to a resource?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), a reverse authorization query is available — the WhoCan resource. It answers the question "who can perform action X on resource Y?" and returns the list of users, groups, and ServiceAccounts:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-networkpolicies
spec:
  resourceAttributes:
    namespace: my-namespace
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

The answer is returned in the `status` field (`users`, `groups`, `serviceAccounts`) directly in the command output; the object is not stored anywhere.

The right to create WhoCan queries is granted by the `d8:user-authz:who-can-checker` cluster role. It is not bound to anyone by default: the query result discloses access subjects across all namespaces, so grant it only to trusted administrators via a ClusterRoleBinding.

The managers of the `iam` and `security` subsystems have this right as well. It comes with the `d8:subsystem-capability:iam:inspect_access` capability of `d8:subsystem:iam:manager` and `d8:subsystem:security:manager`, and so the roles above them have it too (`d8:subsystem:iam:superadmin`, `d8:subsystem:security:superadmin`, `d8:system:manager` and `d8:system:superadmin`).

## How do I find out what a specific user, group, or ServiceAccount is allowed to do?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), the SubjectAccessReport resource is available — the counterpart of WhoCan. It answers the question "what is this subject allowed to do" and returns a ready-made report: which roles are granted through which bindings, which actions on which resources are allowed cluster-wide and in every namespace, and where each permission comes from.

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: what-can-jane-do
spec:
  subject:
    kind: User
    name: jane@example.com
EOF
```

The `spec.subject.kind` field accepts `User`, `Group`, and `ServiceAccount` (for the latter, `spec.subject.namespace` is required). If `spec.subject` is omitted, the report is built for the caller.

Report specifics:

- The user's groups are resolved automatically from the [Group](../user-authn/cr.html#group) catalog, including nested ones: if the user belongs to group `B` and `B` belongs to `A`, the permissions of `A` are included as well. The groups taken into account are returned in `status.subject.groups`. Groups that are not in the catalog (for example, the ones coming from an external identity provider) can be passed in `spec.groups`.
- Every source of a permission (`status.scopes[].resources[].sources[]`) carries a `matchedBy` field showing whether the permission was granted to the subject personally or through a group, which makes it possible to view the access with group-derived grants excluded.
- Namespaces with identical access are merged into a single section (`status.scopes[].namespaces`), so a project with a dozen namespaces does not turn into a dozen identical tables.
- Permissions granted by a ClusterRoleBinding are reported once, in the cluster-wide scope (`status.scopes[].cluster: true`), since they apply in every namespace.
- The report can be limited to specific namespaces with `spec.namespaces`.

The report is built from RBAC data. Admission webhook restrictions are not folded into the RBAC rows; they are flagged in `status.scopes[].caveat`. For example, editing and deleting system resources is denied below the `superadmin` level — except for cluster administrators (`cluster-admin`, `user-authz:super-admin`, and the bypass groups). If the subject's permissions are limited by a ClusterAuthorizationRule, this is reported in `status.notes`.

The right to build a report about **another** subject is granted by the `d8:user-authz:subject-access-checker` cluster role. Like `who-can-checker`, it is intentionally not bound to anyone by default: the report discloses the full permission map of the subject, including other namespaces. A report about oneself is available to every authenticated user and requires no extra permissions.

The managers of the `iam` and `security` subsystems have this right as well. It comes with the `d8:subsystem-capability:iam:inspect_access` capability of `d8:subsystem:iam:manager` and `d8:subsystem:security:manager`, and so the roles above them have it too (`d8:subsystem:iam:superadmin`, `d8:subsystem:security:superadmin`, `d8:system:manager` and `d8:system:superadmin`).

## How does a user see the list of namespaces available to them?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), the namespace list is filtered automatically: the `d8 k get namespaces` command returns to a user only the namespaces they have access to — via any of the mechanisms (role bindings, ProjectRoleBinding/ClusterProjectRoleBinding, ClusterAuthorizationRule/AuthorizationRule). A user does not see foreign namespaces and cannot learn about their existence from the list.

If the `console` module older than 1.44.0 is enabled, the list is not filtered: the access levels of ClusterAuthorizationRule and AuthorizationRule and the namespace roles keep the permission to list every namespace.

The same list is served by the read-only `accessiblenamespaces` resource — any authenticated user can query it **for themselves**:

```shell
d8 k get accessiblenamespaces
```

This is convenient for scripts and interfaces: there is no need to iterate over namespaces checking access to each one.

## How does an interface find out where a user may perform an action?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), the SelfScopedAccessReview resource gives the user who creates it with information about the namespaces in which they are allowed to perform actions within the [filtering scope](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/filtered-listing.html): in the whole cluster, in how many namespaces of the scope and, on request, in which ones. One query replaces an access check per namespace. The namespaces are counted by the rule that filters a list sent with the same scope, so a section built on the answer opens exactly when its list has objects to return.

Any authenticated user can create the resource, and it answers only about the caller. A query with the `spec.user`, `spec.groups`, `spec.uid` or `spec.extra` fields is refused.

Example query from a Deckhouse web interface page that shows deployments of all projects, offers to create one and shows pod logs:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SelfScopedAccessReview
spec:
  scope: projects
  requests:
  - group: apps
    resource: deployments
    verbs: [list, watch]
  - group: apps
    resource: deployments
    verbs: [create]
    includeNamespaces: true
  - group: ""
    resource: pods
    subresource: log
    verbs: [get]
    includeNamespaces: true
EOF
```

Example answer for a user who administers the `shop-dev` and `shop-stage` namespaces of a project and has read access without pod logs in `shop-prod`:

```yaml
status:
  results:
  - allowed: true
    cluster: false
    count: 3
  - allowed: true
    cluster: false
    count: 2
    namespaces: [shop-dev, shop-stage]
  - allowed: true
    cluster: false
    count: 2
    namespaces: [shop-dev, shop-stage]
```

Example answer for a cluster administrator. The namespaces are neither counted nor listed, even with `includeNamespaces: true`:

```yaml
status:
  results:
  - allowed: true
    cluster: true
  - allowed: true
    cluster: true
  - allowed: true
    cluster: true
```

Example answer for a user who can only read deployments in the `billing-prod` namespace of another project:

```yaml
status:
  results:
  - allowed: true
    cluster: false
    count: 1
  - allowed: false
    cluster: false
    count: 0
    namespaces: []
  - allowed: false
    cluster: false
    count: 0
    namespaces: []
```

The query fields:

- `spec.scope`: Scope of the `X-Deckhouse-Scope` header ([filtering scope](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/filtered-listing.html)): `accessible`, `projects`, `system` or `project`.
- `spec.project`: Project of the `project` scope.
- `spec.requests`: Actions to check. Each sets the API group (`group`, empty for the core group), the resource (`resource`), the subresource if any (`subresource`) and the verbs (`verbs`). `includeNamespaces: true` asks for the names of the namespaces.

The `status.results` list has an entry per request, in the same order as the requests in the SelfScopedAccessReview:

- `allowed`: The action is allowed in the whole cluster or in at least one namespace of the scope.
- `cluster`: The action is allowed in the whole cluster. The namespaces are then neither counted nor listed.
- `count`: Number of namespaces of the scope where the action is allowed.
- `namespaces`: Namespaces of the scope where the action is allowed, sorted alphabetically, when the request sets `includeNamespaces: true`. An empty list means none.
- `reason`: `NotFilterable` when the action on a namespaced resource cannot be answered by namespaces: the list of the resource is not filtered by scope. Only a permission in the whole cluster allows such an action.
- `evaluationError`: Reason why this request could not be checked. The other requests are answered.

The namespaces where the action is allowed are counted as follows:

- A namespace counts when every verb of the request is allowed in it. A request for `list` and `watch` counts the namespaces where both are allowed.
- Only the namespaces the user sees in the `accessiblenamespaces` list are counted and named, and the namespaces where a RoleBinding grants access to every authenticated user (the `system:authenticated` group): RBAC opens them to the user, and the filtered list returns their objects.
- For `get`, `list` and `watch` of a resource, the answer names the namespaces the list with the same scope returns objects from.
- Other verbs and subresources, for example `create` or `pods/log`, are checked in every namespace of the scope.
- `get`, `list` and `watch` of `namespaces` are answered with the accessible namespaces whatever the scope, as the list of namespaces is filtered.
- `get`, `list` and `watch` of `deckhouse.io/projects` are answered with every accessible namespace: each stands for its project, and a namespace without the `projects.deckhouse.io/project` label for the virtual project `multitenancy-manager` counts it in, `deckhouse` for `d8-*`, `kube-*`, `upmeter-*` and namespaces with the `heritage: deckhouse` or `heritage: upmeter` label, `default` for the rest. So `count` and `namespaces` list namespaces, not projects: a project with three accessible namespaces counts three. A user who may list namespaces in the whole cluster sees every project and gets `cluster: true`.
- `update`, `patch` and `delete` of `namespaces` and their subresources are checked for every namespace of the scope, with that namespace as the target: the `ClusterAuthorizationRule` limits apply to them.
- When a request for `namespaces` combines a read with a write, a namespace opened to every authenticated user but missing from `accessiblenamespaces` counts only if `get` is allowed in it and `list` and `watch` in the whole cluster, as the API server checks them.
- `create` and `deletecollection` of `namespaces`, every write of `deckhouse.io/projects`, and every action on another cluster-scoped resource, such as `nodes` or `ModuleConfig`, are allowed only by a permission in the whole cluster. Without one the answer is `allowed: false`, with no `count` and no `reason`.
- A read of a namespaced resource the scope does not filter gets `reason: NotFilterable` unless it is allowed in the whole cluster. These are resources without `list`, resources of aggregated APIs such as `metrics.k8s.io`, and resources the cluster does not serve, `deckhouse.io/projects` without the `multitenancy-manager` module included: their lists with a scope are refused.
- In the `projects` and `project` scopes, a namespace with an empty `projects.deckhouse.io/project` label belongs to no project. The answer for `deckhouse.io/projects` counts it for its virtual project, as described above.
- The `accessible` and `system` scopes include the `default`, `kube-*` and `d8-*` namespaces, as their lists do. The `system` and `projects` scopes can overlap: the namespaces of a project named `d8`, such as `d8-x`, belong to both.

The answer describes authorization, the decision whether the user may perform the action on the resource. Admission can still refuse a write of a particular object:

- update and delete of objects with the `deckhouse.io/system-resource=true` label for users below the `superadmin` level;
- update and delete of objects that `multitenancy-manager` creates in project namespaces, and deletion of a project namespace;
- creation of a namespace while the `allowNamespacesWithoutProjects` parameter of the [`multitenancy-manager` module](/modules/multitenancy-manager/configuration.html) is `false`;
- creation of a ProjectRoleBinding or a ProjectNamespace outside the main namespace of the project;
- creation of users, groups and `ClusterAuthorizationRule` objects with rights beyond the user's own.

The answer cannot express a permission for named objects only (`resourceNames` in a role): it asks about a resource without an object name, so such a permission is not counted. A change of rules or bindings reaches the answer within seconds, while the API server can keep its previous decision for a repeated request for up to 30 seconds.

While the `ClusterAuthorizationRules` are not yet loaded or no longer tracked, the whole query is answered with `503` and the `Retry-After` header: the API server refuses requests in that state too, and an answer computed without the rules would read as a refusal.

The answer applies the `ClusterAuthorizationRule` limits whenever the authorization webhook of `user-authz` is enabled through the [`controlPlaneConfigurator.enabled`](configuration.html#parameters-controlplaneconfigurator) parameter and the `control-plane-manager` module, which puts it into the API server configuration, is enabled. If a third-party authorization webhook is set by hand in the [`apiserver.authz.webhookURL`](/modules/control-plane-manager/configuration.html#parameters-apiserver-authz-webhookurl) parameter of the `control-plane-manager` module, the API server may not apply these limits, while the answer still does.

To keep an interface consistent with the data it shows, follow these rules:

1. Ask with the scope of the list the page shows.
1. Show a section when `allowed` is `true`.
1. Cache answers per scope and project.
1. Treat an empty list returned with a scope as the absence of objects, not as missing permissions.
1. Take the namespaces of a creation form from `namespaces` of a request with `includeNamespaces: true`.

The `projects` and `project` scopes require the [`multitenancy-manager`](/modules/multitenancy-manager/) module to be enabled in the cluster. Without it, a query with these scopes is refused with `400`, and the `accessible` and `system` scopes work as usual. The cluster then serves no projects, and requests for `deckhouse.io/projects` get `reason: NotFilterable` unless they are allowed in the whole cluster.

The server answers with an error in the following cases:

- `400`: The query is invalid. Examples are an unknown scope, more than 1000 requests, more than five requests with `includeNamespaces: true`, more than 16 verbs in one request, or more than 10,000,000 checks of a verb in a namespace. A user with 10,000 namespaces reaches the last bound with 1000 requests of one verb, or 500 requests of two: split such a query into several.
- `429`: The same user has 30 queries in progress.
- `504`: The query was not answered within 10 seconds.
- `503`: `permission-browser-apiserver` is unavailable, or it has not loaded the `ClusterAuthorizationRules` yet (with `Retry-After`).

## How do I protect objects that my module places in user namespaces?

The marker is a **label**, `deckhouse.io/system-resource: "true"`, on the object itself — not an annotation. For edits and deletes the objects to protect are selected on the API server side, in the admission `matchConditions`, and those read labels; for `exec`, `attach` and `port-forward` the admission object is the connect request rather than the pod, so the webhook reads the pod itself — its labels again. Either way an object marked with an annotation is not seen and stays open, with nothing to say so. Put the label on every namespaced object your module places into user namespaces (pods, services, PVCs, secrets, custom resources): it then becomes writable by `superadmin` and cluster administrators only, and its pods reachable by `exec`, `attach` and `port-forward` only for them.

For what the label restricts, see [Admin level restrictions and superadmin rights](./#admin-level-restrictions-and-superadmin-rights).
