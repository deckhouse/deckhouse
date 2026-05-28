---
title: "The registry-packages-proxy module"
description: "Internal proxy for optimizing access to packages from container registries."
---

The `registry-packages-proxy` module provides an in-cluster HTTP proxy service for accessing packages from container registries. It acts as an intermediary between cluster components and external or internal registries, offering caching capabilities to optimize bandwidth usage and improve package retrieval performance.

This module is a critical infrastructure component that runs on master nodes and is used during cluster bootstrap and runtime operations to fetch packages from container registries.

The module deploys a highly-available proxy service that:

- Runs as a deployment on master nodes with `hostNetwork` enabled to ensure availability during bootstrap when CNI is not yet available.
- Listens on port `4219` (HTTPS) on each master node's IP address.
- Provides a `/package` endpoint for retrieving registry packages by digest.
- Implements local caching of retrieved packages (up to 1 GB) to reduce network traffic and improve performance.
- Watches [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) custom resources to obtain registry credentials for different repositories.
- Uses `kube-rbac-proxy` to secure access to the proxy and metrics endpoints.
- Exposes a public HTTPS API (via Ingress) for Deckhouse CLI binaries.
- Exposes an in-cluster HTTPS API for package icons (no public Ingress).

## Architecture

The proxy service consists of two containers:

1. **registry-packages-proxy**: The main proxy application that:
   - Fetches packages from remote registries using digests.
   - Caches packages locally in an ephemeral volume (1 GB max).
   - Supports authentication to registries via credentials from [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) resources.
   - Provides health checks and Prometheus metrics.
   - Listens on `127.0.0.1:5080` (HTTP, internal).

1. **kube-rbac-proxy**: Provides RBAC-based access control:
   - Exposes the service on port `4219` (HTTPS).
   - Secures `/metrics` endpoint with Kubernetes RBAC authorization.
   - Secures `/package` endpoint requiring appropriate permissions.
   - Secures `/v1/images/*` (Deckhouse CLI downloads) with Kubernetes RBAC authorization.
   - Allows unauthenticated access to `/healthz`.

## HTTP API

After the cluster is bootstrapped and the DNS name template is configured in the [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) parameter, the module creates an Ingress for `registry-packages-proxy` using that template (for example, `registry-packages-proxy.company.my` for `publicDomainTemplate: "%s.company.my"`).

The endpoints listed below are reachable in two ways:

- **Public** (via the Ingress on the `registry-packages-proxy` public host): only `/v1/images/*`.
- **In-cluster only** (via the `registry-packages-proxy.d8-cloud-instance-manager.svc` Service on port `443`, or `:4219` on each master node for bootstrap): all routes, including `/v1/packages/*`.

### Package icons (`/v1/packages/`) — in-cluster only

Package icons are served **without authentication** (`kube-rbac-proxy` excludes these paths from RBAC) but are **only reachable from inside the cluster**. They are intentionally **not** routed through the public Ingress, so the public domain `registry-packages-proxy.<publicDomain>` will not serve them.

| Method | Path                                                                       | Description |
|--------|----------------------------------------------------------------------------|-------------|
| `GET`, `HEAD` | `/v1/packages/<PACKAGE-REPOSITORY>/<PACKAGE-NAME>/metadata/icon/`          | Icon of the latest semver tag |
| `GET`, `HEAD` | `/v1/packages/<PACKAGE-REPOSITORY>/<PACKAGE-NAME>/metadata/icon`           | Same as above |
| `GET`, `HEAD` | `/v1/packages/<PACKAGE-REPOSITORY>/<PACKAGE-NAME>/metadata/icon/<VERSION>` | Icon of a specific version (`<VERSION>` is a semantic version, e.g. `v1.0.1`) |

`<PACKAGE-REPOSITORY>` is the `metadata.name` of a [PackageRepository](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#packagerepository) custom resource; its `spec.registry.repo` field tells the proxy which registry path to pull the icon from. The proxy reads the icon from the OCI image `<spec.registry.repo>/<PACKAGE-NAME>:<TAG>`.

The proxy looks for the following files inside the package image, in this priority order, and returns the first one it finds:

| Path                | `Content-Type`    |
|---------------------|-------------------|
| `docs/icon.svg`     | `image/svg+xml`   |
| `docs/icon.png`     | `image/png`       |
| `docs/icon.jpg`     | `image/jpeg`      |
| `docs/icon.jpeg`    | `image/jpeg`      |

If none of these are present (or the file is larger than 4 MiB), the proxy returns `404 Not Found`; callers should fall back to a default icon. SVG is preferred because it is resolution-independent.

Example request from a pod in the cluster:

```shell
curl -fsSk "https://registry-packages-proxy.d8-cloud-instance-manager.svc/v1/packages/my-repo/my-module/metadata/icon/"
```

Example response headers (when the image ships `docs/icon.svg`):

```console
Content-Type: image/svg+xml
Content-Disposition: attachment; filename="<PACKAGE-NAME>.svg"
```

### Deckhouse CLI downloads (`/v1/images/`) — public

These endpoints are reachable through the public Ingress (`registry-packages-proxy.<publicDomain>`) and require a valid Kubernetes token (or client certificate accepted by `kube-rbac-proxy`) and RBAC permission to `get` the `deployments/cli-binary` subresource `registry-packages-proxy` in namespace `d8-cloud-instance-manager`.

Grant access with the ClusterRole `d8:registry-packages-proxy:cli-download` (bind it to users or ServiceAccounts via ClusterRoleBinding or RoleBinding).

| Method | Path                          | Description |
|--------|-------------------------------|-------------|
| `GET` | `/v1/images/<IMAGE>/tags`     | JSON list of tags |
| `GET`, `HEAD` | `/v1/images/<IMAGE>/tags/<TAG>` | OCI image as `application/x-gzip` (flattened layers) |

Allowed `<IMAGE>` values:

- `deckhouse-cli`
- `deckhouse-cli/plugins/<PLUGIN>` (single path segment for `<PLUGIN>`)

Example:

```shell
curl -fsS -H "Authorization: Bearer ${TOKEN}" \
  "https://registry-packages-proxy.example.com/v1/images/deckhouse-cli/tags"
```

### Internal `/package` endpoint

The legacy `/package?digest=...` endpoint (used during bootstrap and by internal components) remains protected by RBAC (`deployments/http` subresource). It is not exposed through the public Ingress.

## Package retrieval flow

When a component requests a package:

1. The request includes a `digest` parameter (required) and optional `repository` and `path` parameters.
1. The proxy checks its local cache for the requested digest.
1. If cached, the package is served directly from cache.
1. If not cached:
   - The proxy fetches credentials for the specified registry from watched [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) resources.
   - The package is retrieved from the remote registry.
   - The package is streamed to the client while simultaneously being cached for future requests.
1. Responses include appropriate HTTP headers for caching (`Cache-Control`, `ETag`, `Content-Length`).

## High availability

The module ensures high availability through:

- Running multiple replicas on master nodes (in HA configurations).
- Pod anti-affinity rules to distribute pods across different masters.
- PodDisruptionBudget to prevent simultaneous disruption of all replicas.
- Vertical Pod Autoscaler support for automatic resource adjustments.

## RBAC roles provided by the module

| ClusterRole | Purpose |
|-------------|---------|
| `d8:registry-packages-proxy:cli-download` | Access to `/v1/images/*` |
| `d8:registry-packages-proxy:packages-download` | Reserved for future authenticated `/v1/packages/*` routes (icons are served anonymously, in-cluster only) |

## Limitations

- The module runs exclusively on master nodes.
- It requires `hostNetwork: true` to function during bootstrap phase.
- Cache size is limited to 1 GB per pod.
- Most HTTP endpoints require Kubernetes RBAC; only health checks (healthz) and package icons are anonymous. Package icons are additionally limited to in-cluster access (no public Ingress route).
- Package icons are read from fixed paths inside the package image (`docs/icon.svg`, `docs/icon.png`, `docs/icon.jpg`, `docs/icon.jpeg`) with SVG preferred. Maximum icon size is 4 MiB.
