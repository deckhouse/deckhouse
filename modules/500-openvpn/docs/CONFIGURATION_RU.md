---
title: "Модуль openvpn: настройки"
---

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](../150-user-authn/). Также можно настроить аутентификацию с помощью параметра [externalAuthentication](#parameters-auth-externalauthentication). Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы просмотреть сгенерированный пароль, выполните команду:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите Secret:

```shell
kubectl -n d8-openvpn delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
