---
title: "Модуль user-authn: настройки"
---

<!-- SCHEMA -->

Автоматический деплой [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy) в namespace вашего приложения и подключение его к Dex происходят при создании custom resource [`DexAuthenticator`](cr.html#dexauthenticator).

**Важно!** Так как использование OpenID Connect по протоколу HTTP является слишком значительной угрозой безопасности (что подтверждается, например, тем, что Kubernetes API-сервер не поддерживает работу с OIDC по HTTP), данный модуль можно установить только при включенном HTTPS (`https.mode` выставить в отличное от `Disabled` значение или на уровне кластера, или в самом модуле).

**Важно!** При включении данного модуля аутентификация во всех веб-интерфейсах перестанет использовать HTTP Basic Auth и переключится на Dex (который, в свою очередь, будет использовать настроенные вами внешние провайдеры).
Для настройки kubectl необходимо перейти по адресу `https://kubeconfig.<modules.publicDomainTemplate>/`, авторизоваться в настроенном внешнем провайдере и скопировать shell-команды к себе в консоль.

**Важно!** Для работы аутентификации в dashboard и kubectl требуется [донастройка API-сервера](faq.html#настройка-kube-apiserver). Для автоматизации этого процесса реализован модуль [control-plane-manager](../../modules/control-plane-manager/), который включен по умолчанию.

## Настройка ресурсов

Модуль `user-authn` позволяет задать значения запросов и лимитов ресурсов для всех своих компонентов. По умолчанию используются следующие параметры:

- **Dex OIDC провайдер** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Генератор kubeconfig** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Basic Auth прокси** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Dex authenticator** — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты);
- **Redis** (используется Dex authenticator) — 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты).

{% alert level="info" %}
Если в кластере включён [Vertical Pod Autoscaler (VPA)](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/vertical-pod-autoscaler/), значения лимитов управляются автоматически. Вы можете задать минимальные и максимальные границы через секцию `resources`.
{% endalert %}

### Пример конфигурации

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
