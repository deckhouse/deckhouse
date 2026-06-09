# Multi-namespace projects — design

> Status: **draft / discussion.** How a Deckhouse project evolves from "project == one namespace"
> to a project that its administrator can split into several namespaces — *optionally*, because not
> every project needs self-service. Sibling of
> [Cluster object grants](./CLUSTER_OBJECT_GRANTS_DESIGN.md) (per-project resource grants/quotas).

## Problem

Today a project is exactly one namespace (1:1): the `Project` creates a namespace with the same
name. We want a project whose admin can create **several** namespaces inside it. But self-service
is not free — for many projects a single namespace is simpler and safer. So the **mode is chosen at
project creation**, and single-namespace stays the default.

## Modes

Chosen once, at creation, via `Project.spec.namespaces.selfService`:

- **Single-namespace (default, classic).** Project ⇔ one namespace named after the project. Current
  behaviour; no `ProjectNamespace`, no prefixing. Nothing changes for existing projects.
- **Multi-namespace (self-service).** The project is a logical container. Its admin creates workload
  namespaces via [`ProjectNamespace`](#projectnamespace). A locked-down
  [control namespace](#the-control-namespace) named after the project holds management objects only.

## Project spec changes (first-class fields)

Inspired by Capsule's `Tenant`. New/changed `Project.spec`:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
  labels:
    environment: production        # used by ClusterResourceGrantPolicy.projectSelector
spec:
  projectTemplateName: default
  parameters: {}
  # Who administers the project; the controller binds them in every project namespace + control ns.
  owners:
  - kind: Group                    # User | Group | ServiceAccount
    name: team-a-admins
    accessLevel: Admin             # maps to a Deckhouse access-level / ClusterRole
  # Namespace policy for the project.
  namespaces:
    selfService: true              # false (default) = single-namespace classic project
    prefix: team-a                 # mandatory in multi-ns; default = project name
    max: 10                        # max namespaces in the project (Capsule namespaceOptions.quota)
  # Pod Security Standard for the project (effective enforce = max(cluster floor, this)).
  podSecurityStandard: Restricted          # Privileged | Baseline | Restricted
  # Default pod placement for the project — stamped onto every project namespace (see Default features).
  nodeSelector:
    node-pool: tenants
  tolerations:
  - key: dedicated
    operator: Equal
    value: tenants
    effect: NoSchedule
  # Per-project feature toggles wired to other modules (no-op if that module is off) — see Default features.
  features:
    vulnerabilityScanning: true            # operator-trivy scans the project's namespaces
    monitoring: true                       # tenant monitoring: scrape + PodMonitor/PrometheusRule + Grafana
  # Project-total COMPUTE quota (native ResourceQuota keys), distributed to namespaces.
  # OBJECT quota (per-class storage/LB/… limits) lives on a separate ClusterResourceGrant — see Quotas.
  quota:
    compute:
      requests.cpu: "40"
      requests.memory: 80Gi
      limits.cpu: "60"
      limits.memory: 120Gi
      pods: "400"
```

| Field | Why first-class |
|-------|-----------------|
| `owners[]` | identity is project-level, must apply across all the project's namespaces; today RBAC is template-rendered, which does not fit a growing set of namespaces |
| `namespaces.selfService` | mode switch (single vs multi) |
| `namespaces.prefix` / `max` | naming + namespace-count limit are project-level invariants enforced on every namespace create |
| `podSecurityStandard` | first-class PSS level (`Privileged`/`Baseline`/`Restricted`); sets `pod-security.kubernetes.io/enforce`, effective = max(cluster floor, this) |
| `nodeSelector` / `tolerations` | default pod placement for the project; stamped onto every project namespace (see [Default features](#default-features-for-projects-baseline)) |
| `features.vulnerabilityScanning` / `features.monitoring` | first-class toggles wired to `operator-trivy` / the monitoring stack for the project's namespaces (labels + RBAC); no-op if that module is off |
| `quota` | the project-total **compute** budget (native `ResourceQuota` keys) distributed per namespace; object quota lives on `ClusterResourceGrant` (see [Quotas](#quotas)) |

Per-namespace *rendered* resources (NetworkPolicy, LimitRange, security profile, default RBAC) stay
in `ProjectTemplate` (see [ProjectTemplate in multi-namespace](#projecttemplate-in-multi-namespace)).

## ProjectNamespace

In multi-namespace mode the project admin orders a workload namespace with a `ProjectNamespace` — a
**namespaced** resource that is valid **only in the project's main (control) namespace** (so the
admin manages it with ordinary namespace-scoped RBAC, no cluster permissions). Admission **rejects** a
`ProjectNamespace` created in any other namespace.

`ProjectNamespace` is both the **namespace claim** and the **quota claim** for that namespace: its
`spec.quota` (a `compute`/`objects` slice) is this namespace's portion of the project budget — the
controller renders a native `ResourceQuota` from `compute` and a per-namespace `ClusterResourceGrant` from
`objects`, keeping `Σ slices ≤ pool` (compute pool: `Project.spec.quota.compute`; object pool: the
project `ClusterResourceGrant`; see [Quotas](#quotas)).

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectNamespace
metadata:
  name: backend                 # suffix; final namespace = <prefix>-<suffix>
  namespace: team-a             # the project control namespace
spec:
  quota:                        # this namespace's slice of the project total (optional)
    compute:
      requests.cpu: "8"
      requests.memory: 16Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 50Gi
      loadbalancerclasses:
        external:
          services: 2
status:
  namespace: team-a-backend     # the namespace the controller created
  appliedQuota:                 # the full slice actually granted (every entry clamped to the remaining project budget)
    compute:
      requests.cpu: "8"
      requests.memory: 16Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 50Gi
      loadbalancerclasses:
        external:
          services: 2
  conditions: []
```

The controller creates `Namespace` `<prefix>-<suffix>` (here `team-a-backend`), labels it
`projects.deckhouse.io/project=team-a`, renders the per-namespace `ProjectTemplate` resources into
it (network isolation, limits, security — see [Network isolation and security](#network-isolation-and-security)),
renders a native `ResourceQuota` from `spec.quota.compute`, and a per-namespace `ClusterResourceGrant` from
`spec.quota.objects`. `status.appliedQuota` reflects the **whole** slice that was applied (every entry
of both `compute` and `objects`, clamped per-key to what remains of the project budget — not just CPU).
Deleting the `ProjectNamespace` deletes the namespace and releases its slice back to the project
budget.

## ProjectRoleBinding

Project-wide access without touching each namespace. `Project.spec.owners` is the shorthand for the
project administrators; `ProjectRoleBinding` is the general form (any subject, any role) and is what
a project admin uses for self-service team access. It is a **namespaced** resource valid **only in
the project's main (control) namespace** (admission rejects it elsewhere); the controller fans out a
`RoleBinding` into **every** namespace of the project.

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectRoleBinding
metadata:
  name: developers
  namespace: team-a             # the project control namespace
spec:
  subjects:
  - kind: Group
    name: team-a-developers
  accessLevel: Editor           # a Deckhouse access level, or use roleRef to a ClusterRole
status:
  namespaces: [team-a, team-a-backend, team-a-frontend]   # where bindings were created
  conditions: []
```

When a new namespace is added to the project, the controller extends existing `ProjectRoleBinding`s
into it (so access follows the project, not a fixed namespace list). A project admin may only grant
roles up to their own access level (escalation guard).

## Naming and prefix

Multi-namespace projects need collision-free namespace names:

- `Project.spec.namespaces.prefix` is **mandatory** (default = project name). The control namespace
  is `<prefix>`; every workload namespace is `<prefix>-<suffix>`.
- **Reserved prefixes are forbidden:** `d8-`, `kube-`, `upmeter-`, `default`, and any existing
  system namespace.
- **Prefix-collision validation** (admission on `Project` create/update). Reject a prefix `P` if it
  **overlaps** anything, where overlap = one string is a prefix of the other:
  - `P` overlaps a reserved prefix (e.g. reserved `d8` ⇒ `d8-test` is rejected);
  - `P` overlaps another project's prefix (`team` vs `team-a` — either way);
  - any existing namespace **not** belonging to this project equals `P` or starts with `P-`.
  Implementation: keep an index of project prefixes; the webhook checks `P` against
  (reserved ∪ other projects' prefixes ∪ existing namespace names) for the prefix-of relation.
- The `ProjectNamespace` suffix is validated so the final name is RFC1123 and within the length
  limit.

## ProjectTemplate in multi-namespace

Today a `ProjectTemplate` renders its resources into the single project namespace. In
multi-namespace mode:

- Template resources are rendered into **each workload namespace** as it is created — the template
  is parameterized by the **namespace**, not the project (so `.namespace` replaces the implicit
  single namespace).
- Split by altitude:
  - **per-namespace** (NetworkPolicy, LimitRange, default RBAC, security profile, the quota slice) —
    rendered into every namespace;
  - **per-project** (owner bindings, compute quota, namespace policy) — moved to `Project.spec`
    (above); object quota to the project `ClusterResourceGrant` — not the template.

Open: do we render the whole template per-namespace, or let the template author mark resources
per-namespace vs per-project? Leaning: render per-namespace by default; project-level concerns are
`Project.spec` fields, not template objects.

## The control namespace

In multi-namespace mode the controller still creates a namespace named after the project (the
**control namespace**), but it is **not a workload namespace**:

- a **validating webhook** allows only a whitelist of kinds in it — `ProjectNamespace`,
  `ProjectRoleBinding`, the project `ClusterResourceGrant` (object-quota pool), the read-only `AvailableClusterResource`
  catalog — and **rejects workloads** (Pods, Deployments, Services, PVCs, …). Conversely,
  `ProjectNamespace` and `ProjectRoleBinding` are valid **only** here and rejected in any other namespace;
- it is the project admin's console: order namespaces, `kubectl get available`, manage owner-scoped
  bindings.

In single-namespace mode there is no separate control namespace — the one project namespace is the
workload namespace, as today.

## Network isolation and security

Both are **project-scoped** (must cover every namespace of the project) and both rely on labels the
controller puts on project namespaces. Two prerequisites the controller guarantees on each project
namespace:

- `projects.deckhouse.io/project=<project>` — identifies the project (per-project isolation);
- **propagated `Project` labels** (e.g. `environment=production`) — copied from the `Project` onto its
  namespaces (Capsule's `additionalMetadata` idea), so an admin can target a *class* of projects'
  namespaces by label.

But the two are produced by **different** actors/triggers — that is the part the earlier draft got
wrong:

### Network isolation — rendered by the controller (trigger: namespace creation)

Network isolation is part of the `ProjectTemplate`. The **controller renders it into a namespace
when that namespace is created** (on `ProjectNamespace` reconcile), and re-renders into all the
project's namespaces when the template changes. The project admin does **not** write NetworkPolicies.

The only change multi-namespace forces on the *existing* template policy: today it isolates a
**single** namespace; in a multi-namespace project sibling namespaces must reach each other, so the
intra-project rule must select **all namespaces of the project by label** instead of only the local
one — while keeping everything the template already allows (default-deny, DNS/system egress, ingress
controllers). The load-bearing change is just the selector:

```yaml
# inside the template-rendered NetworkPolicy — the rule that permits intra-project traffic:
- from:
  - namespaceSelector:
      matchLabels:
        projects.deckhouse.io/project: team-a   # every namespace of THIS project, not only the local one
```

A standalone, *valid* NetworkPolicy must also keep DNS/system egress (otherwise pods cannot resolve
names) — that already exists in the current template and is unchanged; multi-namespace only widens
the intra-project selector from "this namespace" to "this project". So the existing template policy is
not replaced, only its intra-project selector is broadened.

### Security policies — pre-created by the admin, matched by label (like grants)

PSS / seccomp / capabilities are **not** rendered per project. The cluster admin **pre-creates**
`SecurityPolicy` (admission-policy-engine) once and **attaches it to projects by a label selector** —
exactly the "author once, match by label" model of `ClusterResourceGrantPolicy`. The controller's only job is
to ensure the matching label is on every project namespace (via the propagated `Project` labels
above).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: restricted-production        # admin-managed, reused across many projects
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          environment: production     # propagated from the Project onto its namespaces
  policies:
    allowPrivileged: false
    runAsUser:
      rule: MustRunAsNonRoot
    seccompProfiles:
      allowedProfiles:
      - RuntimeDefault
```

Summary: **network isolation = our rendered template** (trigger: namespace create / template change;
selector widened to the project); **security = admin-pre-created `SecurityPolicy` matched onto
projects by label** (same pattern as grants). The controller owns only the namespace labels that make
both work.

## Default features for projects (baseline)

A cluster admin wants some things to hold for projects **by default** — PSS Restricted everywhere, a
set of `SecurityPolicy`/`OperationPolicy`, pods pinned to certain nodes. These split by **scope** (all
projects / a class / one project) and **strength** (a *default* a project may override vs an *enforced
floor* it may only make stricter). The mechanism is uniform: **the controller always stamps service
labels on every project namespace, and policies select those labels.** Because the membership label is
*always present*, a policy that selects it covers **all** projects and is **template-independent** — a
careless `ProjectTemplate` cannot bypass it.

Three layers, from strongest to most local:

1. **Floor — all projects, cannot be weakened.** The cluster admin authors a `SecurityPolicy` / PSS /
   `OperationPolicy` whose `namespaceSelector` matches `projects.deckhouse.io/project` (Exists)
   (usually also `namespace-role: workload`, to skip the control namespace). One object, every project,
   no per-project work. The controller only guarantees the label.
2. **Class — projects sharing a propagated label.** The same, but the selector matches a propagated
   `Project` label (e.g. `environment: production`). The admin labels the `Project`; the controller
   propagates an **allowlisted** set of keys onto the namespaces.
3. **Per-project — first-class `Project.spec` fields (the per-project layer).** `podSecurityStandard`,
   `nodeSelector`/`tolerations`, and `features.*` toggles; `ProjectTemplate` renders any further
   per-namespace extras. These may add or strengthen, never weaken the floor.

Two kinds of feature, by **who acts on the label**:

- **enforced by this controller** (PSS, placement) — we write the namespace label/annotation directly;
- **delegated to another module** (`features.*`) — we only stamp the label/annotation + grant RBAC that
  the *other* module consumes; the feature lives there, so the toggle is a **no-op if that module is
  off**.

Concretely for the motivating cases:

- **`SecurityPolicy` on all project namespaces** → a `SecurityPolicy` selecting the membership label
  (floor). Exactly the "author once, match by label" model, with the always-present label.
- **PSS Restricted always** → the controller sets `pod-security.kubernetes.io/enforce` from
  `Project.spec.podSecurityStandard`; the **effective level = max(floor, project request)**, so a
  project may go stricter (`restricted` over a `baseline` floor) but never weaker.
- **Pods always to certain nodes** → `Project.spec.nodeSelector`/`tolerations`; the controller stamps
  the `scheduler.alpha.kubernetes.io/node-selector` annotation on each project namespace (the
  `PodNodeSelector` admission plugin injects it into pods). For a *cluster-wide* pin, an
  admission-policy mutation selected by the membership/class label does the same fleet-wide.
- **Vulnerability scanning** → `Project.spec.features.vulnerabilityScanning`; the controller marks the
  project's namespaces for `operator-trivy` and grants the tenant read on the namespaced
  `VulnerabilityReport`s. We don't scan — `operator-trivy` does.
- **Monitoring** → `Project.spec.features.monitoring`; the controller marks the namespaces for scraping
  and grants the tenant RBAC to create `PodMonitor`/`ServiceMonitor`/`PrometheusRule` and a Grafana
  scope. The metrics pipeline is the monitoring stack's, not ours.

The **control namespace** is excluded from workload-oriented defaults (node placement, intra-project
`NetworkPolicy`, scanning) via `namespace-role: control` — it holds management objects, not pods.

## Service labels and annotations

The controller stamps a consistent set of labels so policies can target namespaces, ownership is
clear, and rendered objects trace back to their source (for updates and GC).

**On every controller-managed object** (project namespaces, `AvailableClusterResource`, rendered `ClusterResourceGrant`,
rendered `RoleBinding`/`NetworkPolicy`/`ResourceQuota`):

| label | value | purpose |
|-------|-------|---------|
| `projects.deckhouse.io/project` | `<project>` | **belongs to project** — the join key; `get -A -l projects.deckhouse.io/project=team-a` shows everything for a project; controller GC keys on it |
| `heritage` | `deckhouse` | Deckhouse-managed object (existing convention) |
| `module` | `multitenancy-manager` | owning module — the protective admission policy rejects writes to objects carrying this by non-controller service accounts |

**On project namespaces, additionally:**

| label / annotation | value | purpose |
|--------------------|-------|---------|
| `kubernetes.io/metadata.name` | `<namespace>` | set by kube-apiserver; lets a selector target a namespace by name |
| `projects.deckhouse.io/namespace-role` | `control` \| `workload` | control-namespace lockdown; lets policies target workload namespaces only |
| `pod-security.kubernetes.io/enforce`\|`warn`\|`audit` | `<level>` | PSS; effective `enforce` = max(floor, project request) |
| propagated `Project` label keys (allowlist), e.g. `environment` | from `Project.metadata.labels` | target a *class* of projects by label |
| `scheduler.alpha.kubernetes.io/node-selector` (annotation) | `<selector>` | default node placement, from `Project.spec.nodeSelector` |
| the label `operator-trivy` selects (e.g. `security.deckhouse.io/vulnerability-scan`) | `"true"` | set when `features.vulnerabilityScanning` — opts the namespace into CVE scanning |
| the scrape label the monitoring stack selects (e.g. `monitoring.deckhouse.io/enabled`) | `"true"` | set when `features.monitoring` — opts the namespace into scraping |

**Name / back-reference labels** (on fanned-out objects, value = the source object's name — for update
and GC):

| label | on | points to |
|-------|----|-----------|
| `projects.deckhouse.io/namespace-claim` | the created workload `Namespace` | the `ProjectNamespace` that claimed it (delete claim ⇒ delete namespace) |
| `projects.deckhouse.io/project-role-binding` | each rendered `RoleBinding` | the `ProjectRoleBinding` it came from |

**Referenced, author-defined (not ours)** — labels on granted cluster objects that grant selectors
match: e.g. `shared: "true"` (`allowedSelector`), `rbac.deckhouse.io/tenant-bindable`,
`storageclass.deckhouse.io/system` (`excluded`). These live on the cluster objects, set by their
authors, not by this module.

Propagation is an **allowlist** of `Project` label keys (configured), not "all labels", so a label can
never accidentally match a privileged policy selector.

## Quotas

A project has a **total budget** distributed across its namespaces, in two parts by mechanism:

- **compute — native, on `Project.spec.quota.compute`.** `requests.cpu`/`memory`, `limits.*`, `pods`,
  raw `count/<resource>`. The controller renders a native `ResourceQuota` into each namespace. This
  *is* the natural `ResourceQuota` model, so it stays native.
- **objects — ours, on `ClusterResourceGrant`.** Per-class limits keyed by **grantable-resource → granted name
  (or `*`) → measure** (storage per StorageClass, count per LoadBalancerClass/IngressClass, …). The
  pool is a `ClusterResourceGrant` in the control namespace; the controller renders a read-only `ClusterResourceGrant` into
  each workload namespace with usage. Full model in the
  [cluster object grants design](./CLUSTER_OBJECT_GRANTS_DESIGN.md#clusterresourcegrant).

```yaml
# compute pool — on the Project
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
spec:
  quota:
    compute:
      requests.cpu: "40"
      requests.memory: 80Gi
      limits.cpu: "60"
      limits.memory: 120Gi
      pods: "400"
---
# object pool — a ClusterResourceGrant in the control namespace (cluster-admin-writable only)
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrant
metadata:
  name: objects
  namespace: team-a
spec:
  objects:
    storageclasses:
      "*":                       # all storage classes together
        requests.storage: 1Ti
      fast:                      # the "fast" class, tighter
        requests.storage: 200Gi
    loadbalancerclasses:
      external:
        services: 5              # the "5 external LBs" case — no native per-class key
      internal:
        services: -1             # unlimited
```

**Why object limits are ours even when Kubernetes has a native key.** Native `ResourceQuota` *can*
cap storage per class (`fast.storageclass.storage.k8s.io/requests.storage`) but **cannot** cap per
`LoadBalancerClass` or per `IngressClass` (it only knows the *total* `services.loadbalancers`,
`count/ingresses`). So all per-class object limits are ours and uniform on `ClusterResourceGrant`, regardless of
whether a native key happens to exist.

| Limit | Lives in |
|-------|----------|
| cpu/memory (requests/limits), pods, raw `count/<resource>` | `Project.spec.quota.compute` (native) |
| storage per StorageClass (`*` and per class) | `ClusterResourceGrant.spec.objects.storageclasses` |
| count per LoadBalancerClass (e.g. 5 external) | `ClusterResourceGrant.spec.objects.loadbalancerclasses` |
| count per IngressClass / per custom granted object | `ClusterResourceGrant.spec.objects.<resource>` |
| **namespace count** | `Project.spec.namespaces.max` (enforced on namespace create) |

**Pool, slice, RBAC.** Both budgets are project pools shared across the project's namespaces. The
**pool** is cluster-admin-authored (`Project.spec.quota.compute` and the control-namespace
`ClusterResourceGrant.spec` — both writable by cluster admin only, so a tenant can never raise the total). The
project admin optionally carves **per-namespace slices** via `ProjectNamespace.spec.quota`
(`Σ slices ≤ pool`). The controller renders the native `ResourceQuota` and the read-only per-namespace
`ClusterResourceGrant` into each workload namespace (tenant-visible). Full RBAC table in the
[grants design](./CLUSTER_OBJECT_GRANTS_DESIGN.md#quotas).

### Usage accounting — per-namespace detail, rolled up to the project

Usage is counted at the namespace level and rolled up so the whole budget is visible:

- **per namespace (detail)** — `compute` is the native `ResourceQuota.status.used`; `objects` is that
  namespace's rendered `ClusterResourceGrant.status`;
- **compute rollup** — `Project.status.quota` carries `total` (overall consumption) + a `namespaces[]`
  array, compute `used` summed from each namespace's `ResourceQuota.status.used`;
- **object rollup** — the control-namespace `ClusterResourceGrant.status` carries the project total
  (`projectUsed` vs limit) for every object measure.

```yaml
# compute rollup on the Project
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
status:
  quota:
    total:                       # overall project consumption (compute)
      compute:
      - name: requests.cpu
        limit: "40"
        used: "26"               # Σ of per-namespace ResourceQuota.status.used
      - name: requests.memory
        limit: 80Gi
        used: 51Gi
    namespaces:                  # per-namespace breakdown (compute)
    - namespace: team-a-backend
      compute:
      - name: requests.cpu
        limit: "24"
        used: "16"
    - namespace: team-a-frontend
      compute:
      - name: requests.cpu
        limit: "16"
        used: "10"
```

Object usage lives on `ClusterResourceGrant` (project total in the control-namespace object; per-namespace in
each workload namespace's rendered object) — see the
[grants design](./CLUSTER_OBJECT_GRANTS_DESIGN.md#clusterresourcegrant).

A compute `ResourceQuota` makes Kubernetes require `requests`/`limits` on pods; the defaults come from
the `LimitRange` the `ProjectTemplate` already renders — that is the standard Kubernetes mechanism,
nothing new here.

## Cluster object grants attach by label

Quota lives on the `Project` (compute) and `ClusterResourceGrant` (objects); **availability** — *which* cluster
objects a project may use and the per-project default — is authored separately as a `ClusterResourceGrantPolicy`
and **attaches to a project by label**. A grant's `projectSelector` matches the **Project's labels**; the controller expands the
matched Projects to their namespaces and materializes availability there. This is the same
"author once, match by label" model used for `SecurityPolicy` ([above](#security-policies--pre-created-by-the-admin-matched-by-label));
the full grant model is in the [cluster object grants design](./CLUSTER_OBJECT_GRANTS_DESIGN.md).

```yaml
# A reusable preset: attaches to every Project labelled environment=production.
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production
spec:
  projectSelector:
    matchLabels:
      environment: production    # matches Project.metadata.labels
  resources:
  - resourceName: storageclasses  # allow-list + default only (quota is on the Project)
    allowed:
    - standard
    default: standard
```

So to give a project a set of available resources you **label the Project** (e.g.
`environment: production`) and the matching grants apply — no per-project grant authoring needed.
Quota is **optional and orthogonal**: a resource may be made available with no quota at all (many
have nothing to measure), and a `ClusterResourceGrant` entry without a grant making the resource available does
nothing.

## End-to-end example

A production, multi-namespace project: complex compute/storage quota, owners, two workload
namespaces with quota slices, a self-service developer binding, and cluster-object grants (storage +
load balancers) — network isolation and PSS come from the template/policies above.

```yaml
# 1. The project: mode, prefix, owners, and the project-total native quota.
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
  labels:
    environment: production
spec:
  projectTemplateName: default
  namespaces:
    selfService: true
    prefix: team-a
    max: 10
  owners:
  - kind: Group
    name: team-a-admins
    accessLevel: Admin
  podSecurityStandard: Restricted   # first-class PSS (>= cluster floor)
  nodeSelector:
    node-pool: tenants
  features:
    vulnerabilityScanning: true     # operator-trivy on the project's namespaces
    monitoring: true                # tenant monitoring (scrape + monitors + Grafana)
  quota:
    compute:                        # native ResourceQuota keys (objects go on ClusterResourceGrant below)
      requests.cpu: "40"
      limits.cpu: "60"
      requests.memory: 80Gi
      limits.memory: 120Gi
      pods: "400"
      count/jobs.batch: "50"
---
# 2a. Object-quota pool — a ClusterResourceGrant in the control namespace (cluster-admin-writable only).
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrant
metadata:
  name: objects
  namespace: team-a
spec:
  objects:
    storageclasses:
      "*":
        requests.storage: 1Ti
      fast:
        requests.storage: 200Gi
    loadbalancerclasses:
      external:
        services: 5
      internal:
        services: -1
---
# 2b. Which cluster objects this project may use (storage / load-balancer classes and defaults) are
#     authored SEPARATELY — see the cluster object grants design (CLUSTER_OBJECT_GRANTS_DESIGN.md).
#     The grant does allow-list + default only; the per-class limits live on the ClusterResourceGrant above.
#     A grant attaches to this project by matching its labels:
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production
spec:
  projectSelector:
    matchLabels:
      environment: production
  # resources / allow-list / default — see the cluster object grants design.
---
# 3. Project admin orders two workload namespaces, each claiming a quota slice (Σ ≤ project total).
apiVersion: deckhouse.io/v1alpha2
kind: ProjectNamespace
metadata:
  name: backend
  namespace: team-a
spec:
  quota:
    compute:
      requests.cpu: "24"
      requests.memory: 48Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 150Gi
      loadbalancerclasses:
        external:
          services: 3
---
apiVersion: deckhouse.io/v1alpha2
kind: ProjectNamespace
metadata:
  name: frontend
  namespace: team-a
spec:
  quota:
    compute:
      requests.cpu: "16"
      requests.memory: 32Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 50Gi
      loadbalancerclasses:
        external:
          services: 2
---
# 4. Project admin grants the developers team Editor across the whole project (all namespaces).
apiVersion: deckhouse.io/v1alpha2
kind: ProjectRoleBinding
metadata:
  name: developers
  namespace: team-a
spec:
  subjects:
  - kind: Group
    name: team-a-developers
  accessLevel: Editor
```

Result: `team-a` (control), `team-a-backend`, `team-a-frontend`; compute/storage capped per
namespace and summed within the project total; LB and storage-class usage capped and defaulted by
the grant; developers are Editors everywhere in the project; namespaces talk to each other but are
isolated from other projects; PSS enforced. A tenant sees `kubectl get available storageclasses -n
team-a-backend`.

## Migration (existing projects)

Existing projects are single-namespace (project == namespace) and must keep working untouched.

1. **Additive, defaulted API.** Every new `Project.spec` field is optional and its default reproduces
   today's behaviour: `namespaces.selfService` defaults to `false` ⇒ existing projects stay
   single-namespace (no prefix, no control namespace, no `ProjectNamespace`); empty `owners`/`quota`
   ⇒ RBAC and `ResourceQuota` stay exactly as `ProjectTemplate` renders them now. Existing projects
   need **zero changes**. If a new API version is cut, the projects conversion webhook (already
   present) sets `selfService: false` on converted objects.

2. **Single → multi-namespace is an explicit, controlled opt-in — never an automatic flip.** A live
   project's namespace already holds workloads, so it cannot silently become a locked control
   namespace. On migration:
   - the existing namespace is **adopted** as the project's first/default workload namespace (an
     auto-created `ProjectNamespace` represents it); `prefix` defaults to the project name;
   - for **migrated** projects the project-name namespace **stays a workload namespace** (not locked)
     — the strict management-only control namespace applies to projects **created** in
     multi-namespace mode (they start empty). This is the documented compatibility compromise; a
     later "strict" step can move a migrated project to a separate locked control namespace once its
     workloads are relocated;
   - further namespaces are `<prefix>-<suffix>` as usual.

3. **Quota / RBAC handover.** Until `Project.spec.quota` / `owners` are set, the
   `ProjectTemplate`-rendered `ResourceQuota` / RBAC stay authoritative. When set, the controller
   takes over the per-namespace `ResourceQuota` and **adds** owner bindings (it does not delete
   template RBAC). The precedence is documented so the switch is predictable.

4. **Prefix validation vs the existing cluster.** Collision validation treats all current namespaces
   as the existing set. A project migrating to multi-namespace must have a `prefix` (= its name) that
   passes the reserved/collision check; a project whose name collides with a reserved prefix or
   another namespace cannot go multi-namespace until resolved (rare edge case).

5. **Mode is set at creation; the migration above is the only sanctioned way to flip it** (no free
   toggling), so the transition is always an explicit, reviewed operation.

## Decisions

- **Two modes, chosen at creation**: single-namespace (default, unchanged) vs multi-namespace
  (self-service). Self-service is opt-in — it is not forced on projects that do not need it.
- **`ProjectNamespace`** (namespaced, in the control namespace) is how a project admin orders a
  namespace in multi-namespace mode; deleting it removes the namespace.
- **`ProjectNamespace` and `ProjectRoleBinding` are valid only in the project's main (control)
  namespace** — admission rejects them in any other namespace.
- **Mandatory prefix** in multi-namespace mode (default = project name); **collision validation**
  against reserved prefixes, other projects' prefixes and existing namespaces using the prefix-of
  relation.
- **Control namespace** = project name, locked by admission to management-only kinds; no workloads.
- **Project spec gains first-class** `owners`, `namespaces` (`selfService`/`prefix`/`max`) and
  `quota`; per-namespace rendered resources stay in `ProjectTemplate`.
- **ProjectTemplate renders per-namespace** in multi-namespace mode; project-level concerns move to
  `Project.spec`.
- **Quotas, by mechanism**: compute → `Project.spec.quota.compute` (native `ResourceQuota`, per
  namespace); objects → `ClusterResourceGrant` (`spec` pool in the control namespace + rendered read-only per
  namespace). Both cluster-admin-authored pools; project admin carves per-NS slices via
  `ProjectNamespace.spec.quota` (`Σ ≤ pool`). The grant does allow-list + default only. `max`
  namespaces enforced on create.
- **Availability via grants attaches by label**: a `ClusterResourceGrantPolicy.projectSelector` matches the
  Project's labels (reusable preset, like `SecurityPolicy`). Quota is optional and orthogonal to
  availability.
- **Usage rollup**: compute on `Project.status.quota` (`total` + `namespaces[]`, summed from per-NS
  `ResourceQuota.status.used`); objects on `ClusterResourceGrant.status` (project total in the control namespace,
  per-NS in each rendered `ClusterResourceGrant`).
- **A compute `ResourceQuota` is always rendered together with a `LimitRange`** (defaults + min/max
  from the template), because Kubernetes rejects pods without requests/limits under a compute quota.
- **The controller propagates `Project` labels onto its namespaces**, so network isolation and
  admin-authored `SecurityPolicy` can target a class of projects by label.
- **Network isolation is rendered by the controller** (from the template, per namespace, on creation)
  with a project-scoped selector; **`SecurityPolicy` is pre-created by the admin and matched by
  label**, like grants.
- **Default features = floor + class + per-project**, all via label selection. The floor (a policy
  selecting the always-present membership label) cannot be weakened by a template or project; PSS
  effective `enforce` = max(floor, request); the control namespace is excluded via `namespace-role`.
- **First-class `Project.spec` feature fields**: `podSecurityStandard` and `nodeSelector`/`tolerations`
  (enforced by this controller); `features.vulnerabilityScanning` / `features.monitoring` (delegated —
  the controller only wires labels + RBAC to `operator-trivy` / the monitoring stack; no-op if that
  module is off). We never implement scanning or monitoring ourselves.
- **Default pod placement** via `Project.spec.nodeSelector`/`tolerations` → the
  `scheduler.alpha.kubernetes.io/node-selector` annotation on each project namespace.
- **Service labels** (see [Service labels and annotations](#service-labels-and-annotations)): every
  managed object carries `projects.deckhouse.io/project` (belongs-to), `heritage: deckhouse`,
  `module: multitenancy-manager`; namespaces also carry `namespace-role` and PSS labels; fanned-out
  objects carry name back-reference labels for update/GC. Project-label propagation is an allowlist.

## Open questions

- `ProjectNamespace`: namespaced in the control namespace (leaning) vs cluster-scoped.
- Prefix: force `prefix == project name`, or allow a custom prefix with the collision check?
- `ProjectTemplate`: render the whole template per-namespace, or add per-namespace/per-project
  markers to template resources?
- Migrated projects keep the project-name namespace as a workload namespace (not locked) — do we
  offer a later "strict" step to relocate workloads and lock it, or leave migrated projects with the
  relaxed control namespace forever? (see [Migration](#migration-existing-projects))
- Exact allowed-kinds whitelist for the control namespace.
- How `Project.spec.quota` reconciles with the `ResourceQuota` that `ProjectTemplate` renders today
  (who owns the per-namespace `ResourceQuota` in each mode).
