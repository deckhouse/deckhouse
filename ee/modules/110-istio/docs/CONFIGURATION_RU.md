---
title: "Модуль istio: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  istioEnabled: "true"
```

## Аутентификация

По умолчанию используется модуль [user-authn](/{{ page.lang }}/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, то модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить секрет:

```shell
kubectl -n d8-istio delete secret/kiali-basic-auth
```

**Внимание:** параметр `auth.password` больше не поддерживается.

## Параметры

<!-- SCHEMA -->
