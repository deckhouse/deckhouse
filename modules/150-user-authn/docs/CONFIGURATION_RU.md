---
title: "Модуль user-authn: настройки"
---

<!-- SCHEMA -->

{% alert level="warning" %}
Модуль работает только с HTTPS, так как использование OpenID Connect по HTTP — серьёзная угроза безопасности, а API-сервер Kubernetes не поддерживает OIDC по HTTP. При значении `Disabled` параметра `https.mode` модуль отключается.
{% endalert %}

Задайте для `https.mode` значение, отличное от `Disabled`, на уровне кластера ([`modules.https.mode`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-https-mode)) или в модуле. Если вы задаёте секцию `https` модуля, укажите в ней и `mode`. Без `mode` применяется `Disabled`.
