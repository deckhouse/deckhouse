---
title: "Модуль upmeter: настройки"
---

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication`.  
Если эти варианты отключены, то модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы посмотреть сгенерированный пароль, выполните команду:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.webui.password'
```

Чтобы сгенерировать новый пароль, удалите Secret:

```shell
kubectl -n d8-upmeter delete secret/basic-auth-webui
```

Чтобы посмотреть сгенерированный пароль для страницы статуса, выполните команду:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.status.password'
```

Чтобы сгенерировать новый пароль для страницы статуса, удалите Secret:

```shell
kubectl -n d8-upmeter delete secret/basic-auth-status
```

> **Внимание!** Параметры `auth.status.password` и `auth.webui.password` больше не поддерживаются.
