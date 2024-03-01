---
title: "Модуль documentation: настройки"
---

У модуля нет обязательных настроек.

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication`.  
Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы посмотреть сгенерированный пароль, выполните команду:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values documentation -o json | jq '.documentation.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите Secret:

```shell
kubectl -n d8-system delete secret/documentation-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
