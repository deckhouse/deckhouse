---
title: "Модуль operator-argo: примеры использования"
description: "Deckhouse Kubernetes Platform — примеры использования модуля operator-argo."
---

## Включение модуля

Для использования модуля `operator-argo` в Deckhouse Kubernetes Platform, его необходимо включить. Это можно сделать одним из способов, описанных далее.

### Способ 1: Включение с применением ModuleConfig

Для включения модуля создайте ресурс `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: true
```

### Способ 2: Включение с использованием deckhouse-controller

Для включения модуля выполните команду:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module enable operator-argo
```

## Выключение модуля

{% alert level="warning" %}
При отключении модуля будет удалён оператор ArgoCD (все ресурсы из пространства имен `d8-operator-argo`). Развёрнутые инсталляции ArgoCD и приложения останутся нетронутыми.
{% endalert %}

Если вам необходимо отключить модуль `operator-argo`, вы можете сделать это одним из способов, описанных далее.

### Способ 1: Выключение с применением ModuleConfig

Для выключения модуля установите значение `enabled` в `false` в конфигурации `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: false
```

### Способ 2: Выключение с использованием deckhouse-controller

Для выключения модуля выполните следующую команду:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module disable operator-argo
```

## Установка ArgoCD и развертывание ArgoCD Application

Разверните ArgoCD, который будет доступен через Ingress:

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

Создайте пространство имен для приложения:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  labels:
    argocd.argoproj.io/managed-by: argocd
```

Разверните ArgoCD Application:

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

## Использование единой системы аутентификации Deckhouse Kubernetes Platform для аутентификации в ArgoCD

Создайте OAuth2-клиента для аутентификации в ArgoCD:

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

После создания ресурса DexClient, DKP зарегистрирует клиента с идентификатором (clientID) `dex-client-argocd@argocd` (используя шаблон `dex-client-<name>@<namespace>`).

Дождитесь, пока Deckhouse Kunernetes Platform создаст Secret с секретным ключом для клиента:

```shell
kubectl -n argocd get secret/dex-client-argocd
```

Настройте ArgoCD для использования системы аутентификации DKP:

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

Перезапустите сервер ArgoCD:

```shell
kubectl -n argocd rollout restart deploy/argocd-server
```

{% alert level="warning" %}
Если не перезапустить сервер ArgoCD, то попытка входа не удастся, а в логе сервера ArgoCD появится сообщение об ошибке ([issue о проблеме](https://github.com/argoproj/argo-cd/issues/13526)).

<details><summary>Пример сообщения об ошибке...</summary>
<code>time="2024-10-16T14:12:59Z" level=warning msg="Failed to verify token: failed to verify token: token verification failed for all audiences: error for aud "argo-cd": Failed to query provider "https://argocd.<argocd-domain>/api/dex": Get "https://argocd.<argocd-domain>/api/dex/.well-known/openid-configuration": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<argocd-domain>, error for aud "argo-cd-cli": Failed to query provider "https://argocd.<argocd-domain>/api/dex": Get "https://argocd.<argocd-domain>/api/dex/.well-known/openid-configuration": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<argocd-domain>"
</code>
</details>
{% endalert %}

## Предоставление ArgoCD доступа к ресурсам кластера

Для предоставления доступа ArgoCD к кластерным (cluster-wide) ресурсам используйте параметр [clusterConfigNamespaces](configuration.html#parameters-clusterconfignamespaces) в настройках модуля:

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

## Использование собственного доменного имени кластера вместо cluster.local

Чтобы настроить ArgoCD для работы с другим FQDN кластера (например, prod.local), укажите его в параметре clusterDomain:

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
