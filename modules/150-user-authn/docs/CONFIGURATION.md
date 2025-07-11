---
title: "The user-authn module: configuration"
---

<!-- SCHEMA -->

The creation of the [`DexAuthenticator`](cr.html#dexauthenticator) Custom Resource leads to the automatic deployment of [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy) to your application's namespace and connecting it to Dex.

**Caution!** Since using OpenID Connect over HTTP poses a significant threat to security (the fact that Kubernetes API server doesn't support OICD over HTTP confirms that), this module can only be installed if HTTPS is enabled (to do this, set the `https.mode` parameter to the value other than `Disabled` either at the cluster level or in the module).

**Caution!** When this module is enabled, authentication in all web interfaces will be switched from HTTP Basic Auth to Dex (the latter, in turn, will use the external providers that you have defined). To configure kubectl, go to `https://kubeconfig.<modules.publicDomainTemplate>/`, log in to your external provider's account and copy the shell commands to your console.

**Caution!** The API server requires [additional configuration](faq.html#configuring-kube-apiserver) to use authentication for dashboard and kubectl. The [control-plane-manager](../../modules/control-plane-manager/) module (enabled by default) automates this process.

## Resource Configuration

The module allows you to configure resource limits and requests for all components. By default, the following values are used:

- **Dex OIDC Provider**: 100m CPU / 128Mi memory (requests), 250m CPU / 256Mi memory (limits)
- **Kubeconfig Generator**: 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Basic Auth Proxy**: 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Dex Authenticator**: 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)
- **Redis** (used by Dex Authenticator): 10m CPU / 25Mi memory (requests), 100m CPU / 100Mi memory (limits)

### Example Configuration

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
          cpu: "100m"
          memory: "128Mi"
        limits:
          cpu: "250m"
          memory: "256Mi"
      kubeconfigGenerator:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "100m"
          memory: "100Mi"
      basicAuthProxy:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "100m"
          memory: "100Mi"
      dexAuthenticator:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "100m"
          memory: "100Mi"
      redis:
        requests:
          cpu: "10m"
          memory: "25Mi"
        limits:
          cpu: "100m"
          memory: "100Mi"
```

**Note:** When Vertical Pod Autoscaler (VPA) is enabled, the resource limits are managed automatically by VPA, but you can still configure the minimum and maximum allowed values through the `resources` section.
