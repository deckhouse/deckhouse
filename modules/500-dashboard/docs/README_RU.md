---
title: "Модуль dashboard"
description: "Веб-интерфейс Kubernetes Dashboard для управления кластером Deckhouse Platform Certified Security Edition."
webIfaces:
- name: dashboard
---

Устанавливает Web UI Kubernetes Dashboard для ручного управления кластером, который интегрирован с модулями [user-authn](../../modules/user-authn/) и [user-authz](../../modules/user-authz/) (доступ в кластер осуществляется от имени пользователя и с учетом его прав).

Kubernetes Dashboard предоставляет следующие возможности:

- управление подами и другими высокоуровневыми ресурсами;
- доступ к контейнерам через веб-консоль для отладки;
- просмотр логов отдельных контейнеров.

{% alert level="warning" %}
Модуль не поддерживает работу через HTTP.
{% endalert %}

Для работы модуля необходимо:

1. Включить модуль [user-authz](../user-authz/);
1. Включить модуль [user-authn](../user-authn/), либо подключить внешнюю аутентификацию (секция параметров [externalAuthentication](configuration.html#parameters-auth-externalauthentication) модуля).
