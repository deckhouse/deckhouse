---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

<div style="height: 0;" id="how-do-i-limit-user-rights-to-specific-namespaces-obsolete-role-based-model"></div>

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces in the experimental role-based model, use `RoleBinding` with the [use role](./#use-roles) that has the appropriate level of access. [Example...](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

In the current role-based model, use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) CR.

## What if there are two ClusterAuthorizationRules matching to a single user?

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
2. `Administrators` can get, edit, list, and delete objects on the cluster level and in the namespaces labeled `env=prod` and `env=stage`.

Because `Jane Doe` matches two rules, some calculations will be made:

* `Jane Doe` will have the most powerful accessLevel across all matching rules — `ClusterAdmin`.
* The `namespaceSelector` options will be combined, so that Jane will have access to all the namespaces labeled with `env` label of the following values: `review`, `stage`, or `prod`.

{% alert level="warning" %}
If there is a rule without the `namespaceSelector` option and `limitNamespaces` deprecated option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.
{% endalert %}

## What happens to the bindings when the cluster is updated to the release with the controller?

Nothing is recreated, and no access is interrupted. The bindings the Helm chart used to render are
adopted in place by `user-authz-controller`: the same objects, the same names, with the chart's
metadata removed and an `ownerReference` to the rule added.

The one thing to plan for is the **release that performs the migration**. A hook stamps
`helm.sh/resource-policy: keep` on every live binding before Helm runs, so the release engine cannot
delete them when they disappear from the rendered manifest — and then the engine makes one pass over
the previous manifest, fetching every object in it to notice that it must be kept. That pass is
proportional to the number of bindings and happens exactly once:

| Bindings in the release | What to expect |
|---|---|
| up to ~5000 | the release takes longer than usual and finishes on its own |
| more than ~5000 | the pass can exceed the 20-minute module release timeout |

For a cluster above that size, do the upgrade with a raised timeout or on the previous release
engine, and put it back afterwards:

```bash
# one of the two, for the duration of the upgrade
d8 k -n d8-system set env deploy/deckhouse HELM_TIMEOUT=60m
d8 k -n d8-system set env deploy/deckhouse USE_NELM=false
```

Count what you have before deciding:

```bash
d8 k get clusterrolebindings -l heritage=deckhouse,module=user-authz --no-headers | wc -l
d8 k get rolebindings -A -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

If the hook cannot stamp every binding it fails and the release does not start, which leaves the
bindings exactly as they were. That is deliberate: a release that ran without the marks would delete
them.

The cost is one-time. Once the controller owns the bindings, the release no longer contains objects
whose number depends on the number of rules, and its duration stops depending on them.

## How do I check that the controller keeps the bindings in sync?

The `user-authz-controller` component reconciles the ClusterRoleBindings and RoleBindings of every ClusterAuthorizationRule and AuthorizationRule, the `d8:use:dict` grants of the experimental role model, and the projections of manage-role bindings into namespaced use RoleBindings. Three sources tell whether it is healthy.

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

`Ready=True` with the reason `BindingsApplied` means the bindings match the rule. `Ready=False` names the problem: `InvalidSpec` (the rule cannot be rendered into bindings; the message says why) or `ApplyError` (the API server rejected a write; the message names the binding and the error). Events with the same reasons are recorded on the rule.

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
| `d8_user_authz_custom_cluster_roles{level}` | Custom ClusterRoles (annotated with `user-authz.deckhouse.io/access-level`) per access level |
| `d8_user_authz_custom_aggregation_missing{level}` | `1` when the aggregated ClusterRole `user-authz:<level>:custom` has no rules although custom roles it must aggregate exist |
| `d8_user_authz_bindings_keep_stamped_total`, `d8_user_authz_keep_stamp_duration_seconds` | Work of the migration hook that protects chart-rendered bindings from being pruned by the release: objects stamped and the duration of its last run |
| `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` | Standard controller-runtime metrics per reconciler (`controller` label: `clusterauthorizationrule-bindings`, `authorizationrule-bindings`, `dict-bindings`, `manage-bindings`) |

**Alerts** (in the `d8_user_authz` Prometheus rules):

- `D8UserAuthzControllerUnavailable` and `D8UserAuthzControllerTargetDown` when the controller has unavailable replicas or is not scraped for 5 minutes.
- `D8UserAuthzControllerReconcileErrorsHigh` on a sustained reconcile error rate.
- `D8UserAuthzBindingsDrift` when bindings of some kind stay out of the desired state for 15 minutes.
- `D8UserAuthzAuthorizationRulesInvalid` and `D8UserAuthzAuthorizationRuleInvalid` counting and naming the rules whose bindings cannot be applied for 10 minutes.
- `D8UserAuthzCustomRolesNotAggregated` when the aggregated ClusterRole of an access level stays empty although custom roles for it exist.

When a rule stays `Ready=False` with `ApplyError` because a binding's `roleRef` was changed by hand, delete that binding: `roleRef` is immutable and the controller recreates the binding with the right one.

## How do I check that the authorization webhook has the current rules?

The authorization webhook and Permission Browser read the multi-tenancy options of the ClusterAuthorizationRules (parameters: `limitNamespaces`, `namespaceSelector`, `allowAccessToSystemNamespaces`) straight from the API through a shared informer, so a change reaches them within the informer's coalescing window rather than after a Helm render.

**Ordering.** `user-authz-controller` writes the ClusterRoleBindings of a rule around the same time as the rule itself, and the two reach the webhook over independent watches, so the bindings can arrive first. A subject that a controller-managed binding binds to a rule the webhook has not observed naming it is denied every namespace until the rule arrives — a cluster-wide binding never grants more than its rule allows. The same guard applies to Permission Browser, so what it reports cannot be wider than what the API server enforces.

**Health.** The webhook reports the state of its rules informer:

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

The `rules source: directory rebuilt from N rules` line shows the informer has listed the rules and how many it holds; `quarantined` counts the rules whose `limitNamespaces` patterns did not compile.

**Metrics.** The webhook serves them on `127.0.0.1` inside its Pod; the `kube-rbac-proxy` sidecar exposes them on the node and the `PodMonitor` `user-authz-webhook` collects them (the `operator-prometheus` module must be enabled). There is one series per master.

| Metric | Description |
|---|---|
| `user_authz_webhook_rules_informer_synced` | `1` once the webhook has listed the `ClusterAuthorizationRules` at least once. While it is `0`, every subject bound by a rule binding is denied |
| `user_authz_webhook_rules_observed` | Rules the current directory was built from |
| `user_authz_webhook_rules_subjects` | Distinct subjects in the current directory |
| `user_authz_webhook_rules_max_resource_version` | Highest `resourceVersion` among the observed rules — the watermark to compare against the cluster when measuring lag |
| `user_authz_webhook_rules_quarantined` | Rules whose `limitNamespaces` pattern or `namespaceSelector` does not compile. The broken filter is left out, so the subjects get a narrower scope than written |
| `user_authz_webhook_rules_directory_updated_timestamp_seconds` | Unix time of the last rebuild |
| `user_authz_webhook_rules_directory_rebuilds_total`, `user_authz_webhook_rules_directory_rebuild_duration_seconds` | Number of rebuilds and the time they take |
| `user_authz_webhook_rules_watch_errors_total` | List/watch errors of the rules informer |

Permission Browser exports the same set under the `user_authz_permission_browser` prefix. It serves them the same way, on a loopback endpoint behind a `kube-rbac-proxy` sidecar, collected by the `PodMonitor` `permission-browser-apiserver`; its own API server port stays behind the aggregation layer, which Prometheus cannot scrape. Two alerts watch it: `D8UserAuthzPermissionBrowserTargetDown` and `D8UserAuthzPermissionBrowserRulesNotSynced`. A rule that does not compile, or a watch that keeps failing, is a property of the cluster rather than of one consumer, so the webhook alerts above already report it.

**Alerts** (in the `d8_user_authz` Prometheus rules, grouped as `D8UserAuthzWebhookMalfunctioning`):

- `D8UserAuthzWebhookTargetDown` when the webhook is not scraped for 5 minutes (while it is active the other alerts below cannot fire).
- `D8UserAuthzWebhookRulesNotSynced` when an instance has not listed the rules for 10 minutes.
- `D8UserAuthzWebhookRulesQuarantined` when a rule does not compile for 10 minutes.
- `D8UserAuthzWebhookRulesWatchErrors` on a sustained watch error rate for 15 minutes.
- `D8UserAuthzWebhookRulesStale` when the directory has not been rebuilt for a day (expected in a cluster where the rules do not change).

Two more watch the masters against each other, which single-instance metrics cannot: `D8UserAuthzWebhookDirectoryDiverged` when the instances have been built from different sets of rules for 10 minutes — the same request is then answered differently depending on which master takes it — and `D8UserAuthzRulePropagationLag` when one instance has not rebuilt its directory for an hour while another has, which is the asymmetric case where the two agree on the highest `resourceVersion` they have seen and still differ. The second condition of that one is what keeps a cluster whose rules genuinely never change from firing it.

## Why does a change to a ClusterAuthorizationRule take up to 30 seconds to take effect?

Because the API server caches the webhook's answers. The `AuthorizationConfiguration` that `control-plane-manager` renders gives the webhook `authorizedTTL: 5m`, `unauthorizedTTL: 30s` and `timeout: 3s`; the cache key is the whole SubjectAccessReview, so a repeated identical request is answered from the cache without asking the webhook again.

The webhook never allows. A rule says where an access level applies, not whether it grants the verb — that is RBAC's question, and answering it here would take RBAC out of the chain. So every answer the webhook gives is either a denial or no opinion, both of which are cached under `unauthorizedTTL`, and `authorizedTTL` never applies to it.

What this costs is the no-opinion case. If a user made a request while nothing limited them, the API server remembers that no-opinion for 30 seconds, and during those 30 seconds RBAC alone answers the identical request. Creating a rule, or narrowing one, therefore takes effect for a given request within 30 seconds of the last time that exact request was made, and not within the informer's window:

```bash
# Immediately after creating a limiting rule, an identical request asked in the preceding
# 30 seconds is still answered from the cache.
d8 k auth can-i --as=user@example.com get pods -n other-namespace
sleep 30
d8 k auth can-i --as=user@example.com get pods -n other-namespace
```

The window cannot be flushed without restarting `kube-apiserver`. It is also why the webhook answers `503` rather than a denial while its caches are still filling at startup: a denial would be remembered for 30 seconds after the webhook is ready to answer properly.

## How do I extend a role or create a new one?

[The experimental role model](./#experimental-role-based-model) is based on the aggregation principle; it compiles smaller roles into larger ones,
thus providing easy ways to enhance the model with custom roles.

### Creating a new role subsystem

Suppose that the current subsystems do not fit the role distribution in the company. You need to create a new [subsystem](./#subsystems-of-the-role-based-model)
that includes roles from the `deckhouse` subsystem, the `kubernetes` subsystem and the user-authn module.

To meet this need, create the following role:

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

The labels for the new role listed at the top suggest that:

- The hook will use this use role:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- The role must be treated as a managed one:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Note that this label is mandatory.
  
- The role is a subsystem one, and it shall be handled accordingly:

  ```yaml
  rbac.deckhouse.io/level: subsystem
  ```

- There is a subsystem for which the role is responsible:

  ```yaml
  rbac.deckhouse.io/subsystem: custom
  ```

- The `manage:all` role can aggregate this role:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: manager
  ```

Then there are selectors that implement aggregation:

- This one aggregates the manager role from the `deckhouse` subsystem:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- This one aggregates all the rules defined for the user-authn module:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

This way, your role will combine permissions of the `deckhouse` subsystem, `kubernetes` subsystem, and the user-authn module.

Notes:

* There are no restrictions on role name, but we recommend following the same pattern for the sake of readability.
* Use-roles will be created in aggregate subsystems and the module namespace, the role type is specified by the label.

### Extending the custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a manage role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource.

First, you have to add a new selector to the role:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: manager
```

This selector would enable roles to be aggregated to a new subsystem by specifying this label. After adding the new selector, the role will look as follows:

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
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-custom-as: manager
 rules: []
 ```

 Next, you need to create a new role and define permissions for the new resource, e. g., the read-only permission:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-custom-as: manager
     rbac.deckhouse.io/kind: manage
   name: custom:manage:permission:mycustom:superresource:view
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

The role will update the subsystem role to include its rights, so that the role bearer will be able to view the new object.

Notes:

* There are no restrictions on capability names, but we recommend following the same pattern for the sake of readability.

### Extending the existing manage subsystem roles

To extend an existing role, follow the procedure outlined in the section above. Be sure to change the labels and the role name!

For example, here's how you can extend the manager role from the `deckhouse`(`d8:manage:deckhouse:manager`) subsystem:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
  name: custom:manage:permission:mycustom:superresource:view
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

This way, the new role will extend the `d8:manage:deckhouse:manager` role.

### Extending manage subsystem roles and adding a new namespace

If you need to create a new namespace (to create a use role in it by the hook), you only need to add one label:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

This label instructs the hook to create a use role in this namespace:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: manage
     rbac.deckhouse.io/namespace: namespace
   name: custom:manage:permission:mycustom:superresource:view
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

The hook monitors `ClusterRoleBinding`, and when creating a bindings, it loops through all the manage roles to find all the aggregated roles by checking the aggregation rule. It then fetches the namespace from the `rbac.deckhouse.io/namespace` label and creates a use role in that namespace.

### Extending the existing use roles

If the resource belongs to a namespace, you need to extend the use role instead of the manage role. The only difference is the labels and the name:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-kubernetes-as: user
     rbac.deckhouse.io/kind: use
   name: custom:use:capability:mycustom:superresource:view
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

This role will be added to the `d8:use:role:user:kubernetes` role.

## How do I migrate custom roles to the new scheme in DKP 1.78?

{% alert level="warning" %}
This section describes the [role model renaming](./#migration-to-the-new-role-names-in-dkp-178) that will take effect in DKP 1.78. Prior to DKP 1.78, custom roles keep working under the old scheme.
{% endalert %}

Along with the role renaming ([name mapping](./#migration-to-the-new-role-names-in-dkp-178)) in DKP 1.78, the label scheme that drives aggregation will change.

Custom roles created with the old scheme **will stop aggregating permissions** after upgrading to DKP 1.78: the built-in capabilities will be relabeled, and the old aggregation selectors (for example, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as`) will no longer match them. No compatibility aliases are created for custom roles — they must be updated manually.

Mapping between the old and the new scheme:

| Before (old scheme) | After (new scheme) |
|---------------------|--------------------|
| Arbitrary role name (for example, `custom:manage:mycustom:manager`) | Mandatory `d8:custom:` prefix (for example, `d8:custom:subsystem:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` or `use` on your role | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` or `use` on your capability | `rbac.deckhouse.io/kind: custom-capability`, name prefixed with `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <level>` | `rbac.deckhouse.io/aggregate-to-system-as: <level>` |
| Aggregation selector: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` | Only `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` |
| Selector for use permissions: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>` |
| Per-module selector: `rbac.deckhouse.io/kind: manage` + `module: <module>` | `rbac.deckhouse.io/scope: system` + `module: <module>` |

The names of the built-in capabilities will change as well (no aliases):

* `d8:manage:permission:module:<module>:view|edit` → `d8:system-capability:<module>:view|edit`
* `d8:use:capability:module:<module>:view|edit` → `d8:namespace-capability:<module>:view|edit`

Aggregation selectors match labels, not names, so updating the selectors will be enough. Do not bind capabilities directly.

### Migration steps

After upgrading to DKP 1.78, do the following:

1. Create a new version of a custom role — with the `d8:custom:` prefix, the `rbac.deckhouse.io/kind: custom-role` label, and the new aggregation selectors. See the before and after examples below.
1. Recreate your capabilities with the `rbac.deckhouse.io/kind: custom-capability` label and the `d8:custom:` name prefix.
1. Recreate the RoleBinding and ClusterRoleBinding objects pointing at the old role with the new name in the `roleRef` field. This field is immutable, so a binding has to be deleted and created anew.
1. After you ensure the new bindings are correct, delete the old roles and capabilities.

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
    name: d8:custom:subsystem:mycustom:manager
    labels:
      rbac.deckhouse.io/use-role: admin
      rbac.deckhouse.io/kind: custom-role
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: mycustom
      rbac.deckhouse.io/aggregate-to-system-as: manager
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
      - matchLabels:
          rbac.deckhouse.io/scope: system
          module: user-authn
  rules: []
  ```

What changed:

- The name got the mandatory `d8:custom:` prefix
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`
- `rbac.deckhouse.io/aggregate-to-all-as` → `rbac.deckhouse.io/aggregate-to-system-as`
- The `rbac.deckhouse.io/kind: manage` label is removed from the aggregation selectors
- All system permissions of a module are now selected with `rbac.deckhouse.io/scope: system` + `module: <module>`

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
      rbac.deckhouse.io/aggregate-to-custom-as: manager
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
    name: d8:custom:capability:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: custom-capability
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
| `rbac.deckhouse.io/kind` | `manage` or `use` | `custom-role` / `custom-capability` for your own objects; `role` / `capability` on built-in ones (reserved) | The object type in the role model. Mandatory: objects without it are not processed |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Removed | The old role level; replaced by the `scope` label |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | The scope of a role or capability |
| `rbac.deckhouse.io/subsystem` | Subsystem name | Unchanged | The role's subsystem; used with `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | A use-role level | A namespace-role level | Defines which namespace role is automatically granted to the holder of a system/subsystem role in the system namespaces of its modules (via automatically created RoleBinding objects) |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<level>` | Renamed to `rbac.deckhouse.io/aggregate-to-system-as` | Aggregates the object into the system-wide role (`d8:system:<level>`) |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as` | Used in selectors together with `rbac.deckhouse.io/kind: manage` | Used in selectors on its own | Aggregates the object into the subsystem role of the given level |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<level>` (for use permissions) | Renamed to `rbac.deckhouse.io/aggregate-to-namespace-as` | Aggregates the object into the namespace role (`d8:namespace:<level>`) |
| `rbac.deckhouse.io/namespace` | Namespace | Unchanged | An additional namespace where a RoleBinding is automatically created for the role holders |
| `rbac.deckhouse.io/capability` | — | A unique capability name (for example, `system-capability.deckhouse.view`) | A machine-readable identifier of a built-in capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` on alias roles | The role is deprecated and will be removed; migrate the bindings to the new role |
| `module` | Module name | Unchanged | Marks a built-in object as belonging to a DKP module; handy in aggregation selectors together with `scope` |
| `heritage: deckhouse` | Platform object marker | Unchanged | Must not be set on custom objects |

Annotations on ClusterRole objects (the old scheme did not use annotations):

| Annotation | Purpose |
|------------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | The displayed name and description of a role/capability in Russian (the platform sets them on built-in objects; you can set your own on custom ones) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Same in English |
| `rbac.deckhouse.io/deprecated-replaced-by` | Introduced in DKP 1.78 together with the new scheme. The aggregation rules of the previous roles will change so that these roles keep granting the same permissions as their new counterparts — existing bindings will not break. However, the previous roles are kept for one release only: during that time, the bindings must be migrated to the new roles. The annotation is set on every previous role and contains the name of the new role that is equivalent to it in terms of permissions — that is the role to migrate to |

### Adding a custom capability (in the new scheme)

A capability is a regular ClusterRole object with rules that is automatically included into the chosen role via an aggregation label. In the new scheme, a custom capability is created as follows:

1. Decide which role you want to extend: a namespace role, a subsystem role, the system role, or your own custom role.
1. Create a ClusterRole with the `d8:custom:` name prefix (for readability — `d8:custom:capability:<name>:<resource>:<action>`), the `rbac.deckhouse.io/kind: custom-capability` label, and the aggregation label of the target role:
   - `rbac.deckhouse.io/aggregate-to-namespace-as: <viewer|user|manager|admin|superadmin>`: Into the `d8:namespace:<level>` namespace role.
   - `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <viewer|manager|superadmin>`: Into the `d8:subsystem:<subsystem>:<level>` subsystem role.
   - `rbac.deckhouse.io/aggregate-to-system-as: <viewer|manager|superadmin>`: Into the `d8:system:<level>` system role.
   - `rbac.deckhouse.io/aggregate-to-<your subsystem name>-as: <level>`: Into your own custom role (its `aggregationRule` field must contain such a selector).
1. Define the permissions in `rules`.

Kubernetes aggregates the rules automatically: right after the capability is created, its permissions appear for all holders of the target role. You can verify the result with `d8 k auth can-i --as <user>` or by inspecting the resulting role rules: `d8 k get clusterrole <role> -o yaml`.

For configuration examples, refer to the ["Custom role before and after"](#custom-role-before-and-after) and ["Custom capability before and after"](#custom-capability-before-and-after) subsections.
