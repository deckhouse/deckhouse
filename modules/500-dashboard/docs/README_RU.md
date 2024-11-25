---
title: "Модуль dashboard"
webIfaces:
- name: dashboard
---

Устанавливает [Web UI](https://github.com/kubernetes/dashboard) Kubernetes Dashboard для ручного управления кластером, который интегрирован с модулями [user-authn](../../modules/150-user-authn/) и [user-authz](../../modules/140-user-authz/) (доступ в кластер осуществляется от имени пользователя и с учетом его прав).

Модуль не поддерживает работу через HTTP и будет отключен.

{% alert level="warning" %}
Для корректной работы модуля `dashboard` необходимо включить модуль `user-authz`.
{% endalert %}

{% alert level="warning" %}
Для функционирования модуля `dashboard` требуется либо включенный модуль `user-authn`, либо настроенные параметры [`externalAuthentication`](../cr.html#examples).
{% endalert %}

Kubernetes Dashboard предоставляет следующие возможности:

- Управление подами и другими высокоуровневыми ресурсами;
- Доступ к контейнерам через веб-консоль для отладки;
- Просмотр логов отдельных контейнеров.
