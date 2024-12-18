---
title: "Модуль dashboard: настройки"
---

## Аутентификация

По умолчанию используется модуль [user-authn](/products/kubernetes-platform/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через [`externalAuthentication`](examples.html).

Если ни один из этих способов не включен, модуль `dashboard` будет отключен.

{% alert level="warning" %}
Параметры `auth.password` и `accessLevel` больше не поддерживаются.
{% endalert %}

## Настройки

У модуля нет обязательных настроек.

<!-- SCHEMA -->
