---
title: "Модуль openvpn: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  openvpnEnabled: "true"
```

**Внимание!** В панели администратора всегда используется `tunnelNetwork`, статические адреса необходимо выдавать из неё. Если используется UDP, то эти адреса будут сконвертированны для использования в подсети `udpTunnelNetwork`, при этом, `tunnelNetwork` и `udpTunnelNetwork` должны быть одного размера. Пример:
* `tunnelNetwork`: 10.5.5.0/24
* `udpTunnelNetwork`: 10.5.6.0/24
* адрес для пользователя 10.5.5.8 (из диапазона `tunnelNetwork`) будет сконвертирован в 10.5.6.8 (из диапазона `udpTunnelNetwork`)

## Аутентификация

По умолчанию используется модуль [user-authn](/{{ page.lang }}/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, то модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить секрет:

```shell
kubectl -n d8-openvpn delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.

## Параметры

<!-- SCHEMA -->
