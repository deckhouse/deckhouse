---
title: "Модуль istio: настройки"
---

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Для просмотра сгенерированного пароля используйте команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите секрет:

```shell
d8 k -n d8-istio delete secret/kiali-basic-auth
```

{% alert level="warning" %}
Параметр `auth.password` больше не поддерживается.
{% endalert %}
