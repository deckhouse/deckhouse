---
title: "Descheduler resource contract"
description: What a Descheduler v1alpha2 resource turns into, what the module sets for it, and how one resource can stop descheduling for everyone.
---

This document is for producers, that is, whoever writes a `Descheduler` resource: a module that
ships one in its chart (today the external `virtualization` module ships
`Descheduler/virtualization`) or a cluster administrator. It describes Deckhouse 1.77.1 and main,
which run upstream descheduler v0.36.0 with the module's patches. The `Since` columns give the
release in which a field or strategy appeared. Anything this document does not describe is outside
the contract.

## What the module does with your resource

Every `Descheduler` with at least one strategy key becomes one profile of a single descheduler
policy, and one descheduler process runs all the profiles.

- The policy lives in the ConfigMap `d8-descheduler/descheduler-policy`, and the Deployment
  `d8-descheduler/descheduler` runs it with one replica. A profile that the descheduler cannot load
  stops every profile in the cluster, including those of other producers.
- The descheduler reads the policy once, at process start. Its pod restarts when a `Descheduler` is
  created or deleted, when the `spec` of any of them changes, and when a `metrics.k8s.io`
  APIService appears or disappears; the new process runs a cycle for all profiles right away.
  Re-applying an unchanged spec, or changing labels or annotations through `v1alpha2`, does nothing.
- The descheduler runs only while at least one profile exists. Without any `Descheduler` the module
  renders no policy, no Deployment and no scrape configuration, so deleting the last resource stops
  descheduling. Nothing creates a default resource.
- The resource has no status, and the module never writes to it. To see what took effect, read the
  policy and the descheduler pod (see "Signals").
- A resource cannot choose the interval or the nodes pods are evicted from. All profiles run at the
  interval set by `deschedulingInterval` in the `descheduler` ModuleConfig: `Frequent` (5m),
  `Moderate` (15m, the default) or `Rare` (30m). Before 1.76.0 the interval was a fixed 15m. A
  cycle runs only when at least two nodes are Ready; otherwise the descheduler skips it and logs
  `Skipping descheduling cycle: requires >=2 nodes`, so a single-node cluster is never descheduled.
  Pods can be evicted from any Ready node, cordoned and control-plane nodes included; cordoned
  nodes are excluded only as destinations, and no field limits the source nodes. In each cycle the
  deschedule strategies of all profiles run first, then the balance strategies of all profiles;
  profiles run in order of resource name.

## Shipping a resource from a module

```yaml
{{- if has "deckhouse.io/v1alpha2/Descheduler" .Values.global.discovery.apiVersions }}
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: virtualization
spec:
  namespaceLabelSelector:
    matchLabels:
      example.com/descheduling: enabled
  strategies:
    lowNodeUtilization:
      enabled: true
      thresholds: {cpu: 20, memory: 20, pods: 20}
      targetThresholds: {cpu: 70, memory: 70, pods: 70}
{{- end }}
```

Guard the template with the discovery check alone, as the example does. The descheduler module is
optional: it is off in the Minimal bundle and in CSE, and an administrator can disable it. Three
checks are available, and they are true at different times:

| Check | True when |
|---|---|
| `has "deckhouse.io/v1alpha2/Descheduler" .Values.global.discovery.apiVersions` | The running Deckhouse process has installed the descheduler CRD since it started. It stays true after the module is disabled, until the next Deckhouse process start: a pod restart, a leader change, or the self-restart that follows a ModuleRelease or ModulePullOverride deploy. After such a start with the module disabled it is false, although the CRD is still there, so your release removes the object. This holds for Deckhouse 1.74 and later; earlier releases are not covered. |
| `.Capabilities.APIVersions.Has "deckhouse.io/v1alpha2/Descheduler"` | The API server serves the kind. The CRD is never deleted, so this stays true once the module has been enabled. The CRD is not updated while the module is disabled either: its schema is the one from the last release that ran with the module enabled. A resource rendered under this check, or under `helm_lib_api_version_exists` (which ORs it with the discovery check), can therefore use fields the installed CRD lacks, and then your release fails whatever `requirements.deckhouse` says. |
| `has "descheduler" .Values.global.enabledModules` | The module is enabled right now. |

A resource that exists while the module is disabled is inert: no profile runs. None of the checks
tells you whether descheduling runs. Do not list `descheduler` in `requirements.modules` instead:
a required dependency disables your module whenever descheduler is disabled, and even an optional
one makes your module wait for descheduler after a Deckhouse restart, so a broken `Descheduler`
also blocks the release that would fix it.

Gate on the Deckhouse version as well. `v1alpha2` has existed since 1.67.0, but several fields came
later (see the tables below). Since 1.75.0 Deckhouse applies module releases server-side, and a
server-side apply rejects the whole object if it has any key the installed CRD does not declare
(HTTP 500, `field not declared in schema`). Your release then fails, for example when it ships
`removePodsHavingTooManyRestarts` to a 1.75.x cluster. Set `requirements.deckhouse` in your
`module.yaml` to cover the newest field you use, and background evictions if you rely on them (see
"Background evictions").

Name the resource after your module. The resource is cluster-scoped, so the name is its only key,
and it becomes the profile name and the `profile` label of metrics. Fields your chart sets, and any
change made with `kubectl edit`, are reverted the next time your release is applied. A field your
chart does not set, added with `kubectl patch` or `apply`, survives your releases and changes your
profile; a label added with `kubectl label` survives too but changes nothing. Administrators should
create their own resources instead of editing a module's.

Writing `Descheduler` resources needs the user-authz `ClusterAdmin` access level since 1.77.0
(before that only `SuperAdmin`), or an RBACv2 role that includes the capability
`d8:system-capability:descheduler:edit` (named `d8:manage:permission:module:descheduler:edit` up to
1.77); see [how the roles are built](../../../140-user-authz/docs/README.md#how-the-roles-are-built-aggregation-and-capabilities).
A module's own release needs nothing extra.

## Fields

### Resource-wide fields

| Field | Since | What reaches the descheduler |
|---|---|---|
| `metadata.name` | 1.67.0 | The profile name, written unquoted into the policy. A name that YAML reads as a number or a boolean stops every profile; see "What breaks, and how far". A name of `null` loads, but as an empty profile name, so the `profile` label of metrics and the profile part of eviction reasons are empty. |
| `spec.nodeLabelSelector` | 1.67.0 | A label-selector string for the DefaultEvictor `nodeSelector`. It only limits the nodes an evicted pod must fit on; it does not limit the nodes pods are evicted from. See below. |
| `spec.podLabelSelector` | 1.67.0 | DefaultEvictor `labelSelector`: only matching pods are evicted. `{}` matches every pod. The `descheduler.alpha.kubernetes.io/evict` pod annotation bypasses it. |
| `spec.namespaceLabelSelector` | 1.67.0 | DefaultEvictor `namespaceLabelSelector`: only pods in matching namespaces are evicted. `{}` matches every namespace. The evict annotation does not bypass it. On 1.76.x a selector with only `matchExpressions` is ignored. |
| `spec.priorityClassThreshold` | 1.67.0 | DefaultEvictor `priorityThreshold`, by `name` or by `value`, never both. Only pods with a priority strictly below it are evicted. See below. |
| `spec.evictLocalStoragePods` | 1.70.0 | `true` allows evicting pods with any `emptyDir` or `hostPath` volume. Default `false`. |
| `spec.strategies` | 1.67.0 | Required, with at least one strategy key. Each enabled strategy becomes one plugin; see the next table. |

Nothing else reaches the descheduler. Apart from the resource names inside `thresholds` and
`targetThresholds`, which are kept as written, the schema declares no other keys: neither
`spec.nodeSelector` nor upstream plugin arguments such as `numberOfNodes`, `evictableNamespaces`,
`excludeOwnerKinds`, `includingInitContainers` or eviction limits. A module release fails on such a
key, and kubectl's default strict validation rejects it too. The API server prunes the key only
from a write that is not a server-side apply and uses field validation `Warn` (the server default,
which answers with an `unknown field` warning) or `Ignore`.

### Strategies

| Strategy key | Since | Plugin, extension point | Arguments that reach the plugin |
|---|---|---|---|
| `lowNodeUtilization` | 1.67.0 | `LowNodeUtilization`, balance | `thresholds`, `targetThresholds` |
| `highNodeUtilization` | 1.67.0 | `HighNodeUtilization`, balance | `thresholds` |
| `removeDuplicates` | 1.67.0 | `RemoveDuplicates`, balance | none |
| `removePodsViolatingNodeAffinity` | 1.67.0 | `RemovePodsViolatingNodeAffinity`, deschedule | `nodeAffinityType` |
| `removePodsViolatingInterPodAntiAffinity` | 1.67.0 | `RemovePodsViolatingInterPodAntiAffinity`, deschedule | none |
| `removePodsViolatingTopologySpreadConstraint` | 1.75.0 | `RemovePodsViolatingTopologySpreadConstraint`, balance | `constraints`, `topologyBalanceNodeFit` |
| `removePodsHavingTooManyRestarts` | 1.76.0 | `RemovePodsHavingTooManyRestarts`, deschedule | `podRestartThreshold` |

Each strategy has an `enabled` flag, `false` by default. A disabled strategy is not rendered, so
the descheduler never checks its arguments; the CRD still checks their types, enums and bounds. The
profile stays, though. A resource with every strategy disabled is a profile that evicts nothing,
keeps the descheduler running, and still has its resource-wide fields checked at start. To remove a
profile, delete the resource.

- `lowNodeUtilization`: thresholds are percentages of node allocatable. Send both maps, non-empty,
  with the same resource keys, each threshold at or below its target. The CRD bounds `cpu`,
  `memory` and `pods` to integers from 0 to 100 (since 1.75.0). Neither the CRD nor the module
  checks any other key, such as an extended resource, but the descheduler still requires a number
  from 0 to 100 there, and any other value stops every profile. A node is underutilized when every
  listed resource is at or below its threshold, and overutilized when any listed resource is above
  its target. Usage comes from one of two sources:
  - By default, from pod requests. Usage is the sum of pod requests, and limits are ignored. If the
    underutilized nodes do not have a listed resource, the strategy evicts nothing.
  - Since 1.76.0, from node metrics whenever the cluster has any APIService for `metrics.k8s.io`,
    even an unavailable one. The module makes this switch on its own, and a resource cannot opt
    out. In Deckhouse this is the usual state: the `prometheus-metrics-adapter` module, enabled in
    the Default and Managed bundles, provides that API. cpu and memory usage then covers the whole
    node, system daemons included. It is smoothed by a moving average that is refreshed every 5
    seconds and gives each new sample a weight of 0.1, so a change takes about a minute to show.
    The first cycle after a restart (a spec change of any `Descheduler` causes one) usually sees a
    single unsmoothed sample. `pods` is still a pod count. A threshold on any other resource, or a
    Ready node that has had no metrics since the descheduler started, makes the strategy fail
    every cycle, and only the log shows it. If a node's metrics stop later, the descheduler keeps
    seeing the node at its last value until the next restart.
- `highNodeUtilization`: send non-empty `thresholds`. The strategy always uses requests and evicts
  pods only from nodes at or below every threshold. The evicted pods are packed onto busier nodes
  only if they use `schedulerName: high-node-utilization`, the scheduler profile of
  control-plane-manager, which packs correctly since 1.77.0. With the default scheduler they spread
  out again.
- `removeDuplicates`: duplicates are pods with the same namespace, owner and container images. The
  strategy spreads them evenly over the nodes they fit on, which can leave several on one node, and
  acts only when at least two such nodes exist.
- `removePodsViolatingNodeAffinity`: `nodeAffinityType` takes
  `requiredDuringSchedulingIgnoredDuringExecution` (the default) and
  `preferredDuringSchedulingIgnoredDuringExecution`. Omit it for the default; `[]` stops every
  profile.
- `removePodsViolatingTopologySpreadConstraint`: `constraints` defaults to `[DoNotSchedule]`, and
  `[]` acts as the default; `topologyBalanceNodeFit` defaults to `true`. The skew counts only pods
  that pass the profile's pod filter. Pods outside `podLabelSelector`, at or above the priority
  threshold, or protected by the module (see "What the module sets for every profile") do not count
  unless they carry the evict annotation, so the descheduler can see a different skew than the
  scheduler.
- `removePodsHavingTooManyRestarts`: `podRestartThreshold` is at least 1, 100 by default. Keep it at
  or below 2147483647: the API accepts larger values, and they wrap around to a different number on
  the way to the descheduler. A pod is evicted when the restarts of its regular containers reach the
  threshold; init containers are not counted. This is also the only strategy that removes bare pods
  in phase `Failed`.

### nodeLabelSelector

The module flattens the selector into one string, with the requirements sorted by key, and writes it
unquoted into the policy:

| You write | The descheduler gets | Effect |
|---|---|---|
| no field | no selector | Any Ready, uncordoned node that fits the pod counts for `nodeFit`. |
| `{}` | `<none>` | The profile evicts nothing; every pod fails the filter with a log line. |
| a selector the CRD accepts but Kubernetes does not: `In`/`NotIn` with `values: []`, an invalid key or value | `<error>` | The profile evicts nothing. |
| `DoesNotExist` on the key that sorts first, giving `!foo` or `!foo,zone=a` | an empty string (YAML reads `!` as a tag) | The restriction silently disappears. |
| the same followed by a set-based requirement, `!foo,zone in (a)` | `in (a)` | The profile evicts nothing. |
| a single `Exists` on the key `null`, `Null` or `NULL` | an empty string | The restriction silently disappears. |
| a single `Exists` on a key that YAML reads as a number or a boolean (`123`, `true`, `on`) | nothing: the policy does not decode | Every profile stops. |

Use `matchLabels`, `In` or `NotIn`. Put `DoesNotExist` only on a key that sorts after the key of
another requirement.

### Priority threshold

A pod is evicted only if its priority is strictly below the threshold. Without
`priorityClassThreshold` the threshold is 2000000000 (`system-cluster-critical`), and pods at or
above 2000000000 are not evicted anyway. The descheduler resolves a `name` to the class value once,
at start. Every Deckhouse cluster has the same classes:

| Class | Value |
|---|---|
| `standby` | -1 |
| `develop` (global default) | 1000 |
| `cluster-low` | 2000 |
| `staging` | 3000 |
| `production-low` | 4000 |
| `deployment-machinery` | 5000 |
| `production-medium` | 6000 |
| `cluster-medium` | 7000 |
| `production-high` | 9000 |

A pod without `priorityClassName` gets `develop`, priority 1000. With `name: develop`, or a `value`
from 1 to 1000, only `standby` pods and pods of custom classes below the threshold stay evictable;
a negative value or `name: standby` protects `standby` too. For ordinary workloads such a profile
does nothing, and no Event or metric reports it; the only sign is the descheduler log line
`pod has higher priority than specified priority class threshold`. `name: X` protects class X
itself; to include X, name the next class up. To evict as broadly as possible, omit the field.

Never write `value: 0` or `name: ""`; see "What breaks, and how far". A named class must exist at
every descheduler start, and its value must not exceed 2000000000 (so `system-node-critical` is
not allowed). Otherwise every profile stops.

## What the module sets for every profile

Every profile gets the same DefaultEvictor, and a resource cannot change it:

| Setting | Value | Consequence |
|---|---|---|
| `nodeFit` | `true` | A pod is evicted only if it fits another Ready, uncordoned node that matches `nodeLabelSelector`. The checks are node selector, required node affinity, NoSchedule and NoExecute taints, the pod's required anti-affinity, and requests against allocatable. Volume topology, host ports, required pod affinity, topology spread and DRA claims are not checked, so a pod on a node-local or zonal volume can be evicted although it can only come back to the same node or zone. |
| `evictSystemCriticalPods` | `false` | Pods with priority 2000000000 or higher are not evicted. |
| `ignorePvcPods` | `false` | Pods with PVCs are evicted. |
| `evictFailedBarePods` | `true` | Pods without an owner are evicted only in phase `Failed`. |
| `evictLocalStoragePods` | from the resource | Pods with `emptyDir` or `hostPath` volumes are protected unless it is `true`. |
| namespaces `d8-*` and `kube-system` | refused by default | Not evicted by any profile. |

DaemonSet pods, mirror and static pods and terminating pods are not evicted either. Pods without a
PDB, pods with ResourceClaims and pods annotated
`descheduler.alpha.kubernetes.io/prefer-no-eviction` are evicted; that annotation only changes the
order. The policy sets no eviction limits, so one cycle may evict any number of pods per node and
per namespace. Every eviction goes through the Eviction API with the pod's default grace period,
so PodDisruptionBudgets are the only cap.

**The `descheduler.alpha.kubernetes.io/evict` pod annotation overrides most of this.** For a pod
with it, whatever the value, the descheduler skips the `d8-*` and `kube-system` refusal, the
priority threshold and the system-critical check, the local-storage, DaemonSet, bare-pod, mirror,
static and terminating checks, and `podLabelSelector`. It still applies `nodeFit`,
`namespaceLabelSelector` and PDBs. Anyone who can annotate a pod can make it evictable, and an
annotated pod that is already terminating stays a candidate, so it may get repeated eviction
requests. When the scope of a profile must hold, use `namespaceLabelSelector`, not
`podLabelSelector`.

## Background evictions

The descheduler runs with the upstream `EvictionsInBackground` feature gate, meant for workloads
whose pods cannot leave at once, such as virtual machines that migrate. The gate is always on in
1.76.13 and later 1.76 releases and in 1.77.1 and later; it is off in 1.77.0 and in every release
before 1.76.13. A cluster on 1.76.13 or a later 1.76 release can update to 1.77.0 and lose it until
1.77.1, so a module that relies on it needs a requirement such as
`requirements.deckhouse: ">= 1.76.13, != 1.77.0"`.

Background eviction applies only to pods that a strategy selects and that pass the checks in "What
the module sets for every profile". In particular, a pod with any `emptyDir` or `hostPath` volume is
skipped unless `evictLocalStoragePods` is `true`; no Event, metric or status reports this, only the
descheduler log (`pod has local storage and is protected against eviction`). For those pods:

1. The pod carries the annotation `descheduler.alpha.kubernetes.io/request-evict-only`.
2. The owning controller answers the eviction with HTTP 429 and a message that contains
   `Eviction triggered evacuation`. Any other 429, such as a PDB rejection or different wording, is
   an ordinary failed eviction.
3. The descheduler records the pod as an eviction in progress and does not ask again while the pod
   is tracked. Tracking expires 10 to 20 minutes after the request. It ends earlier if the pod is
   deleted, completes or loses `request-evict-only`, or if `eviction-in-progress` is removed from it
   after having been set.
4. The controller sets `descheduler.alpha.kubernetes.io/eviction-in-progress` on the pod. That does
   not extend the tracking from step 3, but a pod that carries both annotations is tracked again,
   without expiry, on its next update and whenever the descheduler starts; it then stays tracked
   until it is deleted, completes, or loses either annotation. Between the expiry and that update a
   cycle may request the eviction again. Without `eviction-in-progress` the eviction is requested
   again after the expiry.

Every spec change of any `Descheduler` restarts the descheduler, and after a restart it forgets the
requests whose pods do not carry `eviction-in-progress` yet, so the first cycle of the new process
may request them again. Background evictions leave no Event on the pod and are not counted in
`descheduler_pod_evictions_total`. On 1.77.1 and later the upstream
`descheduler_pods_evicted_total{result="background"}` counts each accepted request, repeats
included; on the 1.76 line (1.76.13 and later) no metric does.

## What breaks, and how far

The CRD checks types, enums and a few ranges. Nothing checks a resource against the descheduler
before it is used, so a mistake surfaces in one of four ways.

**The API server rejects it.** This happens for:

- no strategy key at all (the error names `spec.strategies.lowNodeUtilization`, but any one strategy
  satisfies the rule);
- a `cpu`, `memory` or `pods` threshold that is fractional or outside 0 to 100, and a
  `podRestartThreshold` below 1;
- both `name` and `value` in `priorityClassThreshold`, or neither;
- a value outside an enum, `Exists` or `DoesNotExist` with `values`, `In` or `NotIn` without
  `values`;
- a `matchExpressions` value that is empty, longer than 63 characters, or has no lowercase letter or
  digit, such as `PROD`. For empty values and values like `PROD` the CRD is stricter than
  Kubernetes, and `matchLabels` accepts them. A value longer than 63 characters is invalid in
  Kubernetes too, and `matchLabels` does not check it: in `podLabelSelector` or
  `namespaceLabelSelector` it stops every profile, in `nodeLabelSelector` it becomes `<error>`.

For an administrator this is harmless: the write fails, nothing else changes, and neither the
running descheduler nor other producers are ever affected. For a module that ships the resource it
is not: the whole release fails until the manifest is fixed, and after a Deckhouse restart the
module's failing run also keeps the startup converge from finishing, so the Deckhouse pod stays
NotReady.

**The module stops applying Descheduler changes.** The resource passes the API server, but its
values fail the module's own schema:

- `priorityClassThreshold.value: 0` or `priorityClassThreshold.name: ""`;
- `In` or `NotIn` with `values: []` in `podLabelSelector` or `namespaceLabelSelector`.

From then on no change to any `Descheduler` from any producer reaches the descheduler, deletions
included, and neither does a change of the module settings. The descheduler keeps running the last
good policy, and the `descheduler` Module reports phase `Error` with reason `HookError`.

- To recover, fix or delete the resource. Processing then unblocks on its own: the failed task is
  retried at most 32 seconds later, and the work it held back then runs. Disabling the module also
  unblocks it, at the cost of every profile.
- Until then, do not change the `descheduler` ModuleConfig, except to disable the module. A change
  of its settings queues a descheduler run at the head of Deckhouse's main queue, where it fails and
  is retried without limit, holding up other modules' runs, global hooks and the enabling or
  disabling of modules across the cluster.
- Do not delete objects of the descheduler release either, such as its Deployment or the policy
  ConfigMap. That has the same effect: Deckhouse queues a descheduler run to restore the object,
  that run fails the same way, and the object stays missing until the resource is fixed.
- If Deckhouse restarts, or the module is enabled, while such a resource exists, the converge never
  finishes. After a restart the Deckhouse pod stays NotReady, which holds minor Deckhouse updates,
  and modules that happen to be queued behind descheduler are not deployed. Once the resource is
  fixed or deleted, the Deckhouse pod becomes Ready when the converge finishes.

**The descheduler stops for every profile.** The policy is rendered, but the descheduler refuses to
start, so all profiles stop, at the latest when the descheduler pod next restarts. This happens for:

- a resource name that YAML reads as a number or a boolean: `123`, `012`, `0x1f`, `1e3`, `1.5`,
  `true`, `false`, `yes`, `no`, `on`, `off`, `y`, `n`;
- `lowNodeUtilization` or `highNodeUtilization` enabled without thresholds, a threshold above its
  target, different resource sets in `thresholds` and `targetThresholds`, a threshold that is not a
  number, a threshold below 0 or above 100 on a resource other than `cpu`, `memory` and `pods`;
- `nodeAffinityType: []`;
- a `podRestartThreshold` above 2147483647 that wraps to 0 or below, such as 2147483648;
- a `priorityClassThreshold.name` that does not exist, or a class or value above 2000000000;
- label syntax that the CRD lets through in `podLabelSelector` or `namespaceLabelSelector`, such as
  the key `Bad Key!` or the values `x y` and `a/b`;
- a `nodeLabelSelector` made of a single `Exists` on a key that YAML reads as a number or a boolean.

The resource-wide items apply even when every strategy of the resource is disabled. Helm succeeds
and the module reports itself healthy; see "Signals" for what shows the failure.

**The profile silently does nothing, or the wrong thing.** This covers:

- the `nodeLabelSelector` cases where the profile evicts nothing or the restriction silently
  disappears;
- a priority threshold from 1 to 1000, or a negative one;
- a `lowNodeUtilization` resource that the underutilized nodes do not have;
- `lowNodeUtilization` on node metrics with any other resource, or with a node that has had no
  metrics since the descheduler started;
- a `podRestartThreshold` above 2147483647 that wraps to a positive number (4294967301 becomes 5);
- on 1.76.x, a `namespaceLabelSelector` with only `matchExpressions`, which widens the profile to
  every namespace.

Errors after the descheduler has started, such as a strategy failing in one cycle, are logged and
stay within that strategy and cycle.

## Signals

- `kubectl -n d8-descheduler get configmap descheduler-policy -o yaml` shows the policy the
  descheduler loads; your profile is the entry named after your resource.
- A crash loop of the `descheduler` pod in `d8-descheduler` with `failed to run descheduler server`
  in its log means the policy did not load. The `err` field of that line gives the cause:
  `failed decoding descheduler's policy config` (a name, `nodeLabelSelector` or threshold that YAML
  cannot read), `in profile <name>: ...` (strategy arguments),
  `unable to get priority value from the priority class` or
  `priority threshold can't be greater than 2000000000` (these do not name the profile), or
  `failed to create new descheduler: unable to create "<name>" profile` (pod or namespace
  selectors). With the `extended-monitoring` module enabled, Deployment alerts such as
  `KubernetesDeploymentReplicasUnavailable` also fire for the `descheduler` Deployment.
- `kubectl get module descheduler` with phase `Error` and reason `HookError` means the module has
  stopped applying Descheduler changes. `D8DeckhouseCouldNotRunModuleHook` and
  `D8DeckhouseModuleHookFailsTooOften` fire for the descheduler module's hook, and
  `D8DeckhouseQueueIsHung` for the queue `/modules/descheduler`. `D8DeckhouseCouldNotRunModule`
  fires only when a descheduler module run fails as well, for example after a change of the module
  settings. After a Deckhouse restart, `D8DeckhouseQueueIsHungGlobal` and `D8DeckhousePodIsNotReady`
  fire too.
- Each Eviction request carries the annotations `reason` (`triggered by <profile>/<strategy>:`) and
  `requested-by` (`sigs.k8s.io/descheduler`). They are not stored on the pod; only admission
  webhooks for `pods/eviction`, such as KubeVirt's, and API audit logs that record request bodies
  see them. A successful eviction other than a background one records a Normal Event on the pod with
  the strategy as its reason.
- Since 1.77.0 `descheduler_pod_evictions_total` counts evictions by workload with `profile` set to
  the resource name; a PDB rejection appears as `result="error"`, `reason="too_many_requests"`.

Two resources that match the same pods may evict the same pod twice in one cycle and count it twice.

## Never touch it through v1alpha1

`v1alpha1` is deprecated but still served; the storage version is `v1alpha2`. Conversion between the
two loses almost everything, so read and write the resource only through `deckhouse.io/v1alpha2`:

- A write through `v1alpha1`, even a patch that only adds a label, replaces the spec with the round
  trip: thresholds become 20 and 70, `podRestartThreshold` 100, `constraints` `[DoNotSchedule]`,
  `nodeAffinityType` its default; `podLabelSelector`, `namespaceLabelSelector`,
  `priorityClassThreshold` and `evictLocalStoragePods` are lost, and disabled strategies disappear.
- Such a write also leaves a `deckhouse.io/v1alpha1` entry in `managedFields`. From then on every
  server-side apply through `v1alpha2` fails with HTTP 500:
  `.spec.evictLocalStoragePods: field not declared in schema` while the conversion webhook is up,
  `conversion webhook for deckhouse.io/v1alpha2, Kind=Descheduler failed` while it is down. Before
  applying a module release Deckhouse repairs `managedFields` on its own for an entry left by
  `kubectl edit` (since 1.76.9 also for one left by `deckhouse-controller`); an entry left by
  `kubectl label`, `annotate` or `patch` usually stays, and then the shipping module's release
  fails. One write through `v1alpha2` that is not an apply, such as `kubectl annotate`, clears the
  entry. Resources created before 1.67 carry such an entry from the module itself.
- A resource written as `v1alpha1` keeps only its enabled strategies, with fixed parameters, and its
  node selector. `removeFailedPods` and `removePodsViolatingNodeTaints` are dropped, and a resource
  left with no strategy is ignored with a warning in the Deckhouse log.

Reads and writes of a resource written only through `v1alpha2` never call the conversion webhook, so
webhook outages do not affect them. One exception affects every resource: a cluster upgraded from
1.66 or earlier may still store a `Descheduler` as `v1alpha1`. While it does, an API server restart
during a webhook outage makes every LIST and WATCH of deschedulers fail with HTTP 429, the module's
own watch included, until the webhook is back. If
`kubectl get crd deschedulers.deckhouse.io -o jsonpath={.status.storedVersions}` lists `v1alpha1`,
rewrite each `Descheduler` once through `v1alpha2`, for example with `kubectl annotate`, while the
webhook is healthy. `storedVersions` is never pruned automatically, so it keeps listing `v1alpha1`
after the rewrite: it tells you that such objects may exist, not that they still do.

## Older releases

Earlier releases differ from the sections above in at least these ways:

| Releases | Upstream | Differences |
|---|---|---|
| 1.67.0–1.74 | a snapshot before v0.32.0 | Threshold bounds are not enforced, and the API server fills in 20 (`thresholds`) and 70 (`targetThresholds`) for omitted `cpu`, `memory` and `pods`. An invalid `podLabelSelector` disables only its own profile. A resource written as `v1alpha1` that converts to no strategy stops the module from applying any Descheduler change, as `value: 0` does. In a module release, a key the schema does not declare is pruned, and the object is rejected only if no strategy key is left. |
| 1.75 | v0.34.0 | An invalid `podLabelSelector` disables only its own profile; an invalid `namespaceLabelSelector` crashes the descheduler in the middle of a cycle. |
| 1.76 | v0.35.1 | A `namespaceLabelSelector` with only `matchExpressions` is ignored. No `descheduler_pod_evictions_total`. Background evictions only since 1.76.13. |
| 1.77.0 | v0.36.0 | No background evictions. |

## Differences from the module documentation

| The documentation says | The code does |
|---|---|
| `nodeLabelSelector` selects the nodes to work on. | It only limits the nodes an evicted pod must fit on. |
| `RemovePodsHavingTooManyRestarts` counts init containers, and restarts must exceed the threshold. | Regular containers only; reaching the threshold is enough. |
| `RemoveDuplicates` keeps at most one pod of a controller per node. | It spreads duplicates evenly. |
| Without metrics, usage comes from requests and limits. | Requests only. |
| Without `metrics.k8s.io` the module falls back to requests. | The switch depends on the APIService existing, not on it being available. |
| The owning controller rejects a background eviction with `429 Too Many Requests`. | Only a 429 whose message contains `Eviction triggered evacuation` starts a background eviction; any other 429 is a failed eviction. |
| Background evictions in progress count against the eviction limits. | There are no limits. |
| Pods in `d8-*` and `kube-system`, critical pods, pods with local storage and DaemonSet pods are never evicted. | These are defaults; the evict annotation overrides them, and local storage depends on `evictLocalStoragePods`. |
| The English README enables `RemovePodsViolatingInterPodAntiAffinity` with `highNodeUtilization.enabled`. | It is `removePodsViolatingInterPodAntiAffinity.enabled`. |
| The Russian CRD description has a v1alpha2 `spec.nodeSelector`. | There is none. |
| The module runs every 15 minutes. | Every 5, 15 or 30 minutes, per `deschedulingInterval`. |
| Once enabled, the module works with the default settings. | Nothing runs until a `Descheduler` exists. |

## Rules

1. **Use `deckhouse.io/v1alpha2` for every read, write and patch.** One `v1alpha1` write resets the
   spec and breaks server-side apply of the object.
2. **Start the name with a letter and name the resource after your module.** Never use a name that
   YAML reads as a number, a boolean or null.
3. **Omit a field to get its default; never send it empty.** `nodeLabelSelector: {}`,
   `nodeAffinityType: []`, `values: []`, `priorityClassThreshold.value: 0` and `name: ""` each break
   something: at best the profile silently does nothing, at worst every profile stops or the module
   stops applying Descheduler changes.
4. **Give every enabled utilization strategy its thresholds.** For `lowNodeUtilization`, both maps
   with the same keys, each threshold at or below its target, every value from 0 to 100, and only
   `cpu`, `memory` and `pods` as keys on a cluster that has, or may get, `metrics.k8s.io`.
5. **Check label keys and values yourself.** The CRD lets malformed pod and namespace selectors
   through, and they stop every profile. It also rejects legal values without a lowercase letter or
   digit, such as `PROD`; select on those through `matchLabels`.
6. **Scope with `namespaceLabelSelector`,** and include at least one `matchLabels` entry if you
   support 1.76, where a selector with only `matchExpressions` is ignored. `podLabelSelector` can be
   overridden by pod authors, and `nodeLabelSelector` does not scope at all.
7. **Pick a priority threshold above the classes you want evicted**, and reference only classes that
   always exist, such as the Deckhouse ones.
8. **Make sure the target pods can be evicted at all.** Pods with `emptyDir` or `hostPath` volumes
   need `evictLocalStoragePods: true`.
9. **Protect workloads with PodDisruptionBudgets.** They are the only limit on evictions.
10. **Change the spec only when you mean to.** Every spec change, creation or deletion restarts the
    shared descheduler, and the new process runs a cycle for all producers right away.
11. **Delete the resource to switch the profile off.** Disabling all strategies leaves a running
    no-op profile.

## Checking your template

- [ ] `apiVersion: deckhouse.io/v1alpha2`, guarded template, `requirements.deckhouse` covers the
      newest field used and background evictions if you rely on them.
- [ ] The name starts with a letter and is none of `y`, `n`, `yes`, `no`, `on`, `off`, `true`,
      `false`, `null`.
- [ ] No empty lists, no empty `nodeLabelSelector`, no zero or empty priority threshold.
- [ ] Utilization strategies have complete thresholds from 0 to 100 (for `lowNodeUtilization`, each
      at or below its target), and `lowNodeUtilization` thresholds use only `cpu`, `memory` or
      `pods`.
- [ ] Every selector key and value is a valid Kubernetes label key or value.
- [ ] The priority threshold leaves the intended classes evictable; a named class always exists.
- [ ] If the target pods have `emptyDir` or `hostPath` volumes, `evictLocalStoragePods` is `true`.
- [ ] The workloads in scope have PDBs, and those that must stay have no `evict` annotation.
- [ ] After applying: your profile is in `descheduler-policy`, the descheduler pod created after
      your change is Ready and not restarting, and `kubectl get module descheduler` is not in phase
      `Error`.
