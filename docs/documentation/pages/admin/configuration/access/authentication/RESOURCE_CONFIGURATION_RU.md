---
title: "Настройка ресурсов"
permalink: ru/admin/configuration/access/authentication/resource-configuration.html
lang: ru
---

В Deckhouse Kubernetes Platform можно задать значения лимитов запросов и ресурсов для всех компонентов. По умолчанию используются следующие параметры:

- **Dex OIDC провайдер** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Генератор kubeconfig** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Basic Auth прокси** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Dex authenticator** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Redis** (используется Dex authenticator) — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты).

{% alert level="info" %}
Если в кластере включён [Vertical Pod Autoscaler (VPA)](/modules/vertical-pod-autoscaler/), управление значениями лимитов происходит автоматически, с помощью VPA. Вы можете задать минимальные и максимальные границы через секцию `resources`.
{% endalert %}

## Пример конфигурации

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
