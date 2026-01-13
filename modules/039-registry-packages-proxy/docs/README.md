---
title: "The registry-packages-proxy module"
description: "Internal registry packages proxy for optimizing access to registry packages."
---

## Description

The `registry-packages-proxy` module provides an in-cluster HTTP proxy service for accessing packages from container registries. It acts as an intermediary between cluster components and external or internal registries, offering caching capabilities to optimize bandwidth usage and improve package retrieval performance.

This module is a critical infrastructure component that runs on master nodes and is used during cluster bootstrap and runtime operations to fetch packages from container registries.

## How it works

The module deploys a highly-available proxy service that:

- Runs as a deployment on master nodes with `hostNetwork` enabled to ensure availability during bootstrap when CNI is not yet available
- Listens on port `4219` (HTTPS) on each master node's IP address
- Provides a `/package` endpoint for retrieving registry packages by digest
- Implements local caching of retrieved packages (up to 1Gi) to reduce network traffic and improve performance
- Watches `ModuleSource` custom resources to obtain registry credentials for different repositories
- Uses kube-rbac-proxy to secure access to the proxy and metrics endpoints

### Architecture

The proxy service consists of two containers:

1. **registry-packages-proxy** - The main proxy application that:
   - Fetches packages from remote registries using digests
   - Caches packages locally in an ephemeral volume (1Gi max)
   - Supports authentication to registries via credentials from `ModuleSource` resources
   - Provides health checks and Prometheus metrics
   - Listens on `127.0.0.1:5080` (HTTP, internal)

2. **kube-rbac-proxy** - Provides RBAC-based access control:
   - Exposes the service on port `4219` (HTTPS)
   - Secures `/metrics` endpoint with Kubernetes RBAC authorization
   - Secures `/package` endpoint requiring appropriate permissions
   - Allows unauthenticated access to `/healthz`

### Package retrieval flow

When a component requests a package:

1. The request includes a `digest` parameter (required) and optional `repository` and `path` parameters
2. The proxy checks its local cache for the requested digest
3. If cached, the package is served directly from cache
4. If not cached:
   - The proxy fetches credentials for the specified repository from watched `ModuleSource` resources
   - The package is retrieved from the remote registry
   - The package is streamed to the client while simultaneously being cached for future requests
5. Responses include appropriate HTTP headers for caching (`Cache-Control`, `ETag`, `Content-Length`)

### High availability

The module ensures high availability through:

- Running multiple replicas on master nodes (in HA configurations)
- Pod anti-affinity rules to distribute pods across different masters
- PodDisruptionBudget to prevent simultaneous disruption of all replicas
- Vertical Pod Autoscaler support for automatic resource adjustments

## Limitations

- The module runs exclusively on master nodes
- It requires `hostNetwork: true` to function during bootstrap phase
- Cache size is limited to 1Gi per pod
- The module is designed for internal use by Deckhouse components and requires specific RBAC permissions to access