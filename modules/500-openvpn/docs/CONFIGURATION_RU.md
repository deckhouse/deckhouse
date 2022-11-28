---
title: "Модуль openvpn: настройки"
---

<!-- SCHEMA -->

## Примечания

**Внимание!** В панели администратора всегда используется подсеть, определенная в параметре `tunnelNetwork`. Статические адреса пользователей необходимо выдавать из этой подсети. Если используется протокол UDP, то эти адреса будут конвертированы для использования в подсети `udpTunnelNetwork`. В этом случае сети в параметрах `tunnelNetwork` и `udpTunnelNetwork` должны быть одного размера.

Пример:
* `tunnelNetwork`: `10.5.5.0/24`
* `udpTunnelNetwork`: `10.5.6.0/24`
* Тогда IP-адрес пользователя `10.5.5.8` (из диапазона `tunnelNetwork`) будет сконвертирован в `10.5.6.8` (из диапазона `udpTunnelNetwork`).

## Аутентификация

По умолчанию используется модуль [user-authn](../150-user-authn/). Также можно настроить аутентификацию с помощью параметра [externalAuthentication](#parameters-auth-externalauthentication). Если эти варианты отключены, то модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-openvpn delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
