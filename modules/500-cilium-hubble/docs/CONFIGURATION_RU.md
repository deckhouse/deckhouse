---
title: "Модуль cilium-hubble: настройки"
---

{% include module-bundle.liquid %}

Если модуль `cni-cilium` выключен, параметр `ciliumHubbleEnabled:` не повлияет на включение модуля `cilium-hubble`.

{% include module-settings.liquid %}

## Аутентификация

По умолчанию используется модуль [user-authn](/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication`.  
Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы просмотреть сгенерированный пароль, выполните команду:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите Secret:

```shell
kubectl -n d8-cni-cilium delete secret/hubble-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
