---
title: "Resource configuration"
permalink: en/admin/configuration/access/authentication/resource-configuration.html
description: "Configure resource limits and requests for authentication components in Deckhouse Kubernetes Platform. Dex, Kubeconfig Generator, and Basic Auth Proxy resource management."
---

Deckhouse Kubernetes Platform allows you to configure resource limits and requests for all components. By default, the following values are used:

- **Dex OIDC Provider** — 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Kubeconfig Generator** — 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Basic Auth Proxy** — 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Dex Authenticator** — 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Redis** (used by Dex Authenticator) — 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)

{% alert level="info" %}
When [Vertical Pod Autoscaler (VPA)](/modules/vertical-pod-autoscaler/) is enabled, the resource limits are managed automatically by VPA, but you can still configure the minimum and maximum allowed values through the `resources` section.
{% endalert %}

## Example configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: v1
  settings:
    resources:
      dex:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "20m"
          memory: "50Mi"
      kubeconfigGenerator:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "20m"
          memory: "50Mi"
      basicAuthProxy:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "20m"
          memory: "50Mi"
      dexAuthenticator:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "20m"
          memory: "50Mi"
      redis:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "20m"
          memory: "50Mi"
```
