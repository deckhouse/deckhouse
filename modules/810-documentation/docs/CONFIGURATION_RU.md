---
title: "Модуль documentation: настройки"
---

У модуля нет обязательных настроек.

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](../user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы посмотреть сгенерированный пароль, выполните команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values documentation -o json | jq '.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите ресурс Secret:

```shell
d8 k -n d8-system delete secret/documentation-basic-auth
```

{% alert level="info" %}
Параметр `auth.password` больше не поддерживается.
{% endalert %}
