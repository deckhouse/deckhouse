---
title: "Модуль istio: настройки"
---

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](../user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
d8 k -n d8-istio delete secret/kiali-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
