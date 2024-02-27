---
title: "Модуль dashboard"
webIfaces:
- name: dashboard
---

Устанавливает [Web UI](https://github.com/kubernetes/dashboard) Kubernetes Dashboard для ручного управления кластером. Интерфейс интегрирован с модулями [user-authn](../../modules/150-user-authn/) и [user-authz](../../modules/140-user-authz/). Доступ в кластер осуществляется от имени пользователя и с учетом его прав.

Если модуль работает по HTTP, он использует минимальные права с ролью `User` из модуля [user-authz](../../modules/140-user-authz/).

> **Важно!** Включите модуль `user-authz` для работы с модулем `dashboard`.

Kubernetes Dashboard позволяет:
- управлять подами и другими высокоуровневыми ресурсами;
- получать доступ в контейнеры через веб-консоль для отладки;
- просматривать логи отдельных контейнеров.
