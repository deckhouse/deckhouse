---
title: "The multitenancy-manager module: frequently asked questions"
---

## Grants on cluster resources

### My PVC was rejected with "resource not available to project" — what do I do?

The `StorageClass` you referenced in `spec.storageClassName` is not in your project's allow-list. Check
what your project may use:

```shell
d8 k get available storageclasses -n <project-name> -o yaml
```

If the name you need is missing, ask the cluster administrator to add it to the `ClusterResourceGrantPolicy`
that matches your project (or to use a name that is listed). The same applies to other granted resources
(`ClusterIssuer`, `ClusterRole`, `LoadBalancerClass`) — check the corresponding `AvailableClusterResource`.

### How do I find out which grant policies affect my project?

`ClusterResourceGrantPolicy` is a cluster-scoped resource; a policy affects your project when its
`projectSelector` matches your project's namespace labels. List all policies and inspect their selectors:

```shell
d8 k get clusterresourcegrantpolicies -o yaml
```

The controller also renders an `AvailableClusterResource` catalog in your namespace for each registered
definition — that is the effective, per-project view of what is allowed.

### I changed a grant policy, but existing objects still use the old value — is this a bug?

No. This is **grandfathering**: on UPDATE, values already present in an object are preserved, so narrowing
a policy does not break pre-existing objects. Only CREATE and field changes on UPDATE are validated
against the current grants. If you want an existing object to use a new value, update the field explicitly.

The `ClusterResourceGrantPolicyViolation` alert fires when existing objects violate the current grants —
it is informational; the objects keep working.

### The `ClusterResourceGrantPolicyViolation` alert fired — what does it mean?

It means one or more existing objects in a project reference a cluster resource value that the current
grants no longer allow. The objects are **not broken** (grandfathering), but the administrator is alerted
to the drift. Resolve it by either widening the policy to cover the existing values, or by updating the
objects to use an allowed value. See the Grafana dashboard *Security → Cluster Resource Grant Violations*
and the `d8_cluster_objects_grant_violated` metric.

### How do I allow all StorageClasses except specific ones?

Use the `denied` list (or `deniedSelector`) while leaving the resource otherwise open:

```yaml
resources:
  - resourceName: storageclasses
    denied:
      - expensive-nvme
      - archived-hdd
```

`denied` overrides `allowed`/`allowedSelector`: a name matching both is denied.

### There are no grant policies — does that mean nothing works?

No — the opposite. Without a `ClusterResourceGrantPolicy`, **all** resources are available (the permissive
default). Grant policies are the *only* way to narrow access; the system never restricts anything on its
own.

### I created a grant policy but nothing changed — how do I debug it?

Check, in order:

1. **Selector match** — does the policy's `projectSelector` match your project's namespace labels?
   (`d8 k get ns <project> --show-labels`).
2. **AvailableClusterResource** — does the catalog in the project namespace reflect the policy?
   (`d8 k get available <resource> -n <project> -o yaml`).
3. **Definition exists** — does the `GrantableClusterResourceDefinition` for `resourceName` exist, and is
   its `GrantableClusterResourceReference` bound? (`...status.conditions[Bound]` = `Resolved`).
4. **Path registered** — is the field you expect validated/defaulted covered by a reference's `fieldPaths`?

### How do grants interact with `ClusterAuthorizationRule` / `AuthorizationRule`?

They are independent mechanisms. RBAC (via `ClusterAuthorizationRule`, `AuthorizationRule`, `RoleBinding`)
decides **who can create** an object. Grants decide **which cluster resource values** that object may
reference. A user needs both: the RBAC right to create a PVC *and* a grant allowing the `StorageClass`.

### What happens if I disable the `multitenancy-manager` module?

The grant CRDs and webhooks are removed, so there is **no validation or defaulting** of cluster resource
references — everything is allowed. Existing objects are unaffected. The `x-deckhouse-grantable-resource`
extension in DKP application settings degrades silently (no validation). The `d8:use:dict` role in
`user-authz` is independent and continues to work.

### Enabling grants on an existing cluster — what happens?

Grandfathering protects existing objects: their already-set values are preserved on UPDATE. Only new
objects, and field changes on UPDATE, are validated against the new grants. There is no "backfill" — you
do not need to migrate existing objects before creating policies.

### I narrowed a grant — what happens to objects created under the wider policy?

They keep working (grandfathering). New objects and field changes on UPDATE are validated against the
narrowed grants. If you delete the policy, the resource becomes fully open again.
