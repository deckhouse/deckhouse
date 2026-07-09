# Permission Browser API Server

An aggregated extension API server for authorization-related resources in the Deckhouse `user-authz` module.

## Overview

This server provides:

1. **BulkSubjectAccessReview**: Checking multiple authorization requests in a single HTTP call, dramatically reducing the number of network hops compared to individual `SubjectAccessReview` calls.

2. **AccessibleNamespace**: A computed, ACL-filtered list of namespaces accessible to the requesting user. This is similar to OpenShift's Project API concept - it allows UI applications to show users only the namespaces they have access to, without requiring cluster-wide `list namespaces` permission.

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
