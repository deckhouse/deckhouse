---
title: "Модуль dashboard"
webIfaces:
- name: dashboard
---

Устанавливает [Web UI](https://github.com/kubernetes/dashboard) Kubernetes Dashboard для ручного управления кластером, который интегрирован с модулями [user-authn](../../modules/150-user-authn/) и [user-authz](../../modules/140-user-authz/) (доступ в кластер осуществляется от имени пользователя и с учетом его прав).

Kubernetes Dashboard предоставляет следующие возможности:

- управление подами и другими высокоуровневыми ресурсами;
- доступ к контейнерам через веб-консоль для отладки;
- просмотр логов отдельных контейнеров.

{% alert level="warning" %}
Модуль не поддерживает работу через HTTP.
{% endalert %}

Для работы модуля необходимо:
- включить модуль [user-authz](../user-authz/);
- либо включить модуль [user-authn](../user-authn/), либо подключить внешнюю аутентификацию (секция параметров [externalAuthentication](configuration.html#parameters-auth-externalauthentication) модуля).
