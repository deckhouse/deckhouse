---
title: "Модуль openvpn: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  openvpnEnabled: "true"
```

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

**Внимание:** параметр `auth.password` больше не поддерживается.

## Параметры

<!-- SCHEMA -->
