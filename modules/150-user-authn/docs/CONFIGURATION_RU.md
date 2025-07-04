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

Модуль позволяет настраивать лимиты и запросы ресурсов для всех компонентов. По умолчанию используются следующие значения:

- **Dex OIDC провайдер**: 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты)
- **Генератор kubeconfig**: 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты)
- **Basic Auth прокси**: 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты)
- **Dex authenticator**: 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты)
- **Redis** (используется Dex authenticator): 10m CPU / 25Mi памяти (запросы), 100m CPU / 100Mi памяти (лимиты)

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
          cpu: "20m"
          memory: "50Mi"
        limits:
          cpu: "50m"
          memory: "100Mi"
      kubeconfigGenerator:
        requests:
          cpu: "15m"
          memory: "40Mi"
        limits:
          cpu: "30m"
          memory: "80Mi"
      basicAuthProxy:
        requests:
          cpu: "10m"
          memory: "30Mi"
        limits:
          cpu: "25m"
          memory: "60Mi"
      dexAuthenticator:
        requests:
          cpu: "15m"
          memory: "40Mi"
        limits:
          cpu: "40m"
          memory: "80Mi"
      redis:
        requests:
          cpu: "10m"
          memory: "30Mi"
        limits:
          cpu: "20m"
          memory: "50Mi"
```

**Примечание:** При включенном Vertical Pod Autoscaler (VPA) лимиты ресурсов управляются автоматически VPA, но вы можете настроить минимальные и максимальные допустимые значения через секцию `resources`.
