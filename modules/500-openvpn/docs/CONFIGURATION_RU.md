---
title: "Модуль openvpn: настройки"
---

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](../user-authn/). Также можно настроить аутентификацию с помощью параметра [externalAuthentication](#parameters-auth-externalauthentication). Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы просмотреть сгенерированный пароль, выполните команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите ресурс Secret:

```shell
d8 k -n d8-openvpn delete secret/basic-auth
```

{% alert level="info" %}
Параметр `auth.password` больше не поддерживается.
{% endalert %}
