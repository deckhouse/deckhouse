# Permission Browser API Server

An aggregated extension API server for the `BulkSubjectAccessReview` resource in the Deckhouse `user-authz` module.

## Overview

This server enables checking **multiple** authorization requests in a **single HTTP call**, dramatically reducing the number of network hops compared to individual `SubjectAccessReview` calls. It's designed to support frontend applications (like the Deckhouse UI) that need to check 60-80+ permissions when rendering a page.

## API

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

- No support for `fieldSelector`/`labelSelector` in `ResourceAttributes` (planned for future versions)
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
