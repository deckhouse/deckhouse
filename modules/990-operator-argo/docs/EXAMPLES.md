---
title: "The operator-argo module: usage examples"
description: "Deckhouse Kubernetes Platform — examples of using the operator-argo module."
---

## Enabling the module

Before you start using the `operator-argo` module in your Kubernetes cluster, it needs to be enabled. This can be done in one of the ways described below.

### Method 1: Enabling using ModuleConfig

Create a `ModuleConfig` resource to enable the module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: true
```

### Method 2: Enabling using deckhouse-controller

To enable the module, run the following command:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module enable operator-argo
```

## Disabling the module

{% alert level="warning" %}
Disabling the module will remove the ArgoCD operator (all resources from the `namespace d8-operator-argo`). However, deployed ArgoCD installations and applications will remain untouched.
{% endalert %}

If you need to disable the `operator-argo` module, you can do so using one of the methods described below.

### Method 1: Disabling using ModuleConfig

Disable the module by setting the `enabled` value to `false` in the `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: false
```

### Method 2: Disabling using deckhouse-controller

To disable the module, run the following command:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module disable operator-argo
```

## Installing ArgoCD and deploying an ArgoCD application

Deploy ArgoCD, which will be accessible via Ingress. Create the necessary resources using the following manifest:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: argocd
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  server:
    host: <argocd-domain>
    ingress:
      enabled: true
      tls:
      - hosts:
        - <argocd-domain>
        secretName: argocd-ingress-tls
    # To avoid internal redirection loops from HTTP to HTTPS, the API server should be run with TLS disabled.
    # https://argo-cd.readthedocs.io/en/stable/operator-manual/ingress/#disable-internal-tls
    insecure: true
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: argocd-ingress
  namespace: argocd
spec:
  dnsNames:
  - <argocd-domain>
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  secretName: argocd-ingress-tls
```

Create a namespace for your future application. This will provide isolation and management for your application's resources:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  labels:
    argocd.argoproj.io/managed-by: argocd
```

Deploy an ArgoCD application by specifying where your application is located and how to deploy it. Use the following manifest to create the application:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: argocd
spec:
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  project: default
  source:
    path: helm-guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: HEAD
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## Using Deckhouse Kubernetes Platform single sign-on system for authentication in ArgoCD

Create an OAuth2 client that will be used for authentication in ArgoCD:

```yaml
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: argocd
  namespace: argocd
spec:
  redirectURIs:
    - https://<argocd-domain>/api/dex/callback
    - https://<argocd-domain>/api/dex/callback-reserve
```

After creating DexClient resource, DKP will register a client with the client ID (clientID) `dex-client-argocd@argocd`(`dex-client-<name>@<namespace>`).

Wait for Deckhouse Kunernetes Platform to create a Secret with the client secret:

```shell
kubectl -n argocd get secret/dex-client-argocd
```

Configure ArgoCD to use the DKP single sign-on system:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  sso:
    dex:
      config: |
        connectors:
          - type: oidc
            id: deckhouse
            name: deckhouse
            config:
              issuer: "https://dex.<cluster-domain>/"
              clientID: "dex-client-argocd@argocd"
              clientSecret: "$dex-client-argocd:clientSecret"
    provider: dex
  server:
    host: <argocd-domain>
    ingress:
      enabled: true
      tls:
        - hosts:
            - <argocd-domain>
          secretName: argocd-ingress-tls
    # To avoid internal redirection loops from HTTP to HTTPS, the API server should be run with TLS disabled.
    # https://argo-cd.readthedocs.io/en/stable/operator-manual/ingress/#disable-internal-tls
    insecure: true
```

Restart the ArgoCD server:

```shell
kubectl -n argocd rollout restart deploy/argocd-server
```

{% alert level="warning" %}
If you don’t restart the server, login attempts will fail, and you will see an error in the ArgoCD server log ([issue](https://github.com/argoproj/argo-cd/issues/13526)).

<details><summary>An example of the error message...</summary>
<code>time="2024-10-16T14:12:59Z" level=warning msg="Failed to verify token: failed to verify token: token verification failed for all audiences: error for aud "argo-cd": Failed to query provider "https://argocd.<argocd-domain>/api/dex": Get "https://argocd.<argocd-domain>/api/dex/.well-known/openid-configuration": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<argocd-domain>, error for aud "argo-cd-cli": Failed to query provider "https://argocd.<argocd-domain>/api/dex": Get "https://argocd.<argocd-domain>/api/dex/.well-known/openid-configuration": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<argocd-domain>"
</code>
</details>
{% endalert %}

## Granting ArgoCD access to cluster-wide resources

To allow ArgoCD to manage cluster-wide resources, specify the [clusterConfigNamespaces](configuration.html#parameters-clusterconfignamespaces) parameter in the module settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: true
  settings:
    clusterConfigNamespaces: <list of namespaces of cluster-scoped Argo CD instances>
  version: 1
```

## Using a Custom Cluster Domain Instead of cluster.local

To configure ArgoCD to use a custom cluster FQDN (e.g., prod.local), set the clusterDomain field accordingly:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  ...
  clusterDomain: "prod.local"
  ...
```
