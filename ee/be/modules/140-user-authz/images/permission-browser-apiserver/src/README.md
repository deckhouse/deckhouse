# Permission Browser API Server

An aggregated extension API server for authorization-related resources in the Deckhouse `user-authz` module.

## Overview

This server provides:

1. **BulkSubjectAccessReview**: Checking multiple authorization requests in a single HTTP call, dramatically reducing the number of network hops compared to individual `SubjectAccessReview` calls.

2. **AccessibleNamespace**: A computed, ACL-filtered list of namespaces accessible to the requesting user. This is similar to OpenShift's Project API concept - it allows UI applications to show users only the namespaces they have access to, without requiring cluster-wide `list namespaces` permission.

3. **WhoCan**: The reverse RBAC lookup - given an action, which subjects may perform it.

4. **SubjectAccessReport**: The forward counterpart of `WhoCan` - given a subject, everything it may do, assembled from its bindings instead of probed one review at a time.

5. **RoleAccessReport**: The catalogue - given a role, everything it grants. What `SubjectAccessReport` answers about a person, this answers about the role model itself, for the export a security officer keeps and a regulator reads.

## API

### Resource: `AccessibleNamespace`

- **Group**: `authorization.deckhouse.io`
- **Version**: `v1alpha1`
- **Kind**: `AccessibleNamespace` / `AccessibleNamespaceList`
- **Endpoints**:
  - `GET /apis/authorization.deckhouse.io/v1alpha1/accessiblenamespaces` - List accessible namespaces
  - `GET /apis/authorization.deckhouse.io/v1alpha1/accessiblenamespaces/{name}` - Get specific namespace if accessible

#### Response Schema

```yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: AccessibleNamespaceList
metadata:
  resourceVersion: ""  # Always empty - watch not supported
items:
  - metadata:
      name: default
  - metadata:
      name: my-app-namespace
```

#### How It Works

A namespace is considered "accessible" if BOTH conditions are met:
1. **Multi-tenancy allows access**: The user's `ClusterAuthorizationRule` doesn't deny the namespace (via `limitNamespaces`, `namespaceSelector`, or system namespace restrictions)
2. **RBAC grants any namespaced permission**: The user has at least one RoleBinding or ClusterRoleBinding that grants ANY verb on ANY namespaced resource in that namespace

#### Limitations

- **Watch NOT supported**: Clients must poll for updates. The `resourceVersion` is always empty.
- **Computed at request time**: The list is calculated based on current RBAC and multi-tenancy rules. Changes propagate after informer cache sync (up to 30 minutes).
- **Best-effort resource scope detection**: If a resource's scope can't be determined via discovery (transient errors / unavailable APIService), we treat it as namespaced.

#### Security

- **No existence disclosure**: GET requests for inaccessible namespaces return 404 (not 403)
- **No reason strings in responses**: Denial reasons are logged server-side only

#### Example Usage

```bash
# List all accessible namespaces
kubectl get accessiblenamespaces

# Check if specific namespace is accessible
kubectl get accessiblenamespace my-app-ns
```

```bash
# Using curl
curl -k -H "Authorization: Bearer $TOKEN" \
  https://kubernetes/apis/authorization.deckhouse.io/v1alpha1/accessiblenamespaces
```

### Resource: `BulkSubjectAccessReview`

- **Group**: `authorization.deckhouse.io`
- **Version**: `v1alpha1`
- **Kind**: `BulkSubjectAccessReview`
- **Endpoint**: `POST /apis/authorization.deckhouse.io/v1alpha1/bulksubjectaccessreviews`

### Request/Response Schema

```yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: BulkSubjectAccessReview
spec:
  # Optional: Non-self mode - check access for another user
  user: "some-user"
  groups: ["group1", "group2"]
  uid: "optional-uid"
  extra:
    some-key: ["value1", "value2"]
  
  # Required: List of access checks
  requests:
    - resourceAttributes:
        namespace: "default"
        verb: "get"
        group: ""
        version: "v1"
        resource: "pods"
        # Optional:
        subresource: ""
        name: ""
    - nonResourceAttributes:
        path: "/healthz"
        verb: "get"

status:
  results:
    - allowed: true
      reason: "RBAC: allowed by ClusterRoleBinding..."
    - allowed: false
      denied: true
      reason: "user has no access to the namespace"
```

### Resource: `WhoCan`

- **Group**: `authorization.deckhouse.io`
- **Version**: `v1alpha1`
- **Kind**: `WhoCan`
- **Endpoint**: `POST /apis/authorization.deckhouse.io/v1alpha1/whocans`

`WhoCan` is the reverse-RBAC query, analogous to OpenShift's `oc policy who-can`.
Given an action (a verb on a resource, optionally scoped to a namespace / name /
subresource, or a non-resource URL), it returns the list of subjects (Users,
Groups, ServiceAccounts) that RBAC grants that action to.

This resource is ephemeral - it is not stored, only created.

#### Request/Response Schema

```yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
spec:
  resourceAttributes:
    namespace: "myproject"
    verb: "create"
    group: "networking.k8s.io"
    resource: "networkpolicies"
    # Optional:
    version: ""
    subresource: ""
    name: ""
  # Alternatively, a non-resource URL:
  # nonResourceAttributes:
  #   path: "/metrics"
  #   verb: "get"

status:
  users:
    - "alice"
  groups:
    - "netops"
  serviceAccounts:
    - namespace: "kube-system"
      name: "controller"
```

#### How It Works

The server scans the informer-backed RBAC caches (no live API calls):

1. **ClusterRoleBindings**: for each binding whose referenced ClusterRole grants
   the requested action, all of the binding's subjects are collected. These apply
   cluster-wide (and therefore to the requested namespace, if any).
2. **RoleBindings** (only when `namespace` is set): for each RoleBinding in the
   target namespace whose referenced Role *or* ClusterRole grants the action, the
   binding's subjects are collected. ServiceAccount subjects with an empty
   namespace default to the RoleBinding's namespace.

Rule matching reuses the same logic as the forward authorizer, so verb / resource
/ apiGroup wildcards (`*`), subresources and `resourceNames` are handled
identically. **Aggregated ClusterRoles** are resolved transparently: the
aggregation controller populates `ClusterRole.Rules`, and the engine reads those
already-aggregated rules from the lister.

#### Security

- **Elevated capability**: `WhoCan` discloses who is granted an action, so the
  ability to call it must be restricted. It is gated by the same delegated
  authorization as every other resource here - the caller must be granted
  `create` on `whocans` in `authorization.deckhouse.io`. Unlike
  `accessiblenamespaces`, the bundled `d8:user-authz:who-can-checker` ClusterRole
  is **not** bound to `system:authenticated`; it must be granted explicitly to
  trusted UIs/administrators.

#### Multi-tenancy

The result reflects **RBAC grants only**. The multi-tenancy layer (which can only
*deny* a subject's own forward requests) is not applied to filter the returned
subjects, because the query is already namespace-scoped and reverse filtering each
returned subject through multi-tenancy would be expensive. A returned subject may
still be further restricted by multi-tenancy at actual request time. This is a
documented limitation (see Limitations below).

#### Example Usage

```bash
# Who can create networkpolicies in namespace "myproject"?
cat <<EOF | kubectl create -f - -o yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-netpol
spec:
  resourceAttributes:
    namespace: myproject
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

### Resource: `SubjectAccessReport`

- **Group**: `authorization.deckhouse.io`
- **Version**: `v1alpha1`
- **Kind**: `SubjectAccessReport`
- **Endpoints**:
  - `POST /apis/authorization.deckhouse.io/v1alpha1/subjectaccessreports`
  - `POST /apis/authorization.deckhouse.io/v1alpha1/subjectaccessreports/nonself`

Answers "what can this subject do". A `SubjectAccessReview` answers one yes/no
question at a time, so building this picture from reviews means guessing which
questions to ask; the report instead walks the subject's bindings once and
reports what they grant.

The resource is ephemeral - it is not stored, only created.

#### Request/Response Schema

```yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
spec:
  # Omit to report on the caller itself (self mode).
  subject:
    kind: User          # User | Group | ServiceAccount
    name: alice
    namespace: ""       # ServiceAccount only
  # Extra groups to attribute to the subject. Ignored in self mode: the caller's
  # own token is the source of truth for its groups.
  groups: ["netops"]
  # Namespaces to report on. Empty means every namespace the subject is bound in.
  namespaces: []
  resolveGroups: true   # resolve group membership through the group catalog
  expandWildcards: true # expand "*" rules against the discovery snapshot

status:
  subject:
    kind: User
    name: alice
    groups: ["netops", "d8:platform"]   # implicit, requested and resolved
  # Every binding that matched, flat, before any aggregation.
  roleAssignments:
    - bindingKind: RoleBinding
      bindingName: alice-admin
      namespace: myproject
      roleKind: ClusterRole
      roleName: d8:namespace:admin
      matchedBy: {kind: User, name: alice}
      role: {scope: namespace, level: admin, title: "Namespace Admin"}
  # Access grouped into scopes: the cluster-wide one, then one per set of
  # namespaces whose access is identical (a project reads as one section).
  scopes:
    - namespaces: ["myproject"]
      resources:
        - group: ""
          resource: pods
          verbs: ["get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"]
          viaWildcard: false
          viaVerbWildcard: false
          sources:
            - bindingKind: RoleBinding
              bindingName: alice-admin
              roleKind: ClusterRole
              roleName: d8:namespace:admin
              matchedBy: {kind: User, name: alice}
              verbs: ["get", "list", "..."]
      caveat:
        protectedVerbs: true
        superadmin: false
        restrictedNamespaces: ["myproject"]
  notes: []
  evaluationError: ""
  truncated: false
```

#### How It Works

Everything is served from the informer caches: one pass over ClusterRoleBindings
plus one over RoleBindings, regardless of how many resources the cluster has.

1. **Identity**: the subject plus its groups. Group membership is resolved
   through the group catalog (`deckhouse.io` Groups), following nested groups
   with a cycle guard. In self mode the caller's token is used as-is.
2. **Bindings**: every binding listing the identity is collected, along with
   which subject entry matched - so a client can show access with and without
   group membership.
3. **Rules**: the referenced roles' rules are expanded into rows. `verbs: ["*"]`
   is expanded to the common verbs and the row is marked `viaVerbWildcard`,
   because the rule also grants the privileged verbs RBAC checks separately
   (`escalate`, `bind`, `impersonate`, `proxy`). Resource wildcards are expanded
   against the discovery snapshot and marked `viaWildcard`.
4. **Scopes**: namespaces whose access is identical are merged into one scope.

#### Caveats

RBAC is not the only thing standing between a subject and an object, and a
report that showed RBAC alone would overstate what the subject can actually do.
`scopes[].caveat` carries the difference:

- `protectedVerbs`: the scope grants verbs the system-resource admission webhook
  restricts on Deckhouse-owned objects.
- `superadmin` / `superadminNamespaces`: the subject bypasses that webhook, either
  by holding one of its superadmin roles, by holding `cluster-admin`, or by
  belonging to one of its bypass groups (`system:masters` and friends).
- `restrictedNamespaces`: where the restriction does apply.

The role and group sets used here are the webhook's own, asserted by a contract
test against `modules/140-user-authz/webhooks/validating/system_resources.py`.
A report claiming a restriction the webhook does not apply - or missing one it
does - is worse than no report.

#### Security

- **Self mode** (no `spec.subject`) is granted to `system:authenticated` through
  `d8:user-authz:self-subject-access-checker`. It discloses nothing the caller
  cannot already obtain from `SelfSubjectRulesReview`. `spec.groups` is therefore
  ignored in this mode: honouring it would hand out any named group's permission
  map to anybody.
- **Non-self mode** requires `create` on the `subjectaccessreports/nonself`
  subresource, granted by `d8:user-authz:subject-access-checker`. It must be
  bound explicitly, and never to an administrator whose
  `ClusterAuthorizationRule` carries namespace filters: the resource is
  cluster-scoped, so the answer is not confined to the namespaces such an
  administrator may see.

#### Example Usage

```bash
# What can alice do?
cat <<EOF | kubectl create -f - -o yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: alice-report
spec:
  subject:
    kind: User
    name: alice
EOF
```

### Resource: `RoleAccessReport`

- **Group**: `authorization.deckhouse.io`
- **Version**: `v1alpha1`
- **Kind**: `RoleAccessReport`
- **Endpoint**: `POST /apis/authorization.deckhouse.io/v1alpha1/roleaccessreports`

Answers "what does this role grant" - the catalogue side of the question
`SubjectAccessReport` answers for a subject. It exists for the export a security
officer keeps and a regulator reads: which resources are covered by which role,
in a form that can be diffed against the export from last quarter.

The resource is ephemeral - it is not stored, only created.

#### Request/Response Schema

```yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: RoleAccessReport
spec:
  # primary: the scope-based roles. legacy: the access levels of
  # ClusterAuthorizationRule. Defaults to primary.
  model: primary
  # Empty selection reports every role of the model.
  roles:
    names: []
    scopes: []          # namespace | project | subsystem | system (primary)
    accessLevels: []    # User | PrivilegedUser | ... (legacy)
    excludeCustom: false  # leave out the roles created in this cluster (primary)
  expandWildcards: true   # expand "*" against the discovery snapshot
  includeComposition: false  # the detailed mode: which capability granted what

status:
  snapshot:
    time: "2026-08-04T12:00:00Z"
    model: primary
    expandedWildcards: true
    discoveryResources: 412   # what the wildcards were expanded against
    digest: "9f2c..."         # hash of the roles, excluding the timestamp
  roles:
    - name: d8:namespace:admin
      role: {scope: namespace, level: admin, titles: {ru: "..."}}
      legacyName: d8:use:role:admin
      namespaced: true
      composition:
        - name: d8:namespace-capability:kubernetes:manage_workloads
          role: {titles: {ru: "..."}}
      resources:
        - group: apps
          resource: deployments
          verbs: [get, list, watch, create, update, patch, delete]
          viaWildcard: false
          viaVerbWildcard: false
          sources:
            - roleKind: ClusterRole
              roleName: d8:namespace-capability:kubernetes:manage_workloads
              verbs: [get, list, watch]
  notes: []
  truncated: false
```

#### How It Works

1. **Rows come from the capabilities**, not from the role's own `rules`. The
   aggregation controller fills a role by concatenating what its capabilities
   grant, without recording which one granted what; expanding them one at a time
   is the only way the detailed mode can name the capability behind a row.
2. **Wildcards are expanded by the same code as `SubjectAccessReport`** and
   marked the same way (`viaWildcard`, `viaVerbWildcard`). Two implementations
   would drift, and an export that contradicts the access report is worse than
   no export.
3. **A namespace- or project-scoped role drops its cluster-scoped rows.** The
   binding webhook refuses a `ClusterRoleBinding` carrying such a role, so those
   rules exist in RBAC but can never be exercised; listing them would overstate
   the access the document reports.
4. **The legacy model resolves an access level the way the fan-out hook does**:
   the level's own ClusterRole plus every ClusterRole annotated
   `user-authz.deckhouse.io/access-level` for that level or a lower one.
5. **`excludeCustom` reports the model the platform ships.** Custom roles belong
   to one cluster, and listing them beside the model reads as if the platform
   shipped them - but they are real access, so leaving them out is the caller's
   decision, the count of skipped ones goes into `notes`, and a role named in
   `names` is reported either way.

#### Caveats

- **The digest excludes the timestamp** so two exports of an unchanged cluster
  compare equal. It covers the reported roles and nothing else.
- **`discoveryResources` is part of the answer**, not decoration: "every
  resource" means one thing on a cluster with virtualization installed and
  another without it, and a wildcard row cannot be read a year later without it.
- **Subresources are only expanded when a rule names them.** A bare `*` grants
  top-level resources; enumerating every subresource would bury the rows that
  matter.

#### Security

The report describes roles, never subjects, and every row of it is already
readable in the ClusterRoles themselves - so it is not an elevated capability
and needs no self/non-self split. `d8:user-authz:role-catalogue-reader` exists
for identities that should produce the export without being able to read the
whole RBAC tree.

#### Example Usage

```bash
# The plain matrix of the namespace-scoped roles.
cat <<EOF | kubectl create -f - -o yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: RoleAccessReport
metadata:
  name: namespace-roles
spec:
  roles:
    scopes: [namespace]
EOF

# What the legacy Admin access level grants, with the roles it is assembled from.
cat <<EOF | kubectl create -f - -o yaml
apiVersion: authorization.deckhouse.io/v1alpha1
kind: RoleAccessReport
metadata:
  name: legacy-admin
spec:
  model: legacy
  roles:
    accessLevels: [Admin]
  includeComposition: true
EOF
```

## Modes of Operation

### Self Mode

When `spec.user` is not specified, the server checks permissions for the **authenticated user** making the request (similar to `SelfSubjectAccessReview`).

### Non-Self Mode

When `spec.user` is specified, the server checks permissions for that user. The caller must have appropriate permissions to perform access reviews for other users.

## Authorization Logic

The server uses a **composite authorizer** with the following order:

1. **Multi-tenancy layer**: Enforces Deckhouse `ClusterAuthorizationRule` restrictions (namespace filters, system namespace access). This layer can only **deny**; it never allows.

2. **RBAC layer**: Standard Kubernetes RBAC checks using in-memory informers.

### Multi-tenancy Restrictions

The multi-tenancy engine applies the same restrictions as the `user-authz-webhook`:

- `limitNamespaces`: Regex patterns for allowed namespaces
- `namespaceSelector`: Label selectors for allowed namespaces
- `allowAccessToSystemNamespaces`: Access to `kube-*`, `d8-*`, `default` namespaces
- Cluster-scoped requests for namespaced resources are denied if user has namespace restrictions

## Local Development

### Prerequisites

- Go 1.23+
- Kubernetes cluster access (for in-cluster testing)

### Building

```bash
cd src
make build-local
```

### Running Code Generation

```bash
make generate
```

### Running Tests

```bash
make test
```

### Running Locally

```bash
./permission-browser-apiserver \
  --secure-port=8443 \
  --tls-cert-file=/path/to/tls.crt \
  --tls-private-key-file=/path/to/tls.key \
  --user-authz-config=/path/to/config.json
```

## Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `--secure-port` | HTTPS port | 443 |
| `--tls-cert-file` | TLS certificate file | - |
| `--tls-private-key-file` | TLS key file | - |
| `--user-authz-config` | Path to user-authz webhook config | `/etc/user-authz-webhook/config.json` |
| `--authentication-kubeconfig` | Kubeconfig for authentication | - |
| `--authorization-kubeconfig` | Kubeconfig for authorization | - |

## Limitations (v1alpha1)

### BulkSubjectAccessReview
- No support for `fieldSelector`/`labelSelector` in `ResourceAttributes` (planned for future versions)

### AccessibleNamespace
- **Watch NOT supported**: The `resourceVersion` is always empty (`""`). Clients must poll.
- **Computed resource**: The list is calculated at request time, not stored. There's no etcd persistence.
- **Best-effort discovery**: If the API discovery cannot determine whether a resource is namespaced, it assumes namespaced for safety.

### WhoCan
- **RBAC-only result**: Subjects are derived from (Cluster)RoleBindings. Multi-tenancy restrictions are NOT applied to filter the returned subjects (a returned subject may still be denied by multi-tenancy at actual request time).
- **Role explainability not included**: The result lists subjects but not which specific (Cluster)Role/(Cluster)RoleBinding granted each one (kept out for simplicity; may be added later).
- **Computed resource**: Results are calculated at request time from informer caches, not stored. Changes propagate after informer cache sync (up to 30 minutes).

### SubjectAccessReport
- **RBAC plus caveats, not an authorization decision**: the report says what the bindings grant and flags where admission narrows it; it does not replace a `SubjectAccessReview` for a specific request.
- **Multi-tenancy is reported, not applied per row**: a `ClusterAuthorizationRule` narrowing the subject is surfaced through `status.notes`, not by removing individual rows.
- **Wildcard verbs are sampled**: a `verbs: ["*"]` row lists the common verbs and sets `viaVerbWildcard`; the privileged verbs (`escalate`, `bind`, `impersonate`, `proxy`) are covered by the rule but not enumerated.
- **Degraded discovery is reported, not hidden**: when the discovery snapshot is incomplete, wildcard expansion covers fewer resources and `status.notes` says so. Resources unknown to the snapshot are kept rather than dropped - an audit report should never quietly undercount.
- **Bounded output**: namespaces, resource rows, sources per row and role assignments are capped; `status.truncated` reports that a cap was hit.
- **Computed resource**: built at request time from informer caches, not stored.

### General
- The server caches RBAC rules and namespace information; changes may take up to 30 minutes to propagate
- Multi-tenancy config is reloaded every second from the mounted ConfigMap

## Metrics

| Metric | Description |
|--------|-------------|
| `bulksar_requests_total` | Total number of BulkSubjectAccessReview requests |
| `bulksar_checks_total` | Total number of individual permission checks |
| `bulksar_request_duration_seconds` | Histogram of request durations |

## Health Endpoints

- `/readyz`: Returns 200 when informer caches are synced
- `/livez`: Returns 200 when server is alive
