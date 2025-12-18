---
title: "Мониторинг SLA кластера: настройки"
---

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, то модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.webui.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-webui
```

Посмотреть сгенерированный пароль для страницы статуса можно командой:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.status.password'
```

Чтобы сгенерировать новый пароль для страницы статуса, нужно удалить секрет:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-status
```

> **Внимание!** Параметры `auth.status.password` и `auth.webui.password` больше не поддерживаются.
