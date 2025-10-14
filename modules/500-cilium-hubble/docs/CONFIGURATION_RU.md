---
title: "Модуль cilium-hubble: настройки"
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Если модуль `cni-cilium` выключен, параметр `ciliumHubbleEnabled:` не повлияет на включение модуля `cilium-hubble`.

{% include module-conversion.liquid %}

{% include module-settings.liquid %}

## Аутентификация

По умолчанию используется модуль [user-authn](/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication`.
Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Чтобы просмотреть сгенерированный пароль, выполните команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите Secret:

```shell
d8 k -n d8-cni-cilium delete secret/hubble-basic-auth
```

{% alert level="info" %}
Параметр `auth.password` больше не поддерживается.
{% endalert %}
