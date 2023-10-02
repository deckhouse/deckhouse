---
title: "Модуль dashboard: настройки"
---

У модуля нет обязательных настроек.

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values dashboard -o json | jq '.dashboard.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-dashboard delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
