---
title: "Модуль cilium-hubble: настройки"
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Модуль останется отключенным вне зависимости от параметра `ciliumHubbleEnabled:`, если не включен модуль `cni-cilium`.

{% include module-settings.liquid %}

## Аутентификация

По умолчанию используется модуль [user-authn](/products/kubernetes-platform/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-cni-cilium delete secret/hubble-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
