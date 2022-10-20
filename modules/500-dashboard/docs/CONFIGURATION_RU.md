---
title: "Модуль dashboard: настройки"
---

{% include module-bundle.liquid %}

У модуля нет обязательных настроек.

## Аутентификация

По умолчанию используется модуль [user-authn](/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, то модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values dashboard -o json | jq '.dashboard.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить секрет:

```shell
kubectl -n d8-dashboard delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.

## Параметры

<!-- SCHEMA -->
