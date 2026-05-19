---
title: "Модуль user-authn: настройки"
---

<!-- SCHEMA -->

Автоматический деплой [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy) в namespace вашего приложения и подключение его к Dex происходят при создании custom resource [`DexAuthenticator`](cr.html#dexauthenticator).

**Важно!** Так как использование OpenID Connect по протоколу HTTP является слишком значительной угрозой безопасности (что подтверждается, например, тем, что Kubernetes API-сервер не поддерживает работу с OIDC по HTTP), данный модуль можно установить только при включенном HTTPS (`https.mode` выставить в отличное от `Disabled` значение или на уровне кластера, или в самом модуле).

**Важно!** При включении данного модуля аутентификация во всех веб-интерфейсах перестанет использовать HTTP Basic Auth и переключится на Dex (который, в свою очередь, будет использовать настроенные вами внешние провайдеры).
Для настройки kubectl необходимо перейти по адресу `https://kubeconfig.<modules.publicDomainTemplate>/`, авторизоваться в настроенном внешнем провайдере и скопировать shell-команды к себе в консоль.

**Важно!** Для работы аутентификации в dashboard и kubectl требуется [донастройка API-сервера](faq.html#настройка-kube-apiserver). Для автоматизации этого процесса реализован модуль [control-plane-manager](/modules/control-plane-manager/), который включен по умолчанию.

Чтобы задать дополнительные лейблы и аннотации для подов `dex-authenticator`, используйте поле `spec.podMetadata`.

Пример (отключение инъекции Istio sidecar):

```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: app-name
  namespace: app-namespace
spec:
  applicationDomain: app-name.kube.my-domain.com
  applicationIngressClassName: nginx
  podMetadata:
    annotations:
      sidecar.istio.io/inject: "false"
```
