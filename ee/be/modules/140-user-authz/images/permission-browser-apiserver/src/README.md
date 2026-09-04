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
      reason: "either you have no access to the namespace or the namespace does not exist"
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

3. **Namespace-ACL reads**: `get`/`list`/`watch` of a cluster-scoped resource whose visibility the apiserver derives from the namespace ACL — currently `deckhouse.io/projects`, visible whenever any namespace of the project is — are reported as **allowed** even when the two layers above say otherwise. The cluster-wide RBAC grant for such a resource is deliberately withheld so that the apiserver's filter engages, and the API answers those reads with the accessible subset (a `get` of an invisible object answers `NotFound`) rather than a 403. Reporting them as denied would make the console hide a section the user can open. Mutating verbs and subresources keep the plain RBAC answer, and so does a cluster that does not serve the resource at all — without `multitenancy-manager` there is no Project CRD, the apiserver's own bypass is gated on the filter being registered for the resource, and an allow here would advertise an API that answers `NotFound`. Resource existence comes from the discovery-backed scope cache.

### Multi-tenancy Restrictions

The multi-tenancy engine applies the same restrictions as the `user-authz-webhook`:

- `limitNamespaces`: Regex patterns for allowed namespaces
- `namespaceSelector`: Label selectors for allowed namespaces
- `allowAccessToSystemNamespaces`: Access to `kube-*`, `d8-*`, `default` namespaces
- Cluster-scoped requests for namespaced resources are denied if user has namespace restrictions

## Local Development

### Prerequisites

- Go 1.25+ (see the `go` directive in `go.mod`)
- Kubernetes cluster access (for in-cluster testing)

### Building

```bash
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

Only `--user-authz-config` is defined by this server; the rest come from the
generic apiserver's recommended options, so the list below is not exhaustive.

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
- **Best-effort discovery**: If discovery cannot say whether a resource is namespaced, it is assumed cluster-scoped. Assuming namespaced would make the resolver treat the caller as having namespaced access and list every namespace, so the unknown case is resolved the other way.

### General
- RBAC rules and namespace information are served from watch-driven informers, so changes land within seconds. The 30-minute figure in the code is the informer resync interval — a periodic re-list that guards against a missed watch event, not the propagation delay.
- The multi-tenancy config file is re-read every second. The file itself is a projected ConfigMap, and kubelet refreshes it on its own sync period, so a `ClusterAuthorizationRule` change takes up to about a minute to reach the running pod.

## Health Endpoints

- `/readyz`: Returns 200 once the resource scope cache has been populated at least once, in addition to the generic apiserver's own readiness checks
- `/livez`: Returns 200 when server is alive

This component registers no custom Prometheus metrics. Request rates and latencies are available from the generic apiserver metrics it inherits, for example `apiserver_request_duration_seconds{resource="bulksubjectaccessreviews"}`.
